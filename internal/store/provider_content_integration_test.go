package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	noteID1 := providerCode + "-note-1"
	noteID2 := providerCode + "-note-2"
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
			"DELETE FROM service_provider_notes WHERE note_id IN ($1, $2)", noteID1, noteID2)
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
			{RecordKey: "row:2", SourceRowNumber: 2, NoteID: noteID1, Progress: "待审核"},
			{RecordKey: "row:3", SourceRowNumber: 3, NoteID: noteID2, Progress: "已提交"},
		},
		Notes: []model.ProviderNote{
			{NoteID: noteID1, NoteContent: "第一版正文"},
			{NoteID: noteID2, NoteContent: "第二篇正文"},
		},
	})
	if err != nil {
		t.Fatalf("first ReplaceProviderContentSnapshot() error = %v", err)
	}
	if first.Upserted != 2 || first.Notes != 2 || first.Deleted != 0 {
		t.Fatalf("first result = %+v", first)
	}

	second, err := postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: table,
		Records: []model.ProviderNoteExecution{
			{RecordKey: "row:2", SourceRowNumber: 2, NoteID: noteID1, Progress: "已通过"},
		},
		Notes: []model.ProviderNote{
			{NoteID: noteID1, NoteContent: "标题：存量正文标题\n正文：更新后的正文"},
		},
	})
	if err != nil {
		t.Fatalf("second ReplaceProviderContentSnapshot() error = %v", err)
	}
	if second.Upserted != 1 || second.Deleted != 1 {
		t.Fatalf("second result = %+v", second)
	}
	if err := postgres.UpdateProviderNoteSources(ctx, "youyiyouer", []model.DocumentRef{{
		RecordID:  noteID1,
		Label:     "内部稿件名",
		SourceURL: "https://example.feishu.cn/wiki/manuscript-1",
	}}); err != nil {
		t.Fatalf("UpdateProviderNoteSources() error = %v", err)
	}

	var active, deleted int
	var progress, status, noteContent, sourceTitle, sourceURL string
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
		SELECT note_content, source_title, COALESCE(source_url, '') FROM service_provider_notes WHERE note_id = $1
	`, noteID1).Scan(&noteContent, &sourceTitle, &sourceURL)
	if err != nil {
		t.Fatalf("query provider note: %v", err)
	}
	err = postgres.pool.QueryRow(ctx, `
		SELECT last_sync_status, last_synced_at
		FROM service_provider_content_tables
		WHERE provider_code = $1
	`, providerCode).Scan(&status, &lastSyncedAt)
	if err != nil {
		t.Fatalf("query provider index: %v", err)
	}
	if active != 1 || deleted != 1 || progress != "已通过" || noteContent != "标题：存量正文标题\n正文：更新后的正文" ||
		sourceTitle != "存量正文标题" || sourceURL != "https://example.feishu.cn/wiki/manuscript-1" ||
		status != "succeeded" || lastSyncedAt == nil {
		t.Fatalf("active=%d deleted=%d progress=%q note_content=%q status=%q last_synced_at=%v",
			active, deleted, progress, noteContent, status, lastSyncedAt)
	}
}

func TestProviderContentSyncPreservesManualMaterialAssetsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "provider-manual-assets-integration-test")
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	providerCode := "integration-manual-assets-" + time.Now().Format("20060102150405.000000000")
	noteID := providerCode + "-note"
	materialDigest := sha256.Sum256([]byte(providerCode + "-manual-material"))
	materialID := hex.EncodeToString(materialDigest[:])[:32]
	assetContent := []byte("manual material image")
	assetDigest := sha256.Sum256(assetContent)
	assetID := hex.EncodeToString(assetDigest[:])

	_, err = postgres.pool.Exec(ctx, `
		INSERT INTO service_provider_content_tables (
			provider_code, provider_name, source_url, wiki_token, sheet_id, sheet_name, enabled
		) VALUES ($1, $1, 'https://example.feishu.cn/wiki/test', 'wiki-test', 'sheet-test', '达人笔记执行表', TRUE)
	`, providerCode)
	if err != nil {
		t.Fatalf("insert provider index: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM manual_materials WHERE material_id=$1", materialID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_notes WHERE note_id=$1", noteID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM manuscript_assets WHERE asset_id=$1", assetID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_note_executions WHERE provider_code=$1", providerCode)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_content_tables WHERE provider_code=$1", providerCode)
	}()

	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO manuscript_assets (asset_id, content_type, byte_size, width, height, content)
		VALUES ($1, 'image/png', $2, 1, 1, $3)
	`, assetID, len(assetContent), assetContent); err != nil {
		t.Fatalf("insert manual material asset: %v", err)
	}
	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO manual_materials (material_id, title, body)
		VALUES ($1, '手工素材', '手工素材正文')
	`, materialID); err != nil {
		t.Fatalf("insert manual material: %v", err)
	}
	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO manual_material_assets (material_id, position, asset_id, width, height)
		VALUES ($1, 0, $2, 1, 1)
	`, materialID, assetID); err != nil {
		t.Fatalf("link manual material asset: %v", err)
	}

	if err := postgres.UpsertProviderNotes(ctx, []model.ProviderNote{{
		NoteID: noteID, NoteContent: "供应商稿件正文",
	}}); err != nil {
		t.Fatalf("UpsertProviderNotes() error = %v", err)
	}
	assertManuscriptAssetExists(t, ctx, postgres, assetID)

	_, err = postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: model.ProviderContentTable{
			ProviderCode: providerCode, ProviderName: providerCode, SpreadsheetToken: "spreadsheet-test",
			SheetID: "sheet-test", SheetName: "达人笔记执行表",
		},
		Records: []model.ProviderNoteExecution{{
			RecordKey: "row:2", SourceRowNumber: 2, NoteID: noteID, Progress: "已通过",
		}},
	})
	if err != nil {
		t.Fatalf("ReplaceProviderContentSnapshot() error = %v", err)
	}
	assertManuscriptAssetExists(t, ctx, postgres, assetID)
}

func assertManuscriptAssetExists(t *testing.T, ctx context.Context, postgres *Postgres, assetID string) {
	t.Helper()
	var exists bool
	if err := postgres.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM manuscript_assets WHERE asset_id=$1)
	`, assetID).Scan(&exists); err != nil {
		t.Fatalf("query manuscript asset: %v", err)
	}
	if !exists {
		t.Fatalf("manual material asset %s was deleted", assetID)
	}
}
