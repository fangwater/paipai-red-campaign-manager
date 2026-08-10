package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/model"
)

func referenceMaterialTestID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:24]
}

func TestMaituoReferenceMaterialsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "reference-materials-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "materials-" + time.Now().Format("20060102150405.000000000")
	providerCode := prefix + "-provider"
	source1 := referenceMaterialTestID(prefix + "-source-1")
	source2 := referenceMaterialTestID(prefix + "-source-2")
	reference1 := referenceMaterialTestID(prefix + "-reference-1")
	reference2 := referenceMaterialTestID(prefix + "-reference-2")
	_, err = postgres.pool.Exec(ctx, `
		INSERT INTO service_provider_content_tables (
			provider_code, provider_name, source_url, wiki_token, sheet_id, sheet_name, enabled
		) VALUES ($1, $2, 'https://example.feishu.cn/wiki/test', 'wiki-test', 'sheet-test', '稿件测试表', TRUE)
	`, providerCode, prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM reference_material_contents WHERE reference_note_id=ANY($1::TEXT[])", []string{reference1, reference2})
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_note_executions WHERE provider_code=$1", providerCode)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_notes WHERE note_id=ANY($1::TEXT[])", []string{source1, source2})
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM service_provider_content_tables WHERE provider_code=$1", providerCode)
	}()

	_, err = postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: model.ProviderContentTable{
			ProviderCode: providerCode, ProviderName: prefix, SpreadsheetToken: "spreadsheet-test",
			SheetID: "sheet-test", SheetName: "稿件测试表",
		},
		Records: []model.ProviderNoteExecution{
			{RecordKey: "row:2", SourceRowNumber: 2, NoteID: source1},
			{RecordKey: "row:3", SourceRowNumber: 3, NoteID: source2},
		},
		Notes: []model.ProviderNote{
			{NoteID: source1, NoteContent: "第一篇", SourceTitle: "第一篇稿件",
				SourceURL:        "https://example.feishu.cn/wiki/source-1",
				ReferenceNoteIDs: []string{reference1, reference2, reference1, "invalid", source1}},
			{NoteID: source2, NoteContent: "第二篇", SourceTitle: "第二篇稿件",
				SourceURL:        "https://example.feishu.cn/wiki/source-2",
				ReferenceNoteIDs: []string{reference1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := postgres.MaituoReferenceMaterials(ctx, maituo.ReferenceMaterialsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || result.Stats.MaterialCount != 2 || result.Stats.SourceNoteCount != 2 ||
		result.Stats.ReferenceCount != 3 || result.Stats.ProviderCount != 1 || len(result.Items) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Items[0].ReferenceNoteID != reference1 || result.Items[0].UsageCount != 2 ||
		len(result.Items[0].SourceNoteIDs) != 2 || len(result.Items[0].Providers) != 1 ||
		result.Items[0].HasContent {
		t.Fatalf("first item = %+v", result.Items[0])
	}
	if len(result.Items[0].SourceManuscripts) != 2 {
		t.Fatalf("first item sources = %+v", result.Items[0].SourceManuscripts)
	}
	sources := make(map[string]maituo.ReferenceMaterialSource, len(result.Items[0].SourceManuscripts))
	for _, source := range result.Items[0].SourceManuscripts {
		sources[source.NoteID] = source
	}
	if sources[source1].Title != "第一篇稿件" ||
		sources[source1].URL != "https://example.feishu.cn/wiki/source-1" {
		t.Fatalf("source 1 = %+v", sources[source1])
	}
	if sources[source2].Title != "第二篇稿件" ||
		sources[source2].URL != "https://example.feishu.cn/wiki/source-2" {
		t.Fatalf("source 2 = %+v", sources[source2])
	}

	emptyContent, err := postgres.MaituoReferenceMaterialContent(ctx, reference1)
	if err != nil {
		t.Fatal(err)
	}
	if emptyContent.Found {
		t.Fatalf("empty content = %+v", emptyContent)
	}
	savedContent, found, err := postgres.SaveMaituoReferenceMaterialContent(ctx, maituo.ReferenceMaterialContentInput{
		ReferenceNoteID: reference1,
		NoteContent:     "人工维护的参考正文",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found || !savedContent.Found || savedContent.Source != "manual" ||
		savedContent.NoteContent != "人工维护的参考正文" {
		t.Fatalf("saved content = %+v, found = %t", savedContent, found)
	}
	storedContent, err := postgres.MaituoReferenceMaterialContent(ctx, reference1)
	if err != nil {
		t.Fatal(err)
	}
	if !storedContent.Found || storedContent.Source != "manual" ||
		storedContent.NoteContent != "人工维护的参考正文" {
		t.Fatalf("stored content = %+v", storedContent)
	}
	result, err = postgres.MaituoReferenceMaterials(ctx, maituo.ReferenceMaterialsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Items[0].HasContent || result.Items[0].ContentSource != "manual" {
		t.Fatalf("first item after save = %+v", result.Items[0])
	}
	_, found, err = postgres.SaveMaituoReferenceMaterialContent(ctx, maituo.ReferenceMaterialContentInput{
		ReferenceNoteID: referenceMaterialTestID(prefix + "-unknown"),
		NoteContent:     "不应保存",
	})
	if err != nil || found {
		t.Fatalf("save unknown reference: found = %t, err = %v", found, err)
	}

	filtered, err := postgres.MaituoReferenceMaterials(ctx, maituo.ReferenceMaterialsQuery{
		Search: reference2, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Stats.ReferenceCount != 1 || filtered.Items[0].ReferenceNoteID != reference2 {
		t.Fatalf("filtered = %+v", filtered)
	}

	filtered, err = postgres.MaituoReferenceMaterials(ctx, maituo.ReferenceMaterialsQuery{
		Search: "第二篇稿件", Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Stats.ReferenceCount != 1 || filtered.Items[0].ReferenceNoteID != reference1 {
		t.Fatalf("title filtered = %+v", filtered)
	}
}
