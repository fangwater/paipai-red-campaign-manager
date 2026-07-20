package store

import (
	"context"
	"errors"
	"fmt"

	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5"
)

var ErrProviderSyncLocked = errors.New("another sync for this service provider is already running")

func (p *Postgres) ProviderContentTables(ctx context.Context) ([]model.ProviderContentTable, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT provider_code, provider_name, COALESCE(source_url, ''), COALESCE(wiki_token, ''),
			COALESCE(spreadsheet_token, ''), COALESCE(sheet_id, ''), sheet_name, last_synced_at
		FROM service_provider_content_tables
		WHERE enabled
		ORDER BY provider_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query service-provider content tables: %w", err)
	}
	defer rows.Close()

	tables := make([]model.ProviderContentTable, 0)
	for rows.Next() {
		var table model.ProviderContentTable
		if err := rows.Scan(
			&table.ProviderCode,
			&table.ProviderName,
			&table.SourceURL,
			&table.WikiToken,
			&table.SpreadsheetToken,
			&table.SheetID,
			&table.SheetName,
			&table.LastSyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scan service-provider content table: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service-provider content tables: %w", err)
	}
	return tables, nil
}

func (p *Postgres) MarkProviderContentSyncStarted(ctx context.Context, providerCode string) error {
	commandTag, err := p.pool.Exec(ctx, `
		UPDATE service_provider_content_tables
		SET last_sync_status = 'running', last_sync_error = NULL, updated_at = NOW()
		WHERE provider_code = $1 AND enabled
	`, providerCode)
	if err != nil {
		return fmt.Errorf("mark provider sync started: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("enabled service provider %q was not found", providerCode)
	}
	return nil
}

func (p *Postgres) MarkProviderContentSyncFailed(ctx context.Context, providerCode string, syncErr error) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE service_provider_content_tables
		SET last_sync_status = 'failed', last_sync_error = $2, updated_at = NOW()
		WHERE provider_code = $1
	`, providerCode, syncErr.Error())
	if err != nil {
		return fmt.Errorf("mark provider sync failed: %w", err)
	}
	return nil
}

func (p *Postgres) ReplaceProviderContentSnapshot(ctx context.Context, snapshot model.ProviderContentSnapshot) (model.ProviderSyncResult, error) {
	result := model.ProviderSyncResult{
		Providers: 1,
		Fetched:   len(snapshot.Records),
		Upserted:  len(snapshot.Records),
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin provider sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var locked bool
	lockKey := "provider-content:" + snapshot.Table.ProviderCode
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", lockKey).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire provider sync lock: %w", err)
	}
	if !locked {
		return result, ErrProviderSyncLocked
	}
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE sync_provider_record_keys (
			record_key TEXT PRIMARY KEY
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create provider sync staging table: %w", err)
	}

	batch := &pgx.Batch{}
	for _, record := range snapshot.Records {
		batch.Queue("INSERT INTO sync_provider_record_keys (record_key) VALUES ($1)", record.RecordKey)
		batch.Queue(`
			INSERT INTO service_provider_note_executions (
				provider_code, record_key, source_row_number, submission_date, note_id,
				content_type, cover_type, commercial_intensity, audience, user_scenario,
				note_type, progress, review_feedback, synced_at, deleted_at
			) VALUES (
				$1, $2, $3, NULLIF($4, ''), NULLIF($5, ''),
				NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
				NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), NOW(), NULL
			)
			ON CONFLICT (provider_code, record_key) DO UPDATE SET
				source_row_number = EXCLUDED.source_row_number,
				submission_date = EXCLUDED.submission_date,
				note_id = EXCLUDED.note_id,
				content_type = EXCLUDED.content_type,
				cover_type = EXCLUDED.cover_type,
				commercial_intensity = EXCLUDED.commercial_intensity,
				audience = EXCLUDED.audience,
				user_scenario = EXCLUDED.user_scenario,
				note_type = EXCLUDED.note_type,
				progress = EXCLUDED.progress,
				review_feedback = EXCLUDED.review_feedback,
				synced_at = NOW(),
				deleted_at = NULL
		`, snapshot.Table.ProviderCode, record.RecordKey, record.SourceRowNumber,
			record.SubmissionDate, record.NoteID, record.ContentType, record.CoverType,
			record.CommercialIntensity, record.Audience, record.UserScenario,
			record.NoteType, record.Progress, record.ReviewFeedback)
	}
	if batch.Len() > 0 {
		results := tx.SendBatch(ctx, batch)
		for index := 0; index < batch.Len(); index++ {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return result, fmt.Errorf("execute provider snapshot batch item %d: %w", index+1, err)
			}
		}
		if err := results.Close(); err != nil {
			return result, fmt.Errorf("close provider snapshot batch: %w", err)
		}
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE service_provider_note_executions AS target
		SET deleted_at = NOW(), synced_at = NOW()
		WHERE target.provider_code = $1
		  AND target.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM sync_provider_record_keys AS source
			WHERE source.record_key = target.record_key
		  )
	`, snapshot.Table.ProviderCode)
	if err != nil {
		return result, fmt.Errorf("mark missing provider records deleted: %w", err)
	}
	result.Deleted = commandTag.RowsAffected()

	if _, err := tx.Exec(ctx, `
		UPDATE service_provider_content_tables
		SET spreadsheet_token = NULLIF($2, ''), sheet_id = NULLIF($3, ''), sheet_name = $4,
			last_synced_at = NOW(), last_sync_status = 'succeeded', last_sync_error = NULL, updated_at = NOW()
		WHERE provider_code = $1
	`, snapshot.Table.ProviderCode, snapshot.Table.SpreadsheetToken, snapshot.Table.SheetID, snapshot.Table.SheetName); err != nil {
		return result, fmt.Errorf("update provider sync index: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit provider sync transaction: %w", err)
	}
	return result, nil
}
