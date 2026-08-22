package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoXHSLinksKeepsMetricsAtNotePlacement(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-xhs-links-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	stamp := time.Now().UnixNano()
	prefix := fmt.Sprintf("xhs-link-%d", stamp)
	noteID := prefix + "-note"
	advertiserID := stamp
	campaignIDs := []int64{stamp + 1, stamp + 2}
	unitIDs := []int64{stamp + 11, stamp + 12}
	creativityIDs := []int64{stamp + 21, stamp + 22}
	reportDate := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id=$1", noteID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name=$1", prefix+".xlsx")
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM xhs_jg_advertisers WHERE advertiser_id=$1", advertiserID)
	}()

	searchCost := 24.69
	if _, err := postgres.ImportMaituoCustomerDaily(ctx, maituo.Snapshot{
		FileName: prefix + ".xlsx", FileSHA256: prefix + "-hash", ReportDate: reportDate,
		PresentSheets: []string{maituo.SheetNotes},
		Notes: []maituo.NoteDetail{{
			NoteID: noteID, NoteURL: "https://example.com", Category: "test", Placement: "搜索",
			Spend: 123.45, SearchUsers: 5, SearchCost: &searchCost, CPC: 1, CTRPct: 1,
			RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-note-hash"},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO xhs_jg_advertisers (advertiser_id, advertiser_name) VALUES ($1, $2)
	`, advertiserID, prefix+"-advertiser"); err != nil {
		t.Fatal(err)
	}
	for index := range campaignIDs {
		if _, err := postgres.pool.Exec(ctx, `
			INSERT INTO xhs_jg_campaigns (advertiser_id, campaign_id, campaign_name, placement)
			VALUES ($1, $2, $3, 2)
		`, advertiserID, campaignIDs[index], fmt.Sprintf("%s-campaign-%d", prefix, index+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := postgres.pool.Exec(ctx, `
			INSERT INTO xhs_jg_units (advertiser_id, unit_id, campaign_id, unit_name)
			VALUES ($1, $2, $3, $4)
		`, advertiserID, unitIDs[index], campaignIDs[index], fmt.Sprintf("%s-unit-%d", prefix, index+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := postgres.pool.Exec(ctx, `
			INSERT INTO xhs_jg_creativities (
				advertiser_id, creativity_id, campaign_id, unit_id, creativity_name, note_id
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, advertiserID, creativityIDs[index], campaignIDs[index], unitIDs[index], fmt.Sprintf("%s-creativity-%d", prefix, index+1), noteID); err != nil {
			t.Fatal(err)
		}
	}

	result, err := postgres.MaituoXHSLinks(ctx, maituo.XHSLinkQuery{Search: noteID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v", result)
	}
	item := result.Items[0]
	if item.NoteID != noteID || item.Placement != "搜索" {
		t.Fatalf("note-placement key = %+v", item)
	}
	if item.Spend != 123.45 || item.SearchUsers != 5 || item.SearchCost != searchCost {
		t.Fatalf("note-placement metrics = %+v", item)
	}
	if len(item.Matches) != 2 {
		t.Fatalf("real XHS campaigns = %+v", item.Matches)
	}
	for index, match := range item.Matches {
		if match.CampaignID != campaignIDs[index] || match.CampaignName == "" || len(match.Units) != 1 {
			t.Fatalf("match %d = %+v", index, match)
		}
	}

	delivery, err := postgres.MaituoTrafficDeliveryComparison(ctx, maituo.TrafficDeliveryComparisonQuery{NoteID: noteID, Placement: "搜索"})
	if err != nil {
		t.Fatal(err)
	}
	if len(delivery.Campaigns) != 2 {
		t.Fatalf("delivery campaigns = %+v", delivery.Campaigns)
	}
	for _, campaign := range delivery.Campaigns {
		if campaign.CampaignName == "" || len(campaign.Matches) != 1 {
			t.Fatalf("delivery campaign = %+v", campaign)
		}
	}
}
