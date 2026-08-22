package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoSubaccountDirectoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-subaccount-export-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "directory-integration-" + time.Now().Format("20060102150405.000000000")
	fileName := prefix + ".xlsx"
	reportDate := time.Date(2098, 8, 5, 0, 0, 0, 0, time.UTC)
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id=$1", prefix+"-note")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_subaccounts WHERE subaccount=$1", prefix)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name=$1", fileName)
	}()

	snapshot := maituo.Snapshot{
		FileName: fileName, FileSHA256: prefix, ReportDate: reportDate,
		PresentSheets: []string{maituo.SheetNotes, maituo.SheetSubaccount},
		Notes:         []maituo.NoteDetail{{NoteID: prefix + "-note", NoteURL: "https://example.com", Category: "测评", Placement: "搜索", RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-note-hash"}}},
		Subaccounts:   []maituo.SubaccountOverview{{SPU: prefix, Subaccount: prefix, Placement: "搜索", RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-sub-hash"}}},
	}
	if _, err := postgres.ImportMaituoCustomerDaily(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	directories, err := postgres.MaituoSubaccountDirectories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range directories {
		if item.Subaccount == prefix && item.ReportCount == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("directory %q not found in %+v", prefix, directories)
	}
	reports, err := postgres.MaituoSubaccountReports(ctx, prefix)
	if err != nil || len(reports) != 1 || reports[0].ReportDate != "2098-08-05" {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	exported, err := postgres.MaituoSubaccountSnapshot(ctx, prefix, reportDate)
	if err != nil || len(exported.Notes) != 0 || len(exported.Subaccounts) != 1 {
		t.Fatalf("snapshot=%+v err=%v", exported, err)
	}
}
