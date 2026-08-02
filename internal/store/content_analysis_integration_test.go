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
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SPU != "辅酶" || result.Agency != "全部" || result.Dimension != "audience" {
		t.Fatalf("result filters=%+v", result)
	}
	if result.Coverage.TotalNotes > 0 && (len(result.Types) == 0 || len(result.Dimensions) == 0 || len(result.Cells) == 0) {
		t.Fatalf("non-empty notes returned incomplete matrix: %+v", result.Coverage)
	}
}
