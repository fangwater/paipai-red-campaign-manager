package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5"
)

var (
	ErrGuoraiSyncLocked          = errors.New("another Guorai sync is already running")
	ErrGuoraiCredentialsNotFound = errors.New("Guorai credentials are not stored in PostgreSQL")
)

type GuoraiCredentials struct {
	Username string
	Password string
}

func (p *Postgres) SaveGuoraiCredentials(ctx context.Context, credentials GuoraiCredentials) error {
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		return errors.New("Guorai username and password are required")
	}
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO guorai_credentials (credential_key, username, password_value)
		VALUES ('default', $1, $2)
		ON CONFLICT (credential_key) DO UPDATE SET
			username = EXCLUDED.username,
			password_value = EXCLUDED.password_value,
			updated_at = NOW()
	`, credentials.Username, credentials.Password); err != nil {
		return fmt.Errorf("save Guorai credentials: %w", err)
	}
	return nil
}

func (p *Postgres) LoadGuoraiCredentials(ctx context.Context) (GuoraiCredentials, error) {
	var credentials GuoraiCredentials
	err := p.pool.QueryRow(ctx, `
		SELECT username, password_value
		FROM guorai_credentials
		WHERE credential_key = 'default'
	`).Scan(&credentials.Username, &credentials.Password)
	if errors.Is(err, pgx.ErrNoRows) {
		return GuoraiCredentials{}, ErrGuoraiCredentialsNotFound
	}
	if err != nil {
		return GuoraiCredentials{}, fmt.Errorf("load Guorai credentials: %w", err)
	}
	return credentials, nil
}

func (p *Postgres) AcquireGuoraiSyncLock(ctx context.Context) (func(), error) {
	connection, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire PostgreSQL connection for Guorai sync lock: %w", err)
	}
	var locked bool
	if err := connection.QueryRow(ctx,
		"SELECT pg_try_advisory_lock(hashtextextended('guorai:full-sync', 0))",
	).Scan(&locked); err != nil {
		connection.Release()
		return nil, fmt.Errorf("acquire Guorai sync lock: %w", err)
	}
	if !locked {
		connection.Release()
		return nil, ErrGuoraiSyncLocked
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = connection.Exec(unlockCtx,
				"SELECT pg_advisory_unlock(hashtextextended('guorai:full-sync', 0))",
			)
			connection.Release()
		})
	}, nil
}

type guoraiMetricField struct {
	column string
	json   string
	cast   string
}

var commonGuoraiMetricFields = []guoraiMetricField{
	{"total_pay_user", "totalPayUser", "numeric"}, {"total_pay_amt", "totalPayAmt", "numeric"},
	{"total_pay_order", "totalPayOrder", "numeric"}, {"total_uroi", "totalUroi", "numeric"},
	{"total_oroi", "totalOroi", "numeric"}, {"total_roi", "totalRoi", "numeric"},
	{"part_pay_user", "partPayUser", "numeric"}, {"part_pay_amt", "partPayAmt", "numeric"},
	{"part_pay_order", "partPayOrder", "numeric"}, {"part_pay_user_r", "partPayUserR", "numeric"},
	{"part_pay_amt_r", "partPayAmtR", "numeric"}, {"part_pay_order_r", "partPayOrderR", "numeric"},
	{"part_uroi", "partUroi", "numeric"}, {"part_oroi", "partOroi", "numeric"},
	{"part_roi", "partRoi", "numeric"}, {"new_pay_user", "newPayUser", "numeric"},
	{"new_pay_amt", "newPayAmt", "numeric"}, {"new_pay_order", "newPayOrder", "numeric"},
	{"new_pay_user_r", "newPayUserR", "numeric"}, {"new_pay_amt_r", "newPayAmtR", "numeric"},
	{"new_pay_order_r", "newPayOrderR", "numeric"}, {"note_ad_cost_volume", "noteAdCostVolume", "numeric"},
	{"exposure_count", "exposureCount", "bigint"}, {"click_count", "clickCount", "bigint"},
	{"interact_count", "interactCount", "bigint"}, {"click_r", "clickR", "numeric"},
	{"interact_r", "interactR", "numeric"}, {"click_roi", "clickRoi", "numeric"},
	{"note_endorse_volume", "noteEndorseVolume", "bigint"}, {"note_comment_volume", "noteCommentVolume", "bigint"},
	{"note_collect_volume", "noteCollectVolume", "bigint"}, {"note_share_volume", "noteShareVolume", "bigint"},
	{"note_follow_volume", "noteFollowVolume", "bigint"}, {"is_new", "isNew", "boolean"},
}

var noteOnlyGuoraiMetricFields = []guoraiMetricField{
	{"consume", "consume", "numeric"}, {"note_consume", "noteConsume", "numeric"},
	{"note_heat_consume", "noteHeatConsume", "numeric"},
}

func (p *Postgres) SaveGuoraiSnapshot(ctx context.Context, snapshot model.GuoraiSnapshot) (result model.GuoraiStoreResult, err error) {
	if snapshot.EntityType != "note" && snapshot.EntityType != "plan" {
		return result, fmt.Errorf("unsupported Guorai entity type %q", snapshot.EntityType)
	}
	records, err := decodeGuoraiRecords(snapshot.Records)
	if err != nil {
		return result, err
	}
	if !json.Valid(snapshot.RequestPayload) || !json.Valid(snapshot.RawResponse) {
		return result, errors.New("Guorai snapshot contains invalid request or response JSON")
	}
	err = p.pool.QueryRow(ctx, `
		INSERT INTO guorai_fetch_runs (
			entity_type, enterprise_id, xhs_brand_id, brand_name, merchant_id,
			attribution_shop, window_start, window_end, snapshot_date, source_cutoff_date,
			attribution_type, attribution_model, attribution_window_days, traffic_type,
			status, row_count, request_payload, raw_response
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,0),$14,'running',$15,$16::jsonb,$17::jsonb)
		RETURNING id
	`, snapshot.EntityType, snapshot.EnterpriseID, snapshot.XHSBrandID, snapshot.BrandName, snapshot.MerchantID,
		snapshot.AttributionShop, snapshot.WindowStart, snapshot.WindowEnd, snapshot.SnapshotDate, snapshot.SourceCutoffDate,
		snapshot.AttributionType, snapshot.AttributionModel, snapshot.AttributionWindowDays, snapshot.TrafficType,
		len(records), string(snapshot.RequestPayload), string(snapshot.RawResponse)).Scan(&result.FetchID)
	if err != nil {
		return result, fmt.Errorf("start Guorai fetch run: %w", err)
	}
	result.Rows = len(records)
	defer func() {
		if err == nil {
			return
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = p.pool.Exec(finishCtx, `UPDATE guorai_fetch_runs
			SET status='failed',finished_at=NOW(),error_message=$2 WHERE id=$1`, result.FetchID, err.Error())
	}()

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin Guorai snapshot transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	lockKey := fmt.Sprintf("guorai:%s:%d:%s:%s", snapshot.EntityType, snapshot.EnterpriseID,
		snapshot.XHSBrandID, snapshot.SnapshotDate.Format(time.DateOnly))
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))", lockKey).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire Guorai sync lock: %w", err)
	}
	if !locked {
		return result, ErrGuoraiSyncLocked
	}
	if snapshot.EntityType == "note" {
		err = saveGuoraiNoteRecords(ctx, tx, result.FetchID, snapshot, records)
	} else {
		err = saveGuoraiPlanRecords(ctx, tx, result.FetchID, snapshot, records)
	}
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit Guorai snapshot transaction: %w", err)
	}
	if _, err = p.pool.Exec(ctx, `UPDATE guorai_fetch_runs
		SET status='succeeded',finished_at=NOW(),error_message=NULL WHERE id=$1`, result.FetchID); err != nil {
		return result, fmt.Errorf("finish Guorai fetch run: %w", err)
	}
	return result, nil
}

func decodeGuoraiRecords(data []byte) ([]map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var records []map[string]any
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("decode Guorai records: %w", err)
	}
	return records, nil
}

func saveGuoraiNoteRecords(ctx context.Context, tx pgx.Tx, fetchID int64, snapshot model.GuoraiSnapshot, records []map[string]any) error {
	batch := &pgx.Batch{}
	for _, record := range records {
		noteID := guoraiString(record["noteId"])
		if noteID == "" {
			continue
		}
		raw, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode Guorai note %s: %w", noteID, err)
		}
		queueGuoraiNoteDimension(batch, record, raw)
		batch.Queue(`INSERT INTO guorai_note_snapshots (
			fetch_id,note_id,enterprise_id,xhs_brand_id,merchant_id,snapshot_date,window_start,window_end,raw_payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`, fetchID, noteID, snapshot.EnterpriseID,
			snapshot.XHSBrandID, snapshot.MerchantID, snapshot.SnapshotDate, snapshot.WindowStart, snapshot.WindowEnd, string(raw))
	}
	if err := executeGuoraiBatch(ctx, tx, batch); err != nil {
		return err
	}
	fields := append(append([]guoraiMetricField{}, commonGuoraiMetricFields...), noteOnlyGuoraiMetricFields...)
	return expandGuoraiMetrics(ctx, tx, "guorai_note_snapshots", fields, fetchID)
}

func saveGuoraiPlanRecords(ctx context.Context, tx pgx.Tx, fetchID int64, snapshot model.GuoraiSnapshot, records []map[string]any) error {
	planIDs := make([]string, 0, len(records))
	batch := &pgx.Batch{}
	for _, record := range records {
		planID := guoraiString(record["planId"])
		if planID == "" {
			continue
		}
		planIDs = append(planIDs, planID)
		raw, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode Guorai plan %s: %w", planID, err)
		}
		queueGuoraiPlanDimension(batch, record, raw)
		batch.Queue(`INSERT INTO guorai_plan_snapshots (
			fetch_id,plan_id,enterprise_id,xhs_brand_id,merchant_id,snapshot_date,window_start,window_end,raw_payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`, fetchID, planID, snapshot.EnterpriseID,
			snapshot.XHSBrandID, snapshot.MerchantID, snapshot.SnapshotDate, snapshot.WindowStart, snapshot.WindowEnd, string(raw))
	}
	if len(planIDs) > 0 {
		if _, err := tx.Exec(ctx, "UPDATE guorai_plan_notes SET is_active=FALSE WHERE plan_id=ANY($1)", planIDs); err != nil {
			return fmt.Errorf("deactivate Guorai plan-note relations: %w", err)
		}
	}
	for _, record := range records {
		planID := guoraiString(record["planId"])
		notes, _ := record["noteList"].([]any)
		for _, value := range notes {
			note, ok := value.(map[string]any)
			if !ok || guoraiString(note["noteId"]) == "" {
				continue
			}
			raw, err := json.Marshal(note)
			if err != nil {
				return fmt.Errorf("encode nested Guorai note: %w", err)
			}
			queueGuoraiNoteDimension(batch, note, raw)
			batch.Queue(`INSERT INTO guorai_plan_notes (plan_id,note_id,first_seen_at,last_seen_at,is_active,raw_payload)
				VALUES ($1,$2,NOW(),NOW(),TRUE,$3::jsonb)
				ON CONFLICT (plan_id,note_id) DO UPDATE SET last_seen_at=NOW(),is_active=TRUE,raw_payload=EXCLUDED.raw_payload`,
				planID, guoraiString(note["noteId"]), string(raw))
		}
	}
	if err := executeGuoraiBatch(ctx, tx, batch); err != nil {
		return err
	}
	return expandGuoraiMetrics(ctx, tx, "guorai_plan_snapshots", commonGuoraiMetricFields, fetchID)
}
