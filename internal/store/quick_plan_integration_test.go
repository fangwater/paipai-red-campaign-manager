package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/delivery"
)

func TestQuickPlanTemplateSamplesIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "quick-plan-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()

	var activeAdvertisers int
	if err := postgres.pool.QueryRow(ctx, `
			SELECT COUNT(DISTINCT campaign.advertiser_id)
			FROM xhs_jg_campaigns campaign
		JOIN xhs_jg_units unit
		  ON unit.advertiser_id=campaign.advertiser_id
		 AND unit.campaign_id=campaign.campaign_id
		JOIN xhs_jg_creativities creativity
		  ON creativity.advertiser_id=unit.advertiser_id
		 AND creativity.campaign_id=unit.campaign_id
		 AND creativity.unit_id=unit.unit_id
			WHERE campaign.placement IN (1,2)
			  AND campaign.deleted_at IS NULL AND campaign.campaign_enable=1 AND campaign.campaign_filter_state=1
			  AND COALESCE(NULLIF(campaign.raw_payload->>'not_available_status','')::integer,0)=0
			  AND unit.deleted_at IS NULL AND unit.unit_enable=1 AND unit.unit_filter_state=10 AND unit.not_available_status=0
			  AND creativity.deleted_at IS NULL AND creativity.creativity_enable=1
			  AND creativity.creativity_filter_state=8 AND creativity.audit_status IN (1,7)
		`).Scan(&activeAdvertisers); err != nil {
		t.Fatal(err)
	}
	if activeAdvertisers < 2 {
		t.Fatalf("production snapshot has only %d active quick-plan advertiser(s)", activeAdvertisers)
	}

	samples, err := postgres.QuickPlanTemplateSamples(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Fatal("global quick-plan query returned no active template samples")
	}
	rawPlacements := map[int]int{}
	for _, sample := range samples {
		rawPlacements[sample.Placement]++
		var campaign struct {
			Enable      int `json:"campaign_enable"`
			FilterState int `json:"campaign_filter_state"`
			Placement   int `json:"placement"`
		}
		var unit struct {
			Enable      int `json:"enable"`
			FilterState int `json:"unit_filter_state"`
			Unavailable int `json:"not_available_status"`
		}
		var creative struct {
			Enable      int `json:"creativity_enable"`
			FilterState int `json:"creativity_filter_state"`
			AuditStatus int `json:"audit_status"`
		}
		if err := json.Unmarshal(sample.CampaignPayload, &campaign); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(sample.UnitPayload, &unit); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(sample.CreativePayload, &creative); err != nil {
			t.Fatal(err)
		}
		if campaign.Enable != 1 || campaign.FilterState != 1 || campaign.Placement != sample.Placement {
			t.Fatalf("non-active campaign sample: %+v", campaign)
		}
		if unit.Enable != 1 || unit.FilterState != 10 || unit.Unavailable != 0 {
			t.Fatalf("non-active unit sample: %+v", unit)
		}
		if creative.Enable != 1 || creative.FilterState != 8 || creative.AuditStatus != 1 && creative.AuditStatus != 7 {
			t.Fatalf("non-active creative sample: %+v", creative)
		}
	}
	if rawPlacements[1] == 0 || rawPlacements[2] == 0 {
		t.Fatalf("global query did not return both placements: %+v", rawPlacements)
	}

	templates := delivery.BuildQuickPlanTemplates(samples, time.Now().UTC())
	for _, template := range []delivery.QuickPlanTemplate{templates.Feed, templates.Search} {
		if !template.Available {
			t.Fatalf("global %s template is unavailable: %+v", template.Placement, template)
		}
		if template.SampleCount <= 0 || template.ModeSampleCount <= 0 || template.Summary.DayBudgetFen < 10_000 || len(template.Audiences) == 0 {
			t.Fatalf("global %s template is unusable: %+v", template.Placement, template)
		}
		if template.Placement == "search" && template.KeywordDefaults.BidFen <= 0 {
			t.Fatalf("global search keyword defaults are unusable: %+v", template.KeywordDefaults)
		}
	}
}
