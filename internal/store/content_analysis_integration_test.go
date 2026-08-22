package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

func TestContentAnalysisQueryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "content-analysis-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()

	result, err := postgres.ContentAnalysis(ctx, model.ContentAnalysisQuery{
		SPU: "辅酶", Agency: "全部", Dimension: "audience",
		PublishedStartDate: "2000-01-01", PublishedEndDate: "2999-12-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SPU != "辅酶" || result.Agency != "全部" || result.Dimension != "audience" {
		t.Fatalf("result filters=%+v", result)
	}
	if result.PublishedStartDate != "2000-01-01" || result.PublishedEndDate != "2999-12-31" {
		t.Fatalf("result published date filters=%+v", result)
	}
	for _, cell := range result.Cells {
		for _, note := range cell.Notes {
			if note.PublishedDate < result.PublishedStartDate || note.PublishedDate > result.PublishedEndDate {
				t.Fatalf("note %s published_date=%q outside selected range", note.NoteID, note.PublishedDate)
			}
			for _, campaign := range append(append([]model.ContentAnalysisCampaign{}, note.SearchCampaigns...), note.FeedCampaigns...) {
				if campaign.AdvertiserID <= 0 || campaign.CampaignID <= 0 || campaign.Name == "" || campaign.SyncedAt == "" {
					t.Fatalf("note %s returned incomplete XHS campaign: %+v", note.NoteID, campaign)
				}
			}
		}
	}
	if result.Coverage.TotalNotes > 0 && (len(result.Types) == 0 || len(result.Dimensions) == 0 || len(result.Cells) == 0) {
		t.Fatalf("non-empty notes returned incomplete matrix: %+v", result.Coverage)
	}
}
