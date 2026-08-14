package delivery

import "testing"

func TestNormalizeAndValidateDraftForcesPausedCampaign(t *testing.T) {
	spec := validTestDraftSpec()
	spec.Campaign.Enable = 1
	normalized := NormalizeSpec(spec)
	if normalized.Campaign.Enable != 0 {
		t.Fatalf("campaign enable = %d, want paused", normalized.Campaign.Enable)
	}
	errors, warnings := SplitIssues(ValidateDraftSpec(normalized))
	if len(errors) != 0 {
		t.Fatalf("valid spec errors = %+v", errors)
	}
	if len(warnings) == 0 {
		t.Fatal("expected creative diversity warning")
	}
}

func TestValidateDraftRejectsBudgetAndKeywordConflicts(t *testing.T) {
	spec := validTestDraftSpec()
	spec.Budget.DailyLimitFen = 9_999
	spec.Campaign.DayBudgetFen = 9_999
	spec.Units[0].NegativeKeywords = []NegativeKeyword{{Keyword: "辅酶q10"}}
	errors, _ := SplitIssues(ValidateDraftSpec(spec))
	codes := map[string]bool{}
	for _, issue := range errors {
		codes[issue.Code] = true
	}
	for _, code := range []string{"daily_budget_too_low", "campaign_budget_out_of_range", "keyword_polarity_conflict"} {
		if !codes[code] {
			t.Fatalf("missing validation code %q in %+v", code, errors)
		}
	}
}

func TestValidateDraftRejectsKeywordContractViolations(t *testing.T) {
	spec := validTestDraftSpec()
	spec.Units[0].Keywords[0].PhraseMatchType = 3
	spec.Units[0].Keywords[0].FeedBidFen = spec.Budget.MaxBidFen + 1
	spec.Units[0].NegativeKeywords = []NegativeKeyword{{Keyword: "批发", PhraseMatchType: 2}}
	errors, _ := SplitIssues(ValidateDraftSpec(spec))
	codes := map[string]bool{}
	for _, issue := range errors {
		codes[issue.Code] = true
	}
	for _, code := range []string{"keyword_match_type_invalid", "keyword_feed_bid_invalid", "negative_keyword_match_type_invalid"} {
		if !codes[code] {
			t.Fatalf("missing validation code %q in %+v", code, errors)
		}
	}
}

func TestValidateDraftRejectsTargetAndUnitContractViolations(t *testing.T) {
	spec := validTestDraftSpec()
	spec.Units[0].PromotionTarget = 2
	spec.Units[0].Target.Age = "23-27#invalid"
	spec.Units[0].Target.Gender = "female"
	spec.Units[0].Target.Device = "windows"
	spec.Units[0].Target.IntelligentExpansion = 2
	spec.Units[0].Target.KeywordTargetActions = []int{4}
	spec.Units[0].NoteIDs = []string{"bad-note"}
	errors, _ := SplitIssues(ValidateDraftSpec(spec))
	codes := map[string]bool{}
	for _, issue := range errors {
		codes[issue.Code] = true
	}
	for _, code := range []string{"promotion_target_mismatch", "target_age_invalid", "target_gender_invalid", "target_device_invalid", "target_expansion_invalid", "keyword_action_invalid", "unit_note_id_invalid"} {
		if !codes[code] {
			t.Fatalf("missing validation code %q in %+v", code, errors)
		}
	}
}

func validTestDraftSpec() DraftSpec {
	return DraftSpec{
		AdvertiserID: 1234,
		Objective:    "offsite_activation",
		Placement:    "search",
		Budget:       BudgetPolicy{DailyLimitFen: 30_000, TotalLimitFen: 210_000, MaxBidFen: 5_000},
		Notes:        []string{"0123456789abcdef01234567"},
		Campaign: CampaignSpec{
			LocalKey: "campaign", Name: "测试计划", MarketingTarget: 4, Placement: 2,
			PromotionTarget: 1, Enable: 0, TimeType: 0, TimePeriodType: 0,
			BiddingStrategy: 2, LimitDayBudget: 1, DayBudgetFen: 30_000, OptimizeTarget: 1,
		},
		Units: []UnitSpec{{
			LocalKey: "unit-1", Name: "测试单元", EventBidFen: 1_000,
			NoteIDs: []string{"0123456789abcdef01234567"}, PromotionTarget: 1, TargetType: 3,
			Target:   TargetSpec{Age: "23-27", Gender: "all", Device: "all", Cities: "all"},
			Keywords: []KeywordBid{{Keyword: "辅酶q10", BidFen: 800}},
			Creativities: []CreativitySpec{{
				LocalKey: "creative-1", Name: "测试创意", NoteID: "0123456789abcdef01234567",
			}},
		}},
		Experiment: ExperimentSpec{PrimaryMetric: "offsite_15d_cost"},
	}
}
