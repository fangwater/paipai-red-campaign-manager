package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"paipai-red-campaign-manager/internal/model"
	"paipai-red-campaign-manager/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSyncLocked = errors.New("another sync for this Bitable is already running")

type Postgres struct {
	pool        *pgxpool.Pool
	appToken    string
	tableScoped bool
}

func NewPostgres(ctx context.Context, databaseURL, appToken string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return &Postgres{pool: pool, appToken: appToken}, nil
}

// NewTableScopedPostgres reconciles only tables present in each snapshot. It is
// intended for independent single-table sync targets that share a Base token.
func NewTableScopedPostgres(ctx context.Context, databaseURL, appToken string) (*Postgres, error) {
	postgres, err := NewPostgres(ctx, databaseURL, appToken)
	if err != nil {
		return nil, err
	}
	postgres.tableScoped = true
	return postgres, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, migrations.InitSQL+"\n"+migrations.ProviderContentSQL+"\n"+migrations.GuoraiSQL+"\n"+migrations.XHSJGCampaignsSQL+"\n"+migrations.XHSJGDeliverySQL+"\n"+migrations.XHSJGManualSyncSQL+"\n"+migrations.MaituoCustomerDailySQL+"\n"+migrations.MaituoNoteReportDatesSQL+"\n"+migrations.MaituoPartialWorkbooksSQL+"\n"+migrations.MaituoDatedSummaryTablesSQL+"\n"+migrations.MaituoRemoveImportVersionSQL+"\n"+migrations.SimilarNoteEmbeddingsSQL+"\n"+migrations.GuoraiCredentialsSQL+"\n"+migrations.MaituoSearchUserOverlapSQL); err != nil {
		return fmt.Errorf("apply PostgreSQL migration: %w", err)
	}
	return nil
}

func (p *Postgres) DocumentsToRefresh(ctx context.Context, refs []model.DocumentRef, staleBefore time.Time) ([]model.DocumentRef, error) {
	fresh := make(map[string]struct{})
	rows, err := p.pool.Query(ctx, `
		SELECT provider, resource_key
		FROM lark_linked_documents
		WHERE fetched_at >= $1
	`, staleBefore)
	if err != nil {
		return nil, fmt.Errorf("query fresh linked documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider, resourceKey string
		if err := rows.Scan(&provider, &resourceKey); err != nil {
			return nil, fmt.Errorf("scan fresh linked document: %w", err)
		}
		fresh[provider+"\x00"+resourceKey] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fresh linked documents: %w", err)
	}

	candidates := make([]model.DocumentRef, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		key := ref.Provider + "\x00" + ref.ResourceKey
		if _, ok := fresh[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, ref)
	}
	return candidates, nil
}

func (p *Postgres) ReplaceSnapshot(ctx context.Context, snapshot model.Snapshot, documents []model.Document) (result model.SyncResult, err error) {
	result.Tables = len(snapshot.Tables)
	result.Documents = len(documents)
	for _, table := range snapshot.Tables {
		result.Fetched += len(table.Records)
	}
	for _, document := range documents {
		if document.Status != "succeeded" {
			result.DocumentErrors++
		}
	}

	runTableID := ""
	if p.tableScoped && len(snapshot.Tables) == 1 {
		runTableID = snapshot.Tables[0].ID
	}
	runID, err := p.startRun(ctx, runTableID)
	if err != nil {
		return result, err
	}
	defer func() {
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err != nil {
			_ = p.finishRun(finishCtx, runID, "failed", result, err)
			return
		}
		_ = p.finishRun(finishCtx, runID, "succeeded", result, nil)
	}()

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", p.appToken).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire sync lock: %w", err)
	}
	if !locked {
		return result, ErrSyncLocked
	}

	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE sync_table_ids (
			table_id TEXT PRIMARY KEY
		) ON COMMIT DROP;
		CREATE TEMP TABLE sync_record_ids (
			table_id TEXT NOT NULL,
			record_id TEXT NOT NULL,
			PRIMARY KEY (table_id, record_id)
		) ON COMMIT DROP;
		CREATE TEMP TABLE sync_document_refs (
			table_id TEXT NOT NULL,
			record_id TEXT NOT NULL,
			field_name TEXT NOT NULL,
			provider TEXT NOT NULL,
			resource_key TEXT NOT NULL,
			PRIMARY KEY (table_id, record_id, field_name, provider, resource_key)
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create sync staging tables: %w", err)
	}

	batch := &pgx.Batch{}
	queued := 0
	for _, table := range snapshot.Tables {
		batch.Queue("INSERT INTO sync_table_ids (table_id) VALUES ($1)", table.ID)
		batch.Queue(`
			INSERT INTO lark_bitable_tables (app_token, table_id, name, revision, synced_at, deleted_at)
			VALUES ($1, $2, $3, $4, NOW(), NULL)
			ON CONFLICT (app_token, table_id) DO UPDATE SET
				name = EXCLUDED.name,
				revision = EXCLUDED.revision,
				synced_at = NOW(),
				deleted_at = NULL
		`, p.appToken, table.ID, table.Name, table.Revision)
		queued += 2

		for _, record := range table.Records {
			batch.Queue("INSERT INTO sync_record_ids (table_id, record_id) VALUES ($1, $2)", table.ID, record.ID)
			batch.Queue(`
				INSERT INTO lark_bitable_records (
					app_token, table_id, record_id, fields,
					lark_created_at, lark_updated_at, synced_at, deleted_at
				) VALUES ($1, $2, $3, $4::jsonb, $5, $6, NOW(), NULL)
				ON CONFLICT (app_token, table_id, record_id) DO UPDATE SET
					fields = EXCLUDED.fields,
					lark_created_at = EXCLUDED.lark_created_at,
					lark_updated_at = EXCLUDED.lark_updated_at,
					synced_at = NOW(),
					deleted_at = NULL
			`, p.appToken, table.ID, record.ID, string(record.Fields), record.CreatedAt, record.UpdatedAt)
			queued += 2
			result.Upserted++
		}
	}

	for _, document := range documents {
		batch.Queue(`
			INSERT INTO lark_linked_documents (
				provider, resource_key, source_url, document_type, title, content,
				revision_id, fetch_status, error_message, fetched_at, last_seen_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), NOW(), NOW())
			ON CONFLICT (provider, resource_key) DO UPDATE SET
				source_url = EXCLUDED.source_url,
				document_type = EXCLUDED.document_type,
				title = CASE WHEN EXCLUDED.fetch_status = 'succeeded' THEN EXCLUDED.title ELSE lark_linked_documents.title END,
				content = CASE
					WHEN EXCLUDED.fetch_status = 'succeeded' OR EXCLUDED.provider IN ('weixin', 'tencent') THEN EXCLUDED.content
					ELSE lark_linked_documents.content
				END,
				revision_id = CASE WHEN EXCLUDED.fetch_status = 'succeeded' THEN EXCLUDED.revision_id ELSE lark_linked_documents.revision_id END,
				fetch_status = EXCLUDED.fetch_status,
				error_message = EXCLUDED.error_message,
				fetched_at = NOW(),
				last_seen_at = NOW()
		`, document.Provider, document.ResourceKey, document.SourceURL, document.DocumentType, document.Title,
			document.Content, document.RevisionID, document.Status, document.ErrorMessage)
		queued++
	}

	for _, ref := range snapshot.DocumentRefs {
		batch.Queue(`
			INSERT INTO sync_document_refs (table_id, record_id, field_name, provider, resource_key)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, ref.TableID, ref.RecordID, ref.FieldName, ref.Provider, ref.ResourceKey)
		batch.Queue(`
			INSERT INTO lark_record_documents (
				app_token, table_id, record_id, field_name, provider, resource_key, source_url, last_seen_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (app_token, table_id, record_id, field_name, provider, resource_key) DO UPDATE SET
				source_url = EXCLUDED.source_url,
				last_seen_at = NOW()
		`, p.appToken, ref.TableID, ref.RecordID, ref.FieldName, ref.Provider, ref.ResourceKey, ref.SourceURL)
		batch.Queue(`
			UPDATE lark_linked_documents
			SET last_seen_at = NOW(), source_url = $3
			WHERE provider = $1 AND resource_key = $2
		`, ref.Provider, ref.ResourceKey, ref.SourceURL)
		queued += 3
	}

	results := tx.SendBatch(ctx, batch)
	for i := 0; i < queued; i++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return result, fmt.Errorf("execute snapshot batch item %d: %w", i+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return result, fmt.Errorf("close snapshot batch: %w", err)
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE lark_bitable_records AS target
		SET deleted_at = NOW(), synced_at = NOW()
		WHERE target.app_token = $1
		  AND target.deleted_at IS NULL
		  AND ($2 = FALSE OR EXISTS (
			SELECT 1 FROM sync_table_ids AS scope
			WHERE scope.table_id = target.table_id
		  ))
		  AND NOT EXISTS (
			SELECT 1
			FROM sync_record_ids AS source
			WHERE source.table_id = target.table_id
			  AND source.record_id = target.record_id
		  )
	`, p.appToken, p.tableScoped)
	if err != nil {
		return result, fmt.Errorf("mark missing records deleted: %w", err)
	}
	result.Deleted = commandTag.RowsAffected()

	if !p.tableScoped {
		if _, err := tx.Exec(ctx, `
			UPDATE lark_bitable_tables AS target
			SET deleted_at = NOW(), synced_at = NOW()
			WHERE target.app_token = $1
			  AND target.deleted_at IS NULL
			  AND NOT EXISTS (
				SELECT 1 FROM sync_table_ids AS source
				WHERE source.table_id = target.table_id
			  )
		`, p.appToken); err != nil {
			return result, fmt.Errorf("mark missing tables deleted: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM lark_record_documents AS target
		WHERE target.app_token = $1
		  AND ($2 = FALSE OR EXISTS (
			SELECT 1 FROM sync_table_ids AS scope
			WHERE scope.table_id = target.table_id
		  ))
		  AND NOT EXISTS (
			SELECT 1
			FROM sync_document_refs AS source
			WHERE source.table_id = target.table_id
			  AND source.record_id = target.record_id
			  AND source.field_name = target.field_name
			  AND source.provider = target.provider
			  AND source.resource_key = target.resource_key
		  )
	`, p.appToken, p.tableScoped); err != nil {
		return result, fmt.Errorf("delete stale document references: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit sync transaction: %w", err)
	}
	return result, nil
}

func (p *Postgres) startRun(ctx context.Context, tableID string) (int64, error) {
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO sync_runs (app_token, table_id, status)
		VALUES ($1, $2, 'running')
		RETURNING id
	`, p.appToken, tableID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create sync run: %w", err)
	}
	return id, nil
}

func (p *Postgres) finishRun(ctx context.Context, runID int64, status string, result model.SyncResult, syncErr error) error {
	var errorMessage *string
	if syncErr != nil {
		message := syncErr.Error()
		errorMessage = &message
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE sync_runs
		SET status = $2,
			fetched_count = $3,
			upserted_count = $4,
			deleted_count = $5,
			tables_count = $6,
			documents_fetched = $7,
			document_errors = $8,
			completed_at = NOW(),
			error_message = $9
		WHERE id = $1
	`, runID, status, result.Fetched, result.Upserted, result.Deleted, result.Tables,
		result.Documents, result.DocumentErrors, errorMessage)
	return err
}
