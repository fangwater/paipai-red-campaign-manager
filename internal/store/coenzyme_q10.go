package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCoenzymeQ10SyncLocked = errors.New("another coenzyme Q10 daily sync is already running")

type CoenzymeQ10SyncRun struct {
	RunID        int64      `json:"run_id"`
	SheetName    string     `json:"sheet_name"`
	Status       string     `json:"status"`
	Fetched      int        `json:"fetched"`
	Inserted     int        `json:"inserted"`
	Updated      int        `json:"updated"`
	Unchanged    int        `json:"unchanged"`
	EarliestDate string     `json:"earliest_date,omitempty"`
	LatestDate   string     `json:"latest_date,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type CoenzymeQ10SyncStatus struct {
	RecordCount  int                  `json:"record_count"`
	EarliestDate string               `json:"earliest_date,omitempty"`
	LatestDate   string               `json:"latest_date,omitempty"`
	LastSyncedAt *time.Time           `json:"last_synced_at,omitempty"`
	Recent       []CoenzymeQ10SyncRun `json:"recent"`
}

func (p *Postgres) FailRunningCoenzymeQ10Syncs(ctx context.Context, reason string) error {
	if reason == "" {
		reason = "manual sync service restarted before the request finished"
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE coenzyme_q10_sync_runs
		SET status='failed', error_message=$1, completed_at=NOW()
		WHERE status='running'
	`, reason)
	if err != nil {
		return fmt.Errorf("recover interrupted coenzyme Q10 syncs: %w", err)
	}
	return nil
}

func (p *Postgres) StartCoenzymeQ10Sync(ctx context.Context, wikiToken, sheetID, sheetName string) (coenzyme.SyncResult, error) {
	var result coenzyme.SyncResult
	err := p.pool.QueryRow(ctx, `
		INSERT INTO coenzyme_q10_sync_runs (
			source_wiki_token, sheet_id, sheet_name, status
		) VALUES ($1,$2,$3,'running')
		RETURNING run_id, sheet_name
	`, wikiToken, sheetID, sheetName).Scan(&result.RunID, &result.SheetName)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return coenzyme.SyncResult{}, ErrCoenzymeQ10SyncLocked
		}
		return coenzyme.SyncResult{}, fmt.Errorf("start coenzyme Q10 sync: %w", err)
	}
	return result, nil
}

func (p *Postgres) ApplyCoenzymeQ10Daily(ctx context.Context, runID int64, snapshot coenzyme.Snapshot) (result coenzyme.SyncResult, err error) {
	result = coenzyme.SyncResult{
		RunID: runID, SheetName: snapshot.SheetName, Fetched: len(snapshot.Records),
		SpreadsheetToken: snapshot.SpreadsheetToken, SheetID: snapshot.SheetID,
	}
	if len(snapshot.Records) == 0 {
		return result, errors.New("coenzyme Q10 daily snapshot is empty")
	}
	result.EarliestDate = snapshot.Records[0].ReportDate.Format("2006-01-02")
	result.LatestDate = snapshot.Records[len(snapshot.Records)-1].ReportDate.Format("2006-01-02")

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin coenzyme Q10 sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('coenzyme-q10-daily',0))`).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire coenzyme Q10 sync lock: %w", err)
	}
	if !locked {
		return result, ErrCoenzymeQ10SyncLocked
	}
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE coenzyme_q10_daily_stage (
			report_date DATE PRIMARY KEY, spend NUMERIC, impressions BIGINT, clicks BIGINT,
			ctr NUMERIC, cpc NUMERIC, cpm NUMERIC, all_transaction_gmv NUMERIC,
			all_store_roi NUMERIC, post_refund_gmv NUMERIC, post_refund_roi NUMERIC,
			coenzyme_gmv NUMERIC, coenzyme_roi NUMERIC, same_day_gmv NUMERIC,
			same_day_roi NUMERIC, search_spend NUMERIC, search_gmv NUMERIC,
			search_roi NUMERIC, search_spend_ratio NUMERIC, source_row_number INTEGER,
			content_hash TEXT
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create coenzyme Q10 staging table: %w", err)
	}

	rows := make([][]interface{}, len(snapshot.Records))
	for index, record := range snapshot.Records {
		rows[index] = []interface{}{
			record.ReportDate, nullableFloat(record.Spend), nullableInt(record.Impressions), nullableInt(record.Clicks),
			nullableFloat(record.CTR), nullableFloat(record.CPC), nullableFloat(record.CPM),
			nullableFloat(record.AllTransactionGMV), nullableFloat(record.AllStoreROI),
			nullableFloat(record.PostRefundGMV), nullableFloat(record.PostRefundROI),
			nullableFloat(record.CoenzymeGMV), nullableFloat(record.CoenzymeROI),
			nullableFloat(record.SameDayGMV), nullableFloat(record.SameDayROI),
			nullableFloat(record.SearchSpend), nullableFloat(record.SearchGMV),
			nullableFloat(record.SearchROI), nullableFloat(record.SearchSpendRatio),
			record.SourceRowNumber, record.ContentHash,
		}
	}
	columns := []string{
		"report_date", "spend", "impressions", "clicks", "ctr", "cpc", "cpm",
		"all_transaction_gmv", "all_store_roi", "post_refund_gmv", "post_refund_roi",
		"coenzyme_gmv", "coenzyme_roi", "same_day_gmv", "same_day_roi", "search_spend",
		"search_gmv", "search_roi", "search_spend_ratio", "source_row_number", "content_hash",
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"coenzyme_q10_daily_stage"}, columns, pgx.CopyFromRows(rows)); err != nil {
		return result, fmt.Errorf("stage coenzyme Q10 daily records: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE target.report_date IS NULL),
			COUNT(*) FILTER (WHERE target.report_date IS NOT NULL AND target.content_hash IS DISTINCT FROM source.content_hash),
			COUNT(*) FILTER (WHERE target.report_date IS NOT NULL AND target.content_hash = source.content_hash)
		FROM coenzyme_q10_daily_stage source
		LEFT JOIN coenzyme_q10_daily target USING (report_date)
	`).Scan(&result.Fetched, &result.Inserted, &result.Updated, &result.Unchanged); err != nil {
		return result, fmt.Errorf("compare coenzyme Q10 daily records: %w", err)
	}
	if _, err := tx.Exec(ctx, coenzymeQ10DailyUpsert, runID); err != nil {
		return result, fmt.Errorf("upsert coenzyme Q10 daily records: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit coenzyme Q10 daily sync: %w", err)
	}
	return result, nil
}

func (p *Postgres) FinishCoenzymeQ10Sync(ctx context.Context, result coenzyme.SyncResult, runErr error) error {
	status, errorMessage := "succeeded", ""
	if runErr != nil {
		status, errorMessage = "failed", runErr.Error()
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE coenzyme_q10_sync_runs SET
			spreadsheet_token=NULLIF($2,''), sheet_id=COALESCE(NULLIF($3,''),sheet_id),
			sheet_name=COALESCE(NULLIF($4,''),sheet_name), status=$5,
			fetched_count=$6, inserted_count=$7, updated_count=$8, unchanged_count=$9,
			earliest_date=NULLIF($10,'')::DATE, latest_date=NULLIF($11,'')::DATE,
			error_message=NULLIF($12,''), completed_at=NOW()
		WHERE run_id=$1
	`, result.RunID, result.SpreadsheetToken, result.SheetID, result.SheetName, status,
		result.Fetched, result.Inserted, result.Updated, result.Unchanged,
		result.EarliestDate, result.LatestDate, errorMessage)
	if err != nil {
		return fmt.Errorf("finish coenzyme Q10 sync %d: %w", result.RunID, err)
	}
	return nil
}

func (p *Postgres) CoenzymeQ10SyncStatus(ctx context.Context, limit int) (CoenzymeQ10SyncStatus, error) {
	if limit < 1 || limit > 100 {
		return CoenzymeQ10SyncStatus{}, errors.New("coenzyme Q10 sync run limit must be between 1 and 100")
	}
	var status CoenzymeQ10SyncStatus
	if err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(MIN(report_date)::TEXT,''), COALESCE(MAX(report_date)::TEXT,''),
			(SELECT MAX(completed_at) FROM coenzyme_q10_sync_runs WHERE status='succeeded')
		FROM coenzyme_q10_daily
	`).Scan(&status.RecordCount, &status.EarliestDate, &status.LatestDate, &status.LastSyncedAt); err != nil {
		return status, fmt.Errorf("summarize coenzyme Q10 daily records: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, sheet_name, status, fetched_count, inserted_count, updated_count,
			unchanged_count, COALESCE(earliest_date::TEXT,''), COALESCE(latest_date::TEXT,''),
			COALESCE(error_message,''), started_at, completed_at
		FROM coenzyme_q10_sync_runs
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return status, fmt.Errorf("list coenzyme Q10 sync runs: %w", err)
	}
	defer rows.Close()
	status.Recent = make([]CoenzymeQ10SyncRun, 0, limit)
	for rows.Next() {
		var run CoenzymeQ10SyncRun
		if err := rows.Scan(
			&run.RunID, &run.SheetName, &run.Status, &run.Fetched, &run.Inserted, &run.Updated,
			&run.Unchanged, &run.EarliestDate, &run.LatestDate, &run.ErrorMessage,
			&run.StartedAt, &run.CompletedAt,
		); err != nil {
			return status, fmt.Errorf("scan coenzyme Q10 sync run: %w", err)
		}
		status.Recent = append(status.Recent, run)
	}
	if err := rows.Err(); err != nil {
		return status, fmt.Errorf("iterate coenzyme Q10 sync runs: %w", err)
	}
	return status, nil
}

const coenzymeQ10DailyUpsert = `
	INSERT INTO coenzyme_q10_daily (
		report_date, spend, impressions, clicks, ctr, cpc, cpm, all_transaction_gmv,
		all_store_roi, post_refund_gmv, post_refund_roi, coenzyme_gmv, coenzyme_roi,
		same_day_gmv, same_day_roi, search_spend, search_gmv, search_roi,
		search_spend_ratio, source_row_number, content_hash, sync_run_id
	)
	SELECT report_date, spend, impressions, clicks, ctr, cpc, cpm, all_transaction_gmv,
		all_store_roi, post_refund_gmv, post_refund_roi, coenzyme_gmv, coenzyme_roi,
		same_day_gmv, same_day_roi, search_spend, search_gmv, search_roi,
		search_spend_ratio, source_row_number, content_hash, $1
	FROM coenzyme_q10_daily_stage
	ON CONFLICT (report_date) DO UPDATE SET
		spend=EXCLUDED.spend, impressions=EXCLUDED.impressions, clicks=EXCLUDED.clicks,
		ctr=EXCLUDED.ctr, cpc=EXCLUDED.cpc, cpm=EXCLUDED.cpm,
		all_transaction_gmv=EXCLUDED.all_transaction_gmv, all_store_roi=EXCLUDED.all_store_roi,
		post_refund_gmv=EXCLUDED.post_refund_gmv, post_refund_roi=EXCLUDED.post_refund_roi,
		coenzyme_gmv=EXCLUDED.coenzyme_gmv, coenzyme_roi=EXCLUDED.coenzyme_roi,
		same_day_gmv=EXCLUDED.same_day_gmv, same_day_roi=EXCLUDED.same_day_roi,
		search_spend=EXCLUDED.search_spend, search_gmv=EXCLUDED.search_gmv,
		search_roi=EXCLUDED.search_roi, search_spend_ratio=EXCLUDED.search_spend_ratio,
		source_row_number=EXCLUDED.source_row_number, content_hash=EXCLUDED.content_hash,
		sync_run_id=EXCLUDED.sync_run_id, updated_at=NOW()
	WHERE coenzyme_q10_daily.content_hash IS DISTINCT FROM EXCLUDED.content_hash
`
