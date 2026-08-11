package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"
)

func TestCoenzymeQ10DailyIncrementalSyncIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "coenzyme-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dates := []time.Time{
		time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2098, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2098, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2098, 1, 4, 0, 0, 0, 0, time.UTC),
	}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, `DELETE FROM coenzyme_q10_daily WHERE report_date BETWEEN '2098-01-01' AND '2098-01-04'`)
		_, _ = postgres.pool.Exec(cleanup, `DELETE FROM coenzyme_q10_sync_runs WHERE source_wiki_token='coenzyme-integration'`)
	}()

	firstRun, err := postgres.StartCoenzymeQ10Sync(ctx, "coenzyme-integration", "sheet", coenzyme.SheetName)
	if err != nil {
		t.Fatal(err)
	}
	first, err := postgres.ApplyCoenzymeQ10Daily(ctx, firstRun.RunID, coenzyme.Snapshot{
		SpreadsheetToken: "spreadsheet", SheetID: "sheet", SheetName: coenzyme.SheetName,
		Records: []coenzyme.DailyRecord{
			{ReportDate: dates[0], SourceRowNumber: 6, ContentHash: "hash-1"},
			{ReportDate: dates[1], SourceRowNumber: 7, ContentHash: "hash-2"},
			{ReportDate: dates[2], SourceRowNumber: 8, ContentHash: "hash-3"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.FinishCoenzymeQ10Sync(ctx, first, nil); err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 3 || first.Updated != 0 || first.Unchanged != 0 {
		t.Fatalf("first result = %+v", first)
	}

	secondRun, err := postgres.StartCoenzymeQ10Sync(ctx, "coenzyme-integration", "sheet", coenzyme.SheetName)
	if err != nil {
		t.Fatal(err)
	}
	second, err := postgres.ApplyCoenzymeQ10Daily(ctx, secondRun.RunID, coenzyme.Snapshot{
		SpreadsheetToken: "spreadsheet", SheetID: "sheet", SheetName: coenzyme.SheetName,
		Records: []coenzyme.DailyRecord{
			{ReportDate: dates[1], SourceRowNumber: 7, ContentHash: "hash-2"},
			{ReportDate: dates[2], SourceRowNumber: 8, ContentHash: "hash-3-updated"},
			{ReportDate: dates[3], SourceRowNumber: 9, ContentHash: "hash-4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.FinishCoenzymeQ10Sync(ctx, second, nil); err != nil {
		t.Fatal(err)
	}
	if second.Fetched != 3 || second.Inserted != 1 || second.Updated != 1 || second.Unchanged != 1 {
		t.Fatalf("second result = %+v", second)
	}
	var retained int
	if err := postgres.pool.QueryRow(ctx, `SELECT COUNT(*) FROM coenzyme_q10_daily WHERE report_date BETWEEN '2098-01-01' AND '2098-01-04'`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if retained != 4 {
		t.Fatalf("retained daily rows = %d, want 4", retained)
	}
}
