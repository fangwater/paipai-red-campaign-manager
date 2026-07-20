package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

func TestReplaceProviderContentSnapshotIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "provider-integration-test")
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	providerCode := "integration-" + time.Now().Format("20060102150405.000000000")
	_, err = postgres.pool.Exec(ctx, `
		INSERT INTO service_provider_content_tables (
			provider_code, provider_name, source_url, wiki_token, sheet_id, sheet_name, enabled
		) VALUES ($1, $2, 'https://example.feishu.cn/wiki/test', 'wiki-test', 'sheet-test', '达人笔记执行表', TRUE)
	`, providerCode, providerCode)
	if err != nil {
		t.Fatalf("insert provider index: %v", err)
	}
	defer func() {
		_, _ = postgres.pool.Exec(context.Background(),
			"DELETE FROM service_provider_note_executions WHERE provider_code = $1", providerCode)
		_, _ = postgres.pool.Exec(context.Background(),
			"DELETE FROM service_provider_content_tables WHERE provider_code = $1", providerCode)
	}()

	table := model.ProviderContentTable{
		ProviderCode: providerCode, ProviderName: providerCode, SpreadsheetToken: "spreadsheet-test",
		SheetID: "sheet-test", SheetName: "达人笔记执行表",
	}
	first, err := postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: table,
		Records: []model.ProviderNoteExecution{
			{RecordKey: "row:2", SourceRowNumber: 2, NoteID: "note-1", Progress: "待审核"},
			{RecordKey: "row:3", SourceRowNumber: 3, NoteID: "note-2", Progress: "已提交"},
		},
	})
	if err != nil {
		t.Fatalf("first ReplaceProviderContentSnapshot() error = %v", err)
	}
	if first.Upserted != 2 || first.Deleted != 0 {
		t.Fatalf("first result = %+v", first)
	}

	second, err := postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: table,
		Records: []model.ProviderNoteExecution{
			{RecordKey: "row:2", SourceRowNumber: 2, NoteID: "note-1", Progress: "已通过"},
		},
	})
	if err != nil {
		t.Fatalf("second ReplaceProviderContentSnapshot() error = %v", err)
	}
	if second.Upserted != 1 || second.Deleted != 1 {
		t.Fatalf("second result = %+v", second)
	}

	var active, deleted int
	var progress, status string
	var lastSyncedAt *time.Time
	err = postgres.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE deleted_at IS NULL),
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL),
			COALESCE(MAX(progress) FILTER (WHERE deleted_at IS NULL), '')
		FROM service_provider_note_executions
		WHERE provider_code = $1
	`, providerCode).Scan(&active, &deleted, &progress)
	if err != nil {
		t.Fatalf("query provider records: %v", err)
	}
	err = postgres.pool.QueryRow(ctx, `
		SELECT last_sync_status, last_synced_at
		FROM service_provider_content_tables
		WHERE provider_code = $1
	`, providerCode).Scan(&status, &lastSyncedAt)
	if err != nil {
		t.Fatalf("query provider index: %v", err)
	}
	if active != 1 || deleted != 1 || progress != "已通过" || status != "succeeded" || lastSyncedAt == nil {
		t.Fatalf("active=%d deleted=%d progress=%q status=%q last_synced_at=%v",
			active, deleted, progress, status, lastSyncedAt)
	}
}
