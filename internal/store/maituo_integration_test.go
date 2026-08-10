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
	searchRate := 2.0
	cpc := 3.0
	ctr := 4.0
	feedCost := 8.0
	feedRate := 9.0
	feedCPC := 10.0
	feedCTR := 11.0
	firstSnapshot := maituo.Snapshot{
		FileName: fileName, FileSHA256: prefix + "-first", ReportDate: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), PresentSheets: append([]string(nil), maituo.WorkbookSheets...),
		KPIs:  []maituo.KPI{{Metric: prefix + "-metric", Value: 1, DataBasis: "test", RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "k1"}}},
		Notes: []maituo.NoteDetail{{NoteID: prefix + "-note", NoteURL: "https://example.com", Category: "信息流", Subaccount: prefix + "-account-a", CampaignName: "campaign", Placement: "搜索", Spend: 1, SearchUsers: 12, SearchCost: &one, CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "n1"}}},
		SPUs:  []maituo.SPUOverview{{SPU: prefix + "-spu", AuctionSpend: 1, SearchUsers: 1, SearchCost: 1, SearchRatePct: 1, CPC: 1, CTRPct: 1, NoteCount: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "s1"}}},
		Subaccounts: []maituo.SubaccountOverview{
			{
				SPU: prefix + "-spu", Subaccount: prefix + "-account-a", Placement: "搜索",
				SearchCost: &one, Spend: 5, SearchUsers: 6, SearchRatePct: &searchRate,
				CPC: &cpc, CTRPct: &ctr, NoteCount: 7,
				RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "a1"},
			},
			{
				SPU: prefix + "-spu", Subaccount: prefix + "-account-a", Placement: "信息流",
				EstimatedPostbackCost: &feedCost, Spend: 7, SearchUsers: 8, SearchRatePct: &feedRate,
				CPC: &feedCPC, CTRPct: &feedCTR, NoteCount: 9,
				RowMetadata: maituo.RowMetadata{SourceRow: 3, ContentHash: "a2"},
			},
		},
		Trends: []maituo.SearchTrend{{Date: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), TotalSpend: &one, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: "t1"}}},
	}
	first, err := postgres.ImportMaituoCustomerDaily(ctx, firstSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fetched != 6 || first.Inserted != 6 || first.Updated != 0 || first.Unchanged != 0 || first.Deleted != 0 {
		t.Fatalf("first result: %+v", first)
	}
	diagnosis, err := postgres.MaituoAccountPlanDiagnosis(ctx, prefix+"-spu")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnosis.Accounts) != 2 {
		t.Fatalf("diagnosis accounts: %+v", diagnosis.Accounts)
	}
	var searchAccount *maituo.AccountDiagnosis
	for index := range diagnosis.Accounts {
		if diagnosis.Accounts[index].Placement == "搜索" {
			searchAccount = &diagnosis.Accounts[index]
			break
		}
	}
	if searchAccount == nil || searchAccount.Spend != 5 || searchAccount.SearchUsers != 6 ||
		searchAccount.OriginalCost == nil || *searchAccount.OriginalCost != 1 ||
		searchAccount.CorrectionCoefficient != nil ||
		searchAccount.Cost == nil || *searchAccount.Cost != 1 ||
		searchAccount.SearchRatePct == nil || *searchAccount.SearchRatePct != 2 ||
		searchAccount.CPC == nil || *searchAccount.CPC != 3 ||
		searchAccount.CTRPct == nil || *searchAccount.CTRPct != 4 || searchAccount.NoteCount != 7 {
		t.Fatalf("search diagnosis: %+v", searchAccount)
	}
	if len(searchAccount.Points) != 7 {
		t.Fatalf("diagnosis points: %+v", searchAccount.Points)
	}
	latestPoint := searchAccount.Points[len(searchAccount.Points)-1]
	if latestPoint.Spend == nil || *latestPoint.Spend != 5 || latestPoint.SearchUsers == nil || *latestPoint.SearchUsers != 6 ||
		latestPoint.OriginalCost == nil || *latestPoint.OriginalCost != 1 ||
		latestPoint.CorrectionCoefficient != nil ||
		latestPoint.Cost == nil || *latestPoint.Cost != 1 ||
		latestPoint.NoteCount == nil || *latestPoint.NoteCount != 7 {
		t.Fatalf("diagnosis latest point: %+v", latestPoint)
	}
	if len(diagnosis.AccountOverviews) != 1 || len(diagnosis.AccountOverviews[0].Points) != maituoAccountOverviewDays {
		t.Fatalf("account overviews: %+v", diagnosis.AccountOverviews)
	}
	if len(searchAccount.Plans) != 1 ||
		searchAccount.Plans[0].OriginalCost == nil || *searchAccount.Plans[0].OriginalCost != 1 ||
		searchAccount.Plans[0].CorrectionCoefficient != nil ||
		searchAccount.Plans[0].Cost == nil || *searchAccount.Plans[0].Cost != 1 {
		t.Fatalf("search plans: %+v", searchAccount.Plans)
	}
	businessOverview, err := postgres.BusinessOverview(ctx, 7, prefix+"-spu")
	if err != nil {
		t.Fatal(err)
	}
	if len(businessOverview.OverlapPoints) != 7 {
		t.Fatalf("overview overlap points: %+v", businessOverview.OverlapPoints)
	}
	overlapPoint := businessOverview.OverlapPoints[len(businessOverview.OverlapPoints)-1]
	if overlapPoint.SPUSearchUsers == nil || *overlapPoint.SPUSearchUsers != 1 ||
		overlapPoint.SubaccountSearchUsers == nil || *overlapPoint.SubaccountSearchUsers != 14 ||
		overlapPoint.OverlapUsers == nil || *overlapPoint.OverlapUsers != 13 ||
		overlapPoint.OverlapCoefficient == nil || *overlapPoint.OverlapCoefficient != 14 ||
		overlapPoint.NoteSearchUsers == nil || *overlapPoint.NoteSearchUsers != 12 ||
		overlapPoint.NoteOverlapUsers == nil || *overlapPoint.NoteOverlapUsers != 11 ||
		overlapPoint.NoteOverlapCoefficient == nil || *overlapPoint.NoteOverlapCoefficient != 12 {
		t.Fatalf("overlap latest point: %+v", overlapPoint)
	}
	overview := diagnosis.AccountOverviews[0]
	overviewPoint := overview.Points[len(overview.Points)-1]
	if overview.CurrentTotalSpend != 12 || overviewPoint.TotalSpend == nil || *overviewPoint.TotalSpend != 12 ||
		overviewPoint.SearchSpend == nil || *overviewPoint.SearchSpend != 5 ||
		overviewPoint.SearchCost == nil || *overviewPoint.SearchCost != 1 ||
		overviewPoint.SearchCPC == nil || *overviewPoint.SearchCPC != 3 ||
		overviewPoint.SearchCTRPct == nil || *overviewPoint.SearchCTRPct != 4 ||
		overviewPoint.SearchRatePct == nil || *overviewPoint.SearchRatePct != 2 ||
		overviewPoint.FeedSpend == nil || *overviewPoint.FeedSpend != 7 ||
		overviewPoint.FeedCost == nil || *overviewPoint.FeedCost != 8 ||
		overviewPoint.FeedCPC == nil || *overviewPoint.FeedCPC != 10 ||
		overviewPoint.FeedCTRPct == nil || *overviewPoint.FeedCTRPct != 11 ||
		overviewPoint.FeedSearchRatePct == nil || *overviewPoint.FeedSearchRatePct != 9 {
		t.Fatalf("account overview point: %+v", overviewPoint)
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
	if second.Fetched != 6 || second.Inserted != 1 || second.Updated != 1 || second.Unchanged != 4 || second.Deleted != 1 {
		t.Fatalf("second result: %+v", second)
	}
	thirdSnapshot := secondSnapshot
	thirdSnapshot.FileSHA256 = prefix + "-third"
	third, err := postgres.ImportMaituoCustomerDaily(ctx, thirdSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if third.Fetched != 6 || third.Inserted != 0 || third.Updated != 0 || third.Unchanged != 6 || third.Deleted != 0 {
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
