package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoNoteCampaignAnalysisIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-analysis-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "analysis-" + time.Now().Format("20060102150405.000000000")
	noteID := prefix + "-note"
	campaign := prefix + "-campaign"
	dates := []time.Time{
		time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2098, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2098, 1, 3, 0, 0, 0, 0, time.UTC),
	}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id=$1", noteID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name LIKE $1", prefix+"%")
	}()

	searchCost := 5.0
	notes := [][]maituo.NoteDetail{
		{{NoteID: noteID, NoteURL: "https://example.com", Category: "test", Subaccount: "a", CampaignName: campaign, Placement: "搜索", Spend: 10, SearchUsers: 2, SearchCost: &searchCost, CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-1"}}},
		{{NoteID: noteID, NoteURL: "https://example.com", Category: "test", Subaccount: "a", CampaignName: campaign, Placement: "信息流", Spend: 100, SearchUsers: 10, SearchCost: &searchCost, CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-2"}}},
		{{NoteID: noteID, NoteURL: "https://example.com", Category: "test", Subaccount: "a", CampaignName: campaign, Placement: "搜索", Spend: 20, SearchUsers: 4, SearchCost: &searchCost, CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-3"}}},
	}
	for index, date := range dates {
		_, err := postgres.ImportMaituoCustomerDaily(ctx, maituo.Snapshot{
			FileName:   prefix + "-" + date.Format("2006-01-02") + ".xlsx",
			FileSHA256: prefix + "-hash-" + date.Format("2006-01-02"),
			ReportDate: date, PresentSheets: []string{maituo.SheetNotes}, Notes: notes[index],
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := postgres.MaituoNoteCampaignAnalysis(ctx, maituo.NoteCampaignAnalysisQuery{
		Window: "3d", Search: prefix, Sort: "daily_spend", Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ReportDates) != 3 || result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Items[0].Placement != "搜索" {
		t.Fatalf("daily spend sort first item = %+v", result.Items[0])
	}
	var searchItem *maituo.NoteCampaignAnalysisItem
	for index := range result.Items {
		if result.Items[index].Placement == "搜索" {
			searchItem = &result.Items[index]
			break
		}
	}
	if searchItem == nil || searchItem.ActiveDays != 2 || len(searchItem.Points) != 3 {
		t.Fatalf("search item = %+v", searchItem)
	}
	if searchItem.LatestSpend != 20 || searchItem.LatestSearchCost != 5 {
		t.Fatalf("latest metrics = %+v", searchItem)
	}
	if searchItem.Points[1].Spend != 0 || searchItem.Points[1].SearchCost != 0 || searchItem.Points[1].CumulativeSpend != 10 {
		t.Fatalf("flat point = %+v", searchItem.Points[1])
	}
	last := searchItem.Points[2]
	if last.CumulativeSpend != 30 || last.CumulativeUsers != 6 || last.SearchCost != 5 {
		t.Fatalf("last point = %+v", last)
	}
}

func TestMaituoTrafficComparisonSortsGapBeforeCost(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-traffic-comparison-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "traffic-comparison-" + time.Now().Format("20060102150405.000000000")
	dates := []time.Time{
		time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2099, 2, 2, 0, 0, 0, 0, time.UTC),
	}
	cost := func(value float64) *float64 { return &value }
	note := func(noteID, campaign string, searchCost float64, sourceRow int) maituo.NoteDetail {
		return maituo.NoteDetail{
			NoteID: noteID, NoteURL: "https://example.com", Category: "test", Subaccount: campaign,
			CampaignName: campaign, Placement: "搜索", Spend: searchCost * 2, SearchUsers: 2,
			SearchCost: cost(searchCost), CPC: 1, CTRPct: 1,
			RowMetadata: maituo.RowMetadata{SourceRow: sourceRow, ContentHash: prefix + noteID + campaign + fmt.Sprint(sourceRow)},
		}
	}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id LIKE $1", prefix+"%")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name LIKE $1", prefix+"%")
	}()

	for index, date := range dates {
		notes := []maituo.NoteDetail{
			note(prefix+"-large-gap", "campaign-a", 10, index*10+2),
			note(prefix+"-large-gap", "campaign-b", 30, index*10+3),
			note(prefix+"-small-gap", "campaign-a", 20, index*10+4),
			note(prefix+"-small-gap", "campaign-b", 25, index*10+5),
			note(prefix+"-single", "campaign-only", 100, index*10+6),
		}
		if _, err := postgres.ImportMaituoCustomerDaily(ctx, maituo.Snapshot{
			FileName: prefix + "-" + date.Format("2006-01-02") + ".xlsx", FileSHA256: prefix + "-hash-" + date.Format("2006-01-02"),
			ReportDate: date, PresentSheets: []string{maituo.SheetNotes}, Notes: notes,
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := postgres.MaituoTrafficComparison(ctx, maituo.TrafficComparisonQuery{
		Window: "3d", Search: prefix, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if result.Items[0].NoteID != prefix+"-large-gap" || result.Items[0].SearchCostGap != 20 {
		t.Fatalf("first item = %+v", result.Items[0])
	}
	if result.Items[0].ComparableCampaignCount != 2 {
		t.Fatalf("comparable campaigns = %+v", result.Items[0])
	}
	if result.Items[1].NoteID != prefix+"-small-gap" || result.Items[1].SearchCostGap != 5 {
		t.Fatalf("second item = %+v", result.Items[1])
	}
	if result.Items[2].NoteID != prefix+"-single" || result.Items[2].CampaignCount != 1 || result.Items[2].SearchCostGap != 0 {
		t.Fatalf("single item = %+v", result.Items[2])
	}
	if len(result.Items[0].Campaigns) != 2 || len(result.Items[0].Campaigns[0].Points) != 2 {
		t.Fatalf("campaign details = %+v", result.Items[0].Campaigns)
	}
}
