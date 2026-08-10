package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrXHSJGSyncRunLocked = errors.New("another XHS Spotlight sync is already running")

type XHSJGSyncRun struct {
	RunID                 int64      `json:"run_id"`
	Mode                  string     `json:"mode"`
	Target                string     `json:"target"`
	TriggerType           string     `json:"trigger_type"`
	RequestedAdvertiserID *int64     `json:"requested_advertiser_id,omitempty"`
	Status                string     `json:"status"`
	AdvertisersCount      int        `json:"advertisers_count"`
	CampaignsCount        int        `json:"campaigns_count"`
	UnitsCount            int        `json:"units_count"`
	CreativitiesCount     int        `json:"creativities_count"`
	DeactivatedCount      int64      `json:"deactivated_count"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	StartedAt             time.Time  `json:"started_at"`
	FinishedAt            *time.Time `json:"finished_at,omitempty"`
}

func (p *Postgres) StartXHSJGSyncRun(ctx context.Context, mode, target, trigger string, advertiserID int64) (XHSJGSyncRun, error) {
	var run XHSJGSyncRun
	var requested *int64
	if advertiserID > 0 {
		requested = &advertiserID
	}
	err := p.pool.QueryRow(ctx, `
		INSERT INTO xhs_jg_sync_runs (mode, target, trigger_type, requested_advertiser_id, status)
		VALUES ($1,$2,$3,$4,'running')
		RETURNING run_id, mode, target, trigger_type, requested_advertiser_id, status, started_at
	`, mode, target, trigger, requested).Scan(
		&run.RunID, &run.Mode, &run.Target, &run.TriggerType, &run.RequestedAdvertiserID, &run.Status, &run.StartedAt,
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return XHSJGSyncRun{}, ErrXHSJGSyncRunLocked
		}
		return XHSJGSyncRun{}, fmt.Errorf("start XHS Spotlight sync run: %w", err)
	}
	return run, nil
}

func (p *Postgres) FailRunningXHSJGSyncRuns(ctx context.Context, reason string, staleBefore time.Time) error {
	if reason == "" {
		reason = "sync service restarted before the run finished"
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE xhs_jg_sync_runs
		SET status='failed', error_message=$1, finished_at=NOW()
		WHERE status='running' AND started_at < $2
	`, reason, staleBefore)
	if err != nil {
		return fmt.Errorf("recover interrupted XHS Spotlight sync runs: %w", err)
	}
	return nil
}

func (p *Postgres) FinishXHSJGSyncRun(ctx context.Context, runID int64, status string, advertisers, campaigns, units, creativities int, deactivated int64, syncErr error) error {
	errorMessage := ""
	if syncErr != nil {
		errorMessage = syncErr.Error()
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE xhs_jg_sync_runs SET
			status=$2, advertisers_count=$3, campaigns_count=$4, units_count=$5,
			creativities_count=$6, deactivated_count=$7,
			error_message=NULLIF($8,''), finished_at=NOW()
		WHERE run_id=$1
	`, runID, status, advertisers, campaigns, units, creativities, deactivated, errorMessage)
	if err != nil {
		return fmt.Errorf("finish XHS Spotlight sync run %d: %w", runID, err)
	}
	return nil
}

func (p *Postgres) ListXHSJGSyncRuns(ctx context.Context, limit int) ([]XHSJGSyncRun, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("XHS Spotlight sync run limit must be between 1 and 100")
	}
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, mode, target, trigger_type, requested_advertiser_id, status,
			advertisers_count, campaigns_count, units_count, creativities_count,
			deactivated_count, COALESCE(error_message,''), started_at, finished_at
		FROM xhs_jg_sync_runs
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list XHS Spotlight sync runs: %w", err)
	}
	defer rows.Close()
	runs := make([]XHSJGSyncRun, 0, limit)
	for rows.Next() {
		var run XHSJGSyncRun
		if err := rows.Scan(
			&run.RunID, &run.Mode, &run.Target, &run.TriggerType, &run.RequestedAdvertiserID, &run.Status,
			&run.AdvertisersCount, &run.CampaignsCount, &run.UnitsCount, &run.CreativitiesCount,
			&run.DeactivatedCount, &run.ErrorMessage, &run.StartedAt, &run.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan XHS Spotlight sync run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate XHS Spotlight sync runs: %w", err)
	}
	return runs, nil
}

func (p *Postgres) XHSJGIncrementalSince(ctx context.Context, advertiserID int64, target string) (time.Time, error) {
	incrementalColumn, fullColumn, err := xhsJGSyncCursorColumns(target)
	if err != nil {
		return time.Time{}, err
	}
	var since time.Time
	err = p.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE((
			SELECT COALESCE(%s, %s)
			FROM xhs_jg_advertisers
			WHERE advertiser_id=$1
		), NOW() - INTERVAL '2 days')
	`, incrementalColumn, fullColumn), advertiserID).Scan(&since)
	if err != nil {
		return time.Time{}, fmt.Errorf("query XHS Spotlight %s incremental cursor for advertiser %d: %w", target, advertiserID, err)
	}
	return since, nil
}

func (p *Postgres) MarkXHSJGIncrementalSynced(ctx context.Context, advertiserID int64, target string, syncedAt time.Time) error {
	incrementalColumn, _, err := xhsJGSyncCursorColumns(target)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE xhs_jg_advertisers
		SET %s=$2, last_seen_at=NOW()
		WHERE advertiser_id=$1
	`, incrementalColumn), advertiserID, syncedAt)
	if err != nil {
		return fmt.Errorf("advance XHS Spotlight %s incremental cursor for advertiser %d: %w", target, advertiserID, err)
	}
	return nil
}

func xhsJGSyncCursorColumns(target string) (string, string, error) {
	switch target {
	case "campaigns":
		return "last_campaign_incremental_synced_at", "last_full_synced_at", nil
	case "units":
		return "last_unit_incremental_synced_at", "last_unit_full_synced_at", nil
	default:
		return "", "", fmt.Errorf("unsupported XHS Spotlight incremental sync target %q", target)
	}
}
