package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"paipai-red-campaign-manager/internal/model"
	"paipai-red-campaign-manager/internal/providercontent"

	"github.com/jackc/pgx/v5"
)

var ErrProviderSyncLocked = errors.New("another sync for this service provider is already running")

func (p *Postgres) ProviderContentTables(ctx context.Context) ([]model.ProviderContentTable, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT provider_code, provider_name, COALESCE(source_url, ''), COALESCE(wiki_token, ''),
			COALESCE(spreadsheet_token, ''), COALESCE(sheet_id, ''), sheet_name, last_synced_at,
			last_sync_status, COALESCE(last_sync_error, '')
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
			&table.LastSyncStatus,
			&table.LastSyncError,
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

func (p *Postgres) ProviderNotesToFetch(ctx context.Context, refs []model.DocumentRef) ([]model.DocumentRef, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	noteIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		noteIDs = append(noteIDs, ref.RecordID)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT note_id, COALESCE(source_resource_key, ''), extractor_version
		FROM service_provider_notes
		WHERE note_id = ANY($1::text[])
	`, noteIDs)
	if err != nil {
		return nil, fmt.Errorf("query existing provider notes: %w", err)
	}
	defer rows.Close()
	type savedNote struct {
		resourceKey      string
		extractorVersion int
	}
	existing := make(map[string]savedNote, len(noteIDs))
	for rows.Next() {
		var noteID string
		var saved savedNote
		if err := rows.Scan(&noteID, &saved.resourceKey, &saved.extractorVersion); err != nil {
			return nil, fmt.Errorf("scan existing provider note: %w", err)
		}
		existing[noteID] = saved
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing provider notes: %w", err)
	}

	candidates := make([]model.DocumentRef, 0, len(refs))
	for _, ref := range refs {
		saved, ok := existing[ref.RecordID]
		if !ok || saved.resourceKey != ref.ResourceKey ||
			saved.extractorVersion < model.ManuscriptExtractorVersion {
			candidates = append(candidates, ref)
		}
	}
	return candidates, nil
}

func (p *Postgres) UpdateProviderNoteSources(ctx context.Context, providerCode string, refs []model.DocumentRef) error {
	if len(refs) == 0 {
		return nil
	}
	noteIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		noteIDs = append(noteIDs, ref.RecordID)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT note_id, note_content
		FROM service_provider_notes
		WHERE note_id = ANY($1::text[])
	`, noteIDs)
	if err != nil {
		return fmt.Errorf("query provider note contents for source titles: %w", err)
	}
	contents := make(map[string]string, len(noteIDs))
	for rows.Next() {
		var noteID, content string
		if err := rows.Scan(&noteID, &content); err != nil {
			rows.Close()
			return fmt.Errorf("scan provider note content for source title: %w", err)
		}
		contents[noteID] = content
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate provider note contents for source titles: %w", err)
	}
	rows.Close()

	batch := &pgx.Batch{}
	for _, ref := range refs {
		sourceTitle := providercontent.SourceTitle(providerCode, contents[ref.RecordID], ref.Label, "")
		batch.Queue(`
			UPDATE service_provider_notes
			SET source_title = COALESCE(NULLIF(BTRIM($2), ''), source_title),
				source_url = COALESCE(NULLIF(BTRIM($3), ''), source_url)
			WHERE note_id = $1
		`, ref.RecordID, sourceTitle, ref.SourceURL)
	}
	results := p.pool.SendBatch(ctx, batch)
	for index := 0; index < batch.Len(); index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("update provider note source batch item %d: %w", index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close provider note source batch: %w", err)
	}
	return nil
}

func (p *Postgres) FailRunningProviderContentSyncs(ctx context.Context, reason string) error {
	if reason == "" {
		reason = "manual sync service restarted before the request finished"
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE service_provider_content_tables
		SET last_sync_status = 'failed', last_sync_error = $1, updated_at = NOW()
		WHERE last_sync_status = 'running'
	`, reason)
	if err != nil {
		return fmt.Errorf("recover interrupted provider content syncs: %w", err)
	}
	return nil
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

func (p *Postgres) UpsertProviderNotes(ctx context.Context, notes []model.ProviderNote) error {
	if len(notes) == 0 {
		return nil
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin provider note transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	batch := &pgx.Batch{}
	if err := queueProviderNotes(batch, notes); err != nil {
		return err
	}
	results := tx.SendBatch(ctx, batch)
	for index := 0; index < batch.Len(); index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("execute provider note batch item %d: %w", index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close provider note batch: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM manuscript_assets AS assets
		WHERE NOT EXISTS (
			SELECT 1 FROM service_provider_note_assets AS links
			WHERE links.asset_id = assets.asset_id
		)
	`); err != nil {
		return fmt.Errorf("delete unreferenced manuscript assets: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit provider note transaction: %w", err)
	}
	return nil
}

func (p *Postgres) ReplaceProviderContentSnapshot(ctx context.Context, snapshot model.ProviderContentSnapshot) (model.ProviderSyncResult, error) {
	result := model.ProviderSyncResult{
		Providers:  1,
		Fetched:    len(snapshot.Records),
		Upserted:   len(snapshot.Records),
		Notes:      len(snapshot.Notes),
		NoteErrors: snapshot.NoteErrors,
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
				cover_type, commercial_intensity, audience, user_scenario, note_type,
				progress, synced_at, deleted_at
			) VALUES (
				$1, $2, $3, NULLIF($4, ''), NULLIF($5, ''),
				NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
				NULLIF($11, ''), NOW(), NULL
			)
			ON CONFLICT (provider_code, record_key) DO UPDATE SET
				source_row_number = EXCLUDED.source_row_number,
				submission_date = EXCLUDED.submission_date,
				note_id = EXCLUDED.note_id,
				cover_type = EXCLUDED.cover_type,
				commercial_intensity = EXCLUDED.commercial_intensity,
				audience = EXCLUDED.audience,
				user_scenario = EXCLUDED.user_scenario,
				note_type = EXCLUDED.note_type,
				progress = EXCLUDED.progress,
				synced_at = NOW(),
				deleted_at = NULL
		`, snapshot.Table.ProviderCode, record.RecordKey, record.SourceRowNumber,
			record.SubmissionDate, record.NoteID, record.CoverType, record.CommercialIntensity,
			record.Audience, record.UserScenario, record.NoteType, record.Progress)
	}
	if err := queueProviderNotes(batch, snapshot.Notes); err != nil {
		return result, err
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
	if _, err := tx.Exec(ctx, `
		DELETE FROM manuscript_assets AS assets
		WHERE NOT EXISTS (
			SELECT 1 FROM service_provider_note_assets AS links
			WHERE links.asset_id = assets.asset_id
		)
	`); err != nil {
		return result, fmt.Errorf("delete unreferenced manuscript assets: %w", err)
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

func queueProviderNotes(batch *pgx.Batch, notes []model.ProviderNote) error {
	for _, note := range notes {
		blocks := note.ContentBlocks
		if blocks == nil {
			blocks = []model.ManuscriptBlock{}
		}
		referenceNoteIDs := note.ReferenceNoteIDs
		if referenceNoteIDs == nil {
			referenceNoteIDs = []string{}
		}
		blocksJSON, err := json.Marshal(blocks)
		if err != nil {
			return fmt.Errorf("encode provider note %s blocks: %w", note.NoteID, err)
		}
		for _, asset := range note.Assets {
			batch.Queue(`
				INSERT INTO manuscript_assets (
					asset_id, content_type, byte_size, width, height, content
				) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (asset_id) DO NOTHING
			`, asset.AssetID, asset.ContentType, len(asset.Content), asset.Width, asset.Height, asset.Content)
		}
		extractorVersion := note.ExtractorVersion
		if extractorVersion <= 0 {
			extractorVersion = 1
		}
		batch.Queue(`
			INSERT INTO service_provider_notes (
				note_id, note_content, content_blocks, reference_note_ids, source_title, source_url,
				source_resource_key, source_revision, extractor_version, updated_at
			) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9, NOW())
			ON CONFLICT (note_id) DO UPDATE SET
				note_content = EXCLUDED.note_content,
				content_blocks = EXCLUDED.content_blocks,
				reference_note_ids = EXCLUDED.reference_note_ids,
				source_title = COALESCE(NULLIF(EXCLUDED.source_title, ''), service_provider_notes.source_title),
				source_url = EXCLUDED.source_url,
				source_resource_key = EXCLUDED.source_resource_key,
				source_revision = EXCLUDED.source_revision,
				extractor_version = EXCLUDED.extractor_version,
				updated_at = NOW()
		`, note.NoteID, note.NoteContent, blocksJSON, referenceNoteIDs, note.SourceTitle,
			note.SourceURL, note.SourceResourceKey, note.SourceRevision, extractorVersion)
		batch.Queue("DELETE FROM service_provider_note_assets WHERE note_id = $1", note.NoteID)
		for position, block := range blocks {
			if block.Type != "image" || block.AssetID == "" {
				continue
			}
			batch.Queue(`
				INSERT INTO service_provider_note_assets (
					note_id, position, asset_id, width, height, caption
				) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
			`, note.NoteID, position, block.AssetID, block.Width, block.Height, block.Caption)
		}
	}
	return nil
}
