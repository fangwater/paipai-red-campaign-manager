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

func TestProviderManuscriptAssetIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "manuscript-asset-integration-test")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "manuscript-asset-" + time.Now().Format("20060102150405.000000000")
	noteID := prefix + "-note"
	referenceID := "69b1039d00000000080316ae"
	assetContent := []byte("integration-image")
	digest := sha256.Sum256(assetContent)
	assetID := hex.EncodeToString(digest[:])
	_, err = postgres.pool.Exec(ctx, `
		INSERT INTO service_provider_content_tables (
			provider_code, provider_name, source_url, wiki_token, sheet_id, sheet_name, enabled
		) VALUES ($1, $1, 'https://example.feishu.cn/wiki/test', 'wiki-test', 'sheet-test', '达人笔记执行表', TRUE)
	`, prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = postgres.pool.Exec(context.Background(), "DELETE FROM service_provider_notes WHERE note_id=$1", noteID)
		_, _ = postgres.pool.Exec(context.Background(), "DELETE FROM manuscript_assets WHERE asset_id=$1", assetID)
		_, _ = postgres.pool.Exec(context.Background(), "DELETE FROM service_provider_note_executions WHERE provider_code=$1", prefix)
		_, _ = postgres.pool.Exec(context.Background(), "DELETE FROM service_provider_content_tables WHERE provider_code=$1", prefix)
	}()

	_, err = postgres.ReplaceProviderContentSnapshot(ctx, model.ProviderContentSnapshot{
		Table: model.ProviderContentTable{
			ProviderCode: prefix, ProviderName: prefix, SpreadsheetToken: "sheet-token",
			SheetID: "sheet-test", SheetName: "达人笔记执行表",
		},
		Records: []model.ProviderNoteExecution{{
			RecordKey: "row:2", SourceRowNumber: 2, NoteID: noteID,
			NoteType: "科普", CoverType: "单图", CommercialIntensity: "软广",
			Audience: "职场人", UserScenario: "通勤", Progress: "已发布",
		}},
		Notes: []model.ProviderNote{{
			NoteID: noteID, NoteContent: "定稿正文", SourceResourceKey: "docx:final",
			ReferenceNoteIDs: []string{referenceID}, ExtractorVersion: model.ManuscriptExtractorVersion,
			ContentBlocks: []model.ManuscriptBlock{
				{Type: "paragraph", Text: "定稿正文"},
				{Type: "image", AssetID: assetID, Width: 100, Height: 200, Caption: "定稿配图"},
			},
			Assets: []model.ManuscriptAsset{{
				AssetID: assetID, ContentType: "image/png", ByteSize: int64(len(assetContent)),
				Width: 100, Height: 200, Content: assetContent,
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := postgres.MaituoNoteContent(ctx, noteID)
	if err != nil {
		t.Fatal(err)
	}
	if !content.Found || content.NoteContent != "定稿正文" || len(content.Blocks) != 2 ||
		content.Blocks[1].AssetID != assetID || !content.Tags.Complete ||
		len(content.Tags.UserScenario) != 1 || content.Tags.UserScenario[0] != "通勤" ||
		len(content.ReferenceNoteIDs) != 1 || content.ReferenceNoteIDs[0] != referenceID {
		t.Fatalf("note content = %+v", content)
	}
	asset, found, err := postgres.ManuscriptAsset(ctx, assetID)
	if err != nil || !found || string(asset.Content) != string(assetContent) || asset.ContentType != "image/png" {
		t.Fatalf("asset=%+v found=%v err=%v", asset, found, err)
	}

	toFetch, err := postgres.ProviderNotesToFetch(ctx, []model.DocumentRef{{RecordID: noteID, ResourceKey: "docx:final"}})
	if err != nil || len(toFetch) != 0 {
		t.Fatalf("current note refetch=%+v err=%v", toFetch, err)
	}
	toFetch, err = postgres.ProviderNotesToFetch(ctx, []model.DocumentRef{{RecordID: noteID, ResourceKey: "docx:replacement"}})
	if err != nil || len(toFetch) != 1 {
		t.Fatalf("replacement note refetch=%+v err=%v", toFetch, err)
	}
}
