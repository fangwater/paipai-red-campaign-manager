package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildQuickPlanTemplatesSeparatesGlobalPlacementsAndUsesConfigurationMode(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	samples := []QuickPlanTemplateSample{
		quickPlanTestSample(t, 1, 7, 10_000, 30, 3, "核心职场人", nil, now.Add(-time.Hour)),
		quickPlanTestSample(t, 1, 7, 10_000, 30, 3, "熬夜人群", nil, now),
		quickPlanTestSample(t, 1, 2, 20_000, 15, 3, "核心职场人", nil, now.Add(-2*time.Hour)),
		quickPlanTestSample(t, 2, 2, 20_000, 0, 0, "", []quickKeywordSnapshot{{Keyword: "辅酶q10", Bid: 150, PhraseMatchType: 3}}, now),
		quickPlanTestSample(t, 2, 2, 0, 0, 0, "", []quickKeywordSnapshot{{Keyword: "无预算样本", Bid: 150, PhraseMatchType: 3}}, now),
	}

	templates := BuildQuickPlanTemplates(samples, now)
	if !templates.Feed.Available || templates.Feed.SampleCount != 3 || templates.Feed.ModeSampleCount != 2 {
		t.Fatalf("feed template evidence = %+v", templates.Feed)
	}
	if templates.Feed.Summary.BiddingStrategy != 7 || templates.Feed.Summary.DayBudgetFen != 10_000 {
		t.Fatalf("feed mode summary = %+v", templates.Feed.Summary)
	}
	if templates.Feed.Confidence != roundScore(2.0/3.0) {
		t.Fatalf("feed confidence = %v", templates.Feed.Confidence)
	}
	if len(templates.Feed.Audiences) != 2 || templates.Feed.Audiences[0].Name != "核心职场人" || templates.Feed.Audiences[0].SampleCount != 2 {
		t.Fatalf("feed audiences = %+v", templates.Feed.Audiences)
	}
	if !templates.Search.Available || templates.Search.SampleCount != 1 || templates.Search.Summary.BiddingStrategy != 2 {
		t.Fatalf("search template = %+v", templates.Search)
	}
	if templates.Search.IgnoredSampleCount != 1 {
		t.Fatalf("search ignored samples = %d", templates.Search.IgnoredSampleCount)
	}
	if templates.Search.KeywordDefaults.BidFen != 150 || templates.Search.KeywordDefaults.PhraseMatchType != 3 {
		t.Fatalf("search keyword defaults = %+v", templates.Search.KeywordDefaults)
	}
	if templates.Feed.Summary.BiddingStrategy == templates.Search.Summary.BiddingStrategy {
		t.Fatal("feed and search configurations were unexpectedly mixed")
	}
	encoded, err := json.Marshal(templates)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"advertiser_id"`) || strings.Contains(string(encoded), `"spec"`) || strings.Contains(string(encoded), `"cities"`) {
		t.Fatalf("internal template configuration leaked into response: %s", encoded)
	}
}

func TestCreateQuickPlanDraftOnlyReplacesSelectedInputs(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 30, 0, 0, time.UTC)
	store := &serviceTestStore{quickPlanSamples: []QuickPlanTemplateSample{
		quickPlanTestSample(t, 2, 7, 10_000, 0, 0, "", []quickKeywordSnapshot{
			{Keyword: "样本词一", Bid: 150, PhraseMatchType: 3},
			{Keyword: "样本词二", Bid: 150, PhraseMatchType: 3},
		}, now),
	}}
	service, err := NewService(store, &serviceTestGateway{}, RuleSemanticAdvisor{}, HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	templates, err := service.QuickPlanTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(templates.Search.Audiences) != 1 {
		t.Fatalf("search audiences = %+v", templates.Search.Audiences)
	}
	noteID := "6a7298fc000000003301e8bd"
	draft, err := service.CreateQuickPlanDraft(context.Background(), CreateQuickPlanDraftInput{
		AdvertiserID: 11081105, Placement: "search", NoteID: noteID,
		NoteTitle: "心脏养护笔记", AudienceID: templates.Search.Audiences[0].ID,
		Keywords:       []string{" 心脏养护 ", "熬夜补救", "心脏养护"},
		IdempotencyKey: "quick-plan-test-key",
	}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Spec.Campaign.BiddingStrategy != 7 || draft.Spec.Campaign.DayBudgetFen != 10_000 || draft.Spec.Campaign.Enable != 0 {
		t.Fatalf("fixed campaign configuration changed: %+v", draft.Spec.Campaign)
	}
	if draft.Spec.AdvertiserID != 11081105 {
		t.Fatalf("draft advertiser = %d", draft.Spec.AdvertiserID)
	}
	if len(draft.Spec.Units) != 1 || draft.Spec.Units[0].TargetType != 0 {
		t.Fatalf("selected audience was not applied: %+v", draft.Spec.Units)
	}
	unit := draft.Spec.Units[0]
	if len(unit.NoteIDs) != 1 || unit.NoteIDs[0] != noteID || len(unit.Creativities) != 1 || unit.Creativities[0].NoteID != noteID {
		t.Fatalf("selected note was not applied: %+v", unit)
	}
	if len(unit.SPUNotes) != 1 || unit.SPUNotes[0].SPUID != "1566145" || len(unit.SPUNotes[0].NoteIDs) != 1 || unit.SPUNotes[0].NoteIDs[0] != noteID {
		t.Fatalf("SPU note binding = %+v", unit.SPUNotes)
	}
	if len(unit.Keywords) != 2 {
		t.Fatalf("normalized keywords = %+v", unit.Keywords)
	}
	for _, keyword := range unit.Keywords {
		if keyword.BidFen != 150 || keyword.PhraseMatchType != 3 {
			t.Fatalf("keyword defaults not retained: %+v", keyword)
		}
	}
	targetPayload := targetPayload(unit.Target)
	if targetPayload["haveBrandAIGroup"] != true || targetPayload["reverseConversionDuration"] != float64(30) {
		t.Fatalf("template target fields were not retained: %+v", targetPayload)
	}
	if targetPayload["unsafe_read_only"] != nil {
		t.Fatalf("unknown target field leaked into publish payload: %+v", targetPayload)
	}
	if errors, _ := SplitIssues(ValidateDraftSpec(draft.Spec)); len(errors) != 0 {
		t.Fatalf("quick plan draft validation errors = %+v", errors)
	}
	if store.audits != 1 {
		t.Fatalf("audit count = %d", store.audits)
	}
}

func TestCreateQuickPlanDraftAppliesEditableTemplateOverrides(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 45, 0, 0, time.UTC)
	store := &serviceTestStore{quickPlanSamples: []QuickPlanTemplateSample{
		quickPlanTestSample(t, 2, 7, 20_000, 0, 0, "", []quickKeywordSnapshot{{Keyword: "样本词", Bid: 180, PhraseMatchType: 3}}, now),
	}}
	service, err := NewService(store, &serviceTestGateway{}, RuleSemanticAdvisor{}, HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	templates, err := service.QuickPlanTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	marketingTarget, biddingStrategy := 13, 2
	dayBudget, eventBid, keywordBid := int64(75_000), int64(1_500), int64(275)
	pacingMode, timePeriodType, phraseMatchType := 2, 1, 0
	draft, err := service.CreateQuickPlanDraft(context.Background(), CreateQuickPlanDraftInput{
		AdvertiserID: 11081105, Placement: "search", NoteID: "6a7298fc000000003301e8bd",
		AudienceID: templates.Search.Audiences[0].ID, Keywords: []string{"辅酶Q10"},
		Overrides: &QuickPlanOverrides{
			MarketingTarget: &marketingTarget, BiddingStrategy: &biddingStrategy,
			DayBudgetFen: &dayBudget, EventBidFen: &eventBid, PacingMode: &pacingMode,
			TimePeriodType: &timePeriodType, KeywordBidFen: &keywordBid, PhraseMatchType: &phraseMatchType,
		},
		IdempotencyKey: "quick-plan-override-key",
	}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Spec.Campaign.MarketingTarget != 13 || draft.Spec.Campaign.BiddingStrategy != 2 || draft.Spec.Campaign.DayBudgetFen != dayBudget || draft.Spec.Campaign.PacingMode != 2 || draft.Spec.Campaign.TimePeriodType != 1 {
		t.Fatalf("campaign overrides = %+v", draft.Spec.Campaign)
	}
	if draft.Spec.Budget.DailyLimitFen != dayBudget || draft.Spec.Budget.TotalLimitFen != dayBudget*30 {
		t.Fatalf("budget overrides = %+v", draft.Spec.Budget)
	}
	unit := draft.Spec.Units[0]
	if unit.EventBidFen != eventBid || len(unit.Keywords) != 1 || unit.Keywords[0].BidFen != keywordBid || unit.Keywords[0].PhraseMatchType != phraseMatchType {
		t.Fatalf("unit overrides = %+v", unit)
	}

	invalidBudget := int64(9_999)
	_, err = service.CreateQuickPlanDraft(context.Background(), CreateQuickPlanDraftInput{
		AdvertiserID: 11081105, Placement: "search", NoteID: "6a7298fc000000003301e8bd",
		AudienceID: templates.Search.Audiences[0].ID, Keywords: []string{"辅酶Q10"},
		Overrides: &QuickPlanOverrides{DayBudgetFen: &invalidBudget}, IdempotencyKey: "quick-plan-invalid-override-key",
	}, Actor{ID: "operator-a", Role: "operator"})
	if err == nil || !strings.Contains(err.Error(), "day_budget_fen") {
		t.Fatalf("invalid override error = %v", err)
	}
}

func TestCreateQuickPlanDraftBindsSelectedNoteToInferredItem(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	sample := quickPlanTestSample(t, 2, 7, 100_000, 28_990, 0, "", []quickKeywordSnapshot{{Keyword: "样本词", Bid: 800, PhraseMatchType: 3}}, now)
	var campaign map[string]any
	var unit map[string]any
	if err := json.Unmarshal(sample.CampaignPayload, &campaign); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sample.UnitPayload, &unit); err != nil {
		t.Fatal(err)
	}
	campaign["marketing_target"] = 13
	delete(unit, "spu_note_info")
	unit["item_id"] = "1d866a6d5250401892b20cbb5603cbd4"
	unit["item_note_info"] = []map[string]any{{"item_id": unit["item_id"], "note_ids": unit["note_ids"]}}
	sample.CampaignPayload = quickPlanTestJSON(t, campaign)
	sample.UnitPayload = quickPlanTestJSON(t, unit)

	store := &serviceTestStore{quickPlanSamples: []QuickPlanTemplateSample{sample}}
	service, err := NewService(store, &serviceTestGateway{}, RuleSemanticAdvisor{}, HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	templates, err := service.QuickPlanTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	noteID := "6a7298fc000000003301e8bd"
	draft, err := service.CreateQuickPlanDraft(context.Background(), CreateQuickPlanDraftInput{
		AdvertiserID: 11169649, Placement: "search", NoteID: noteID,
		AudienceID: templates.Search.Audiences[0].ID, Keywords: []string{"辅酶Q10"},
		IdempotencyKey: "quick-item-plan-test-key",
	}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	created := draft.Spec.Units[0]
	if created.ItemID != "1d866a6d5250401892b20cbb5603cbd4" || len(created.ItemNotes) != 1 || created.ItemNotes[0].ItemID != created.ItemID || len(created.ItemNotes[0].NoteIDs) != 1 || created.ItemNotes[0].NoteIDs[0] != noteID {
		t.Fatalf("item note binding = item_id %q item_notes %+v", created.ItemID, created.ItemNotes)
	}
	if len(created.SPUNotes) != 0 {
		t.Fatalf("item template unexpectedly retained SPU bindings: %+v", created.SPUNotes)
	}
	payload := unitPayload(draft, created, 42)
	if payload["item_id"] != created.ItemID {
		t.Fatalf("unit item payload = %+v", payload)
	}
	if errors, _ := SplitIssues(ValidateDraftSpec(draft.Spec)); len(errors) != 0 {
		t.Fatalf("item quick plan validation errors = %+v", errors)
	}
}

func quickPlanTestSample(t *testing.T, placement, bidding int, dayBudget, eventBid int64, targetType int, crowdName string, keywords []quickKeywordSnapshot, syncedAt time.Time) QuickPlanTemplateSample {
	t.Helper()
	noteID := "6a7588c400000000320218b7"
	campaign := map[string]any{
		"marketing_target": 4, "placement": placement, "promotion_target": 1,
		"time_period_type": 0, "time_period": strings.Repeat("1", 168),
		"bidding_strategy": bidding, "limit_day_budget": 1, "campaign_day_budget": dayBudget,
		"optimize_target": 0, "constraint_type": 0, "constraint_value": eventBid,
		"smart_switch": 0, "pacing_mode": 1, "feed_flag": 0, "build_type": 0,
		"event_asset_id": 0, "asset_event": 0, "asset_event_id": 0, "page_category": 0,
		"search_flag": -1, "search_bid_ratio": 1, "deeplink_id": 0, "universal_link_id": 0,
		"detect_url_link": "",
	}
	crowdPackages := []map[string]any{}
	if crowdName != "" {
		crowdPackages = append(crowdPackages, map[string]any{
			"value": "2048_123", "name": crowdName, "type": "dmpCustom", "status": 0, "syncStatus": 0,
		})
	}
	unit := map[string]any{
		"event_bid": eventBid, "note_ids": []string{noteID}, "target_type": targetType,
		"target_config": map[string]any{
			"target_gender": "all", "target_age": "all", "target_device": "all",
			"target_device_price": "all", "target_city": "", "target_city_type": 0,
			"crowd_target":          map[string]any{"crowd_pkg": crowdPackages},
			"intelligent_expansion": 0, "haveBrandAIGroup": true,
			"reverseConversionDuration": 30, "unsafe_read_only": "drop",
		},
		"keyword_target_period": 0, "keyword_target_action": []int{1},
		"spu_note_info":     []map[string]any{{"spu_id": "1566145", "note_ids": []string{noteID}}},
		"keyword_with_bids": keywords, "keyword_gen_type": 0, "target_template_id": 0,
		"target_position": 0, "promotion_target_mode": 0, "search_bid_ratio": 1,
		"landing_page_type": 0, "note_rec_type": 0, "phrase_match_type_upgrade": 1,
		"aigc_note_black_rec": 0, "biz_unit_type": 0,
	}
	creative := map[string]any{
		"mask_prfer": true, "title_mask_prefer": false, "conversion_type": 0,
		"jump_url": "", "conversion_component_types": []int{0},
	}
	return QuickPlanTemplateSample{
		Placement: placement, CampaignPayload: quickPlanTestJSON(t, campaign),
		UnitPayload: quickPlanTestJSON(t, unit), CreativePayload: quickPlanTestJSON(t, creative),
		LatestSyncedAt: syncedAt,
	}
}

func quickPlanTestJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
