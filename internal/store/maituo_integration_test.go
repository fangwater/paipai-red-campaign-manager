package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestImportMaituoCustomerDailyIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().Format("20060102150405.000000000")
	prefix := "integration-" + suffix
	fileName := prefix + ".xlsx"
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_kpis WHERE metric LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_spus WHERE spu LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_subaccounts WHERE spu LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_trends WHERE report_date IN ('2099-01-01','2099-01-02')")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name=$1", fileName)
	}()
	one := 1.0
	firstSnapshot := maituo.Snapshot{
		FileName: fileName, FileSHA256: prefix + "-first", ReportDate: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), PresentSheets: append([]string(nil), maituo.WorkbookSheets...),
		KPIs:        []maituo.KPI{{Metric: prefix + "-metric", Value: 1, DataBasis: "test", RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "k1"}}},
		Notes:       []maituo.NoteDetail{{NoteID: prefix + "-note", NoteURL: "https://example.com", Category: "信息流", Subaccount: "account", CampaignName: "campaign", Placement: "搜索", Spend: 1, SearchUsers: 1, CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "n1"}}},
		SPUs:        []maituo.SPUOverview{{SPU: prefix + "-spu", AuctionSpend: 1, SearchUsers: 1, SearchCost: 1, SearchRatePct: 1, CPC: 1, CTRPct: 1, NoteCount: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "s1"}}},
		Subaccounts: []maituo.SubaccountOverview{{SPU: prefix + "-spu", Subaccount: "account-a", Placement: "搜索", SearchCost: &one, Spend: 1, SearchUsers: 1, CPC: &one, CTRPct: &one, NoteCount: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "a1"}}},
		Trends:      []maituo.SearchTrend{{Date: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), TotalSpend: &one, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "t1"}}},
	}
	first, err := postgres.ImportMaituoCustomerDaily(ctx, firstSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fetched != 5 || first.Inserted != 5 || first.Updated != 0 || first.Unchanged != 0 || first.Deleted != 0 {
		t.Fatalf("first result: %+v", first)
	}
	secondSnapshot := firstSnapshot
	secondSnapshot.FileSHA256 = prefix + "-second"
	secondSnapshot.KPIs = []maituo.KPI{{Metric: prefix + "-metric", Value: 2, DataBasis: "test", RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "k2"}}}
	secondSnapshot.Notes = nil
	secondSnapshot.Subaccounts = append(secondSnapshot.Subaccounts, maituo.SubaccountOverview{SPU: prefix + "-spu", Subaccount: "account-b", Placement: "信息流", Spend: 2, SearchUsers: 0, NoteCount: 1, RowMetadata: maituo.RowMetadata{SourceRow: 3, ContentHash: "a2"}})
	second, err := postgres.ImportMaituoCustomerDaily(ctx, secondSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if second.Fetched != 5 || second.Inserted != 1 || second.Updated != 1 || second.Unchanged != 3 || second.Deleted != 1 {
		t.Fatalf("second result: %+v", second)
	}
	thirdSnapshot := secondSnapshot
	thirdSnapshot.FileSHA256 = prefix + "-third"
	third, err := postgres.ImportMaituoCustomerDaily(ctx, thirdSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if third.Fetched != 5 || third.Inserted != 0 || third.Updated != 0 || third.Unchanged != 5 || third.Deleted != 0 {
		t.Fatalf("third result: %+v", third)
	}
	duplicate, err := postgres.ImportMaituoCustomerDaily(ctx, thirdSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.AlreadySaved || duplicate.RunID != third.RunID {
		t.Fatalf("duplicate result: %+v", duplicate)
	}

}
