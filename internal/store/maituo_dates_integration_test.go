package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoNoteHistoryAndOlderReportProtectionIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-date-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "date-integration-" + time.Now().Format("20060102150405.000000000")
	fileName := prefix + ".xlsx"
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name=$1", fileName)
	}()

	note := maituo.NoteDetail{
		NoteID: prefix + "-note", NoteURL: "https://example.com/note", Category: "信息流",
		Placement: "搜索",
		Spend:     1, SearchUsers: 1, CPC: 1, CTRPct: 1,
		RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "same-note"},
	}
	newerDate := time.Date(2099, 2, 2, 0, 0, 0, 0, time.UTC)
	newer := maituo.Snapshot{FileName: fileName, FileSHA256: prefix + "-newer", ReportDate: newerDate, PresentSheets: append([]string(nil), maituo.WorkbookSheets...), Notes: []maituo.NoteDetail{note}}
	newerResult, err := postgres.ImportMaituoCustomerDaily(ctx, newer)
	if err != nil {
		t.Fatal(err)
	}
	if newerResult.TableCount != 5 || newerResult.Inserted != 1 {
		t.Fatalf("newer result: %+v", newerResult)
	}

	olderDate := time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)
	older := maituo.Snapshot{FileName: fileName, FileSHA256: prefix + "-older", ReportDate: olderDate, PresentSheets: append([]string(nil), maituo.WorkbookSheets...), Notes: []maituo.NoteDetail{note}}
	olderResult, err := postgres.ImportMaituoCustomerDaily(ctx, older)
	if err != nil {
		t.Fatal(err)
	}
	if olderResult.TableCount != 4 || olderResult.Inserted != 1 || len(olderResult.Tables) != 4 || olderResult.Tables[1].Key != "notes" {
		t.Fatalf("older result: %+v", olderResult)
	}

	var activeDates int
	if err := postgres.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT report_date) FROM maituo_customer_daily_notes WHERE note_id=$1 AND deleted_at IS NULL`, note.NoteID).Scan(&activeDates); err != nil {
		t.Fatal(err)
	}
	if activeDates != 2 {
		t.Fatalf("active report dates = %d, want 2", activeDates)
	}

	olderEmpty := older
	olderEmpty.FileSHA256 = prefix + "-older-empty"
	olderEmpty.Notes = nil
	deletedResult, err := postgres.ImportMaituoCustomerDaily(ctx, olderEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if deletedResult.Deleted != 1 {
		t.Fatalf("older empty result: %+v", deletedResult)
	}
	var remainingDate time.Time
	if err := postgres.pool.QueryRow(ctx, `SELECT report_date FROM maituo_customer_daily_notes WHERE note_id=$1 AND deleted_at IS NULL`, note.NoteID).Scan(&remainingDate); err != nil {
		t.Fatal(err)
	}
	if !remainingDate.Equal(newerDate) {
		t.Fatalf("remaining date = %s, want %s", remainingDate, newerDate)
	}

	duplicate, err := postgres.ImportMaituoCustomerDaily(ctx, olderEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.AlreadySaved || duplicate.RunID != deletedResult.RunID {
		t.Fatalf("duplicate result: %+v", duplicate)
	}

	saved, err := postgres.SavedMaituoImports(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) < 2 || saved[0].ReportDate != "2099-02-02" || saved[1].ReportDate != "2099-02-01" {
		t.Fatalf("saved reports: %+v", saved)
	}
	if saved[0].MergedRows != 1 || saved[1].MergedRows != 0 {
		t.Fatalf("saved merged rows: %+v", saved[:2])
	}
	detail, err := postgres.MaituoDailyNotes(ctx, newerDate)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ReportDate != "2099-02-02" || detail.Total != saved[0].MergedRows || len(detail.Items) != detail.Total || detail.Items[0].NoteID != note.NoteID {
		t.Fatalf("daily detail: %+v", detail)
	}
}
