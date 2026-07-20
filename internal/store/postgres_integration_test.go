package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

func TestReplaceSnapshotIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().Format("20060102150405.000000000")
	appToken := "integration-test-app-" + suffix
	tableID := "integration-test-table"
	documentKey := "docx:integration-test-" + suffix
	postgres, err := NewPostgres(ctx, databaseURL, appToken)
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	defer func() {
		cleanupCtx := context.Background()
		_, _ = postgres.pool.Exec(cleanupCtx, "DELETE FROM lark_record_documents WHERE app_token = $1", appToken)
		_, _ = postgres.pool.Exec(cleanupCtx, "DELETE FROM lark_bitable_records WHERE app_token = $1", appToken)
		_, _ = postgres.pool.Exec(cleanupCtx, "DELETE FROM lark_bitable_tables WHERE app_token = $1", appToken)
		_, _ = postgres.pool.Exec(cleanupCtx, "DELETE FROM sync_runs WHERE app_token = $1", appToken)
		_, _ = postgres.pool.Exec(cleanupCtx,
			"DELETE FROM lark_linked_documents WHERE provider = 'feishu' AND resource_key = $1",
			documentKey,
		)
	}()

	ref := model.DocumentRef{
		TableID: tableID, RecordID: "rec-1", FieldName: "稿件",
		Provider: "feishu", ResourceKey: documentKey, SourceURL: "https://example.feishu.cn/docx/test",
	}
	firstSnapshot := model.Snapshot{
		Tables: []model.Table{{
			ID: tableID, Name: "Test table", Revision: 1,
			Records: []model.Record{
				{ID: "rec-1", Fields: []byte(`{"name":"first"}`)},
				{ID: "rec-2", Fields: []byte(`{"name":"second"}`)},
			},
		}},
		DocumentRefs: []model.DocumentRef{ref},
	}
	first, err := postgres.ReplaceSnapshot(ctx, firstSnapshot, []model.Document{{
		Provider: "feishu", ResourceKey: documentKey, SourceURL: ref.SourceURL,
		DocumentType: "docx", Title: "Draft", Content: "Full manuscript",
		RevisionID: 1, Status: "succeeded",
	}})
	if err != nil {
		t.Fatalf("first ReplaceSnapshot() error = %v", err)
	}
	if first.Tables != 1 || first.Upserted != 2 || first.Deleted != 0 || first.Documents != 1 {
		t.Fatalf("first ReplaceSnapshot() result = %+v", first)
	}

	fresh, err := postgres.DocumentsToRefresh(ctx, []model.DocumentRef{ref}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("DocumentsToRefresh() error = %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("fresh document candidates = %d, want 0", len(fresh))
	}

	secondSnapshot := model.Snapshot{
		Tables: []model.Table{{
			ID: tableID, Name: "Test table", Revision: 2,
			Records: []model.Record{{ID: "rec-1", Fields: []byte(`{"name":"updated"}`)}},
		}},
		DocumentRefs: []model.DocumentRef{ref},
	}
	second, err := postgres.ReplaceSnapshot(ctx, secondSnapshot, nil)
	if err != nil {
		t.Fatalf("second ReplaceSnapshot() error = %v", err)
	}
	if second.Upserted != 1 || second.Deleted != 1 {
		t.Fatalf("second ReplaceSnapshot() result = %+v", second)
	}

	var activeCount int
	var name, content string
	err = postgres.pool.QueryRow(ctx, `
		SELECT COUNT(*), MIN(records.fields->>'name'), MIN(documents.content)
		FROM lark_bitable_records AS records
		JOIN lark_record_documents AS refs
		  ON refs.app_token = records.app_token
		 AND refs.table_id = records.table_id
		 AND refs.record_id = records.record_id
		JOIN lark_linked_documents AS documents
		  ON documents.provider = refs.provider
		 AND documents.resource_key = refs.resource_key
		WHERE records.app_token = $1
		  AND records.table_id = $2
		  AND records.deleted_at IS NULL
	`, appToken, tableID).Scan(&activeCount, &name, &content)
	if err != nil {
		t.Fatalf("query synchronized records and documents: %v", err)
	}
	if activeCount != 1 || name != "updated" || content != "Full manuscript" {
		t.Fatalf("active records = %d, name = %q, content = %q", activeCount, name, content)
	}
}
