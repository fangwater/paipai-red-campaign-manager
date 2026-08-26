package delivery

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var noteIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

func ValidateDraftSpec(spec DraftSpec) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	errorAt := func(code, path, message string) {
		issues = append(issues, ValidationIssue{Code: code, Path: path, Message: message, Severity: "error"})
	}
	warnAt := func(code, path, message string) {
		issues = append(issues, ValidationIssue{Code: code, Path: path, Message: message, Severity: "warning"})
	}
	if spec.AdvertiserID <= 0 {
		errorAt("advertiser_required", "advertiser_id", "advertiser_id must be positive")
	}
	if spec.Objective == "" {
		errorAt("objective_required", "objective", "objective is required")
	}
	if spec.Placement == "" {
		errorAt("placement_required", "placement", "placement is required")
	}
	if spec.Budget.DailyLimitFen < 10_000 {
		errorAt("daily_budget_too_low", "budget.daily_limit_fen", "daily budget must be at least 10000 fen")
	}
	if spec.Budget.TotalLimitFen < spec.Budget.DailyLimitFen {
		errorAt("total_budget_too_low", "budget.total_limit_fen", "total budget must be no lower than daily budget")
	}
	if spec.Budget.AdvertiserDailyCapFen > 0 && spec.Budget.DailyLimitFen > spec.Budget.AdvertiserDailyCapFen {
		errorAt("advertiser_cap_exceeded", "budget.daily_limit_fen", "daily budget exceeds the advertiser cap")
	}
	if spec.Budget.MaxBidFen < 0 || spec.Budget.StopLossSpendFen < 0 || spec.Budget.StopLossConversionsMin < 0 {
		errorAt("negative_guardrail", "budget", "bid and stop-loss values cannot be negative")
	}

	campaign := spec.Campaign
	if utf8.RuneCountInString(campaign.Name) < 1 || utf8.RuneCountInString(campaign.Name) > 50 {
		errorAt("campaign_name_invalid", "campaign.name", "campaign name must contain 1 to 50 characters")
	}
	if !oneOf(campaign.MarketingTarget, 3, 4, 8, 9, 10, 13, 14, 15, 16, 17, 20, 21) {
		errorAt("marketing_target_invalid", "campaign.marketing_target", "marketing target is not supported by the current contract")
	}
	if !oneOf(campaign.Placement, 1, 2, 4, 7) {
		errorAt("campaign_placement_invalid", "campaign.placement", "placement must be 1, 2, 4, or 7")
	}
	if campaign.Enable != 0 {
		errorAt("campaign_must_start_paused", "campaign.enable", "new campaigns must be created paused")
	}
	if campaign.TimeType != 0 && campaign.TimeType != 1 {
		errorAt("time_type_invalid", "campaign.time_type", "time_type must be 0 or 1")
	}
	if campaign.TimeType == 1 {
		start, startErr := time.Parse(time.DateOnly, campaign.StartTime)
		end, endErr := time.Parse(time.DateOnly, campaign.ExpireTime)
		if startErr != nil || endErr != nil || end.Before(start) {
			errorAt("campaign_date_range_invalid", "campaign.start_time", "custom campaign dates must be valid YYYY-MM-DD values in ascending order")
		}
	}
	if campaign.LimitDayBudget != 1 {
		errorAt("unlimited_budget_forbidden", "campaign.limit_day_budget", "self-serve delivery requires a specified daily budget")
	}
	if campaign.DayBudgetFen != spec.Budget.DailyLimitFen {
		errorAt("budget_mismatch", "campaign.day_budget_fen", "campaign day budget must equal budget.daily_limit_fen")
	}
	if campaign.DayBudgetFen < 10_000 || campaign.DayBudgetFen >= 99_999_900 {
		errorAt("campaign_budget_out_of_range", "campaign.day_budget_fen", "campaign day budget is outside the platform range")
	}
	for path, rawURL := range map[string]string{
		"campaign.detect_url_link": campaign.DetectURLLink,
	} {
		validateHTTPSURL(path, rawURL, errorAt)
	}

	if len(spec.Units) == 0 {
		errorAt("unit_required", "units", "at least one unit is required")
	}
	if len(spec.Units) > 100 {
		errorAt("too_many_units", "units", "one draft cannot contain more than 100 units")
	}
	unitKeys := map[string]struct{}{}
	creativeKeys := map[string]struct{}{}
	allNotes := map[string]struct{}{}
	for unitIndex, unit := range spec.Units {
		base := fmt.Sprintf("units[%d]", unitIndex)
		if _, exists := unitKeys[unit.LocalKey]; exists {
			errorAt("duplicate_local_key", base+".local_key", "unit local_key must be unique")
		}
		unitKeys[unit.LocalKey] = struct{}{}
		if utf8.RuneCountInString(unit.Name) < 1 || utf8.RuneCountInString(unit.Name) > 50 {
			errorAt("unit_name_invalid", base+".name", "unit name must contain 1 to 50 characters")
		}
		if !oneOf(unit.TargetType, 0, 1, 2, 3) {
			errorAt("target_type_invalid", base+".target_type", "target_type must be 0, 1, 2, or 3")
		}
		if unit.PromotionTarget != campaign.PromotionTarget {
			errorAt("promotion_target_mismatch", base+".promotion_target", "unit promotion_target must match its campaign")
		}
		if unit.Target.Gender != "" && !oneOfString(unit.Target.Gender, "0", "1", "all") {
			errorAt("target_gender_invalid", base+".target.gender", "gender must be 0, 1, or all")
		}
		if unit.Target.Device != "" && !oneOfString(unit.Target.Device, "ios", "android", "all") {
			errorAt("target_device_invalid", base+".target.device", "device must be ios, android, or all")
		}
		if unit.Target.Age != "" && !validAgeTarget(unit.Target.Age) {
			errorAt("target_age_invalid", base+".target.age", "age must contain platform age segments separated by #, or all")
		}
		if !oneOf(unit.Target.IntelligentExpansion, 0, 1) {
			errorAt("target_expansion_invalid", base+".target.intelligent_expansion", "intelligent_expansion must be 0 or 1")
		}
		if unit.EventBidFen < 0 || spec.Budget.MaxBidFen > 0 && unit.EventBidFen > spec.Budget.MaxBidFen {
			errorAt("unit_bid_invalid", base+".event_bid_fen", "unit bid is negative or exceeds the draft guardrail")
		}
		if unit.TargetType == 3 && unit.Target.Age == "" {
			warnAt("age_target_all", base+".target.age", "advanced targeting has no explicit age selection")
		}
		if unit.Target.KeywordTargetPeriod != 0 && !oneOf(unit.Target.KeywordTargetPeriod, 3, 7, 15, 30) {
			errorAt("keyword_period_invalid", base+".target.keyword_target_period", "keyword target period must be 3, 7, 15, or 30 days")
		}
		for actionIndex, action := range unit.Target.KeywordTargetActions {
			if !oneOf(action, 1, 2, 3) {
				errorAt("keyword_action_invalid", fmt.Sprintf("%s.target.keyword_target_actions[%d]", base, actionIndex), "keyword target action must be 1, 2, or 3")
			}
		}
		if unit.KeywordTargetPeriod != 0 && !oneOf(unit.KeywordTargetPeriod, 3, 7, 15, 30) {
			errorAt("unit_keyword_period_invalid", base+".keyword_target_period", "keyword target period must be 3, 7, 15, or 30 days")
		}
		for actionIndex, action := range unit.KeywordTargetAction {
			if !oneOf(action, 1, 2, 3) {
				errorAt("unit_keyword_action_invalid", fmt.Sprintf("%s.keyword_target_actions[%d]", base, actionIndex), "keyword target action must be 1, 2, or 3")
			}
		}
		for noteIndex, noteID := range unit.NoteIDs {
			if !noteIDPattern.MatchString(noteID) {
				errorAt("unit_note_id_invalid", fmt.Sprintf("%s.note_ids[%d]", base, noteIndex), "note_id must be a 24-character hexadecimal ID")
			}
		}
		for spuIndex, spu := range unit.SPUNotes {
			if strings.TrimSpace(spu.SPUID) == "" {
				errorAt("spu_id_required", fmt.Sprintf("%s.spu_notes[%d].spu_id", base, spuIndex), "spu_id is required")
			}
			for noteIndex, noteID := range spu.NoteIDs {
				if !noteIDPattern.MatchString(noteID) {
					errorAt("spu_note_id_invalid", fmt.Sprintf("%s.spu_notes[%d].note_ids[%d]", base, spuIndex, noteIndex), "note_id must be a 24-character hexadecimal ID")
				}
			}
		}
		for itemIndex, item := range unit.ItemNotes {
			if strings.TrimSpace(item.ItemID) == "" {
				errorAt("item_id_required", fmt.Sprintf("%s.item_notes[%d].item_id", base, itemIndex), "item_id is required")
			}
			for noteIndex, noteID := range item.NoteIDs {
				if !noteIDPattern.MatchString(noteID) {
					errorAt("item_note_id_invalid", fmt.Sprintf("%s.item_notes[%d].note_ids[%d]", base, itemIndex, noteIndex), "note_id must be a 24-character hexadecimal ID")
				}
			}
		}
		validateHTTPSURL(base+".landing_page_url", unit.LandingPageURL, errorAt)
		validateHTTPSURL(base+".external_page_url", unit.ExternalPageURL, errorAt)
		keywordSeen := map[string]struct{}{}
		for keywordIndex, keyword := range unit.Keywords {
			path := fmt.Sprintf("%s.keywords[%d]", base, keywordIndex)
			key := strings.ToLower(keyword.Keyword)
			if key == "" || utf8.RuneCountInString(keyword.Keyword) > 50 {
				errorAt("keyword_invalid", path+".keyword", "keyword must contain 1 to 50 characters")
			}
			if _, exists := keywordSeen[key]; exists {
				errorAt("keyword_duplicate", path+".keyword", "keywords must be unique within a unit")
			}
			keywordSeen[key] = struct{}{}
			if keyword.BidFen <= 0 || spec.Budget.MaxBidFen > 0 && keyword.BidFen > spec.Budget.MaxBidFen {
				errorAt("keyword_bid_invalid", path+".bid_fen", "keyword bid must be positive and within the bid guardrail")
			}
			if keyword.FeedBidFen < 0 || spec.Budget.MaxBidFen > 0 && keyword.FeedBidFen > spec.Budget.MaxBidFen {
				errorAt("keyword_feed_bid_invalid", path+".feed_bid_fen", "keyword feed bid must be within the bid guardrail")
			}
			if !oneOf(keyword.PhraseMatchType, 0, 1, 2, 3) {
				errorAt("keyword_match_type_invalid", path+".phrase_match_type", "phrase_match_type must be 0, 1, 2, or 3")
			}
			if keyword.KeywordSource < 0 {
				errorAt("keyword_source_invalid", path+".keyword_source", "keyword_source cannot be negative")
			}
		}
		if len(unit.NegativeKeywords) > 400 {
			errorAt("too_many_negative_keywords", base+".negative_keywords", "a unit cannot contain more than 400 negative keywords")
		}
		negativeSeen := map[string]struct{}{}
		for keywordIndex, keyword := range unit.NegativeKeywords {
			path := fmt.Sprintf("%s.negative_keywords[%d]", base, keywordIndex)
			key := strings.ToLower(strings.TrimSpace(keyword.Keyword))
			if key == "" || utf8.RuneCountInString(keyword.Keyword) > 50 {
				errorAt("negative_keyword_invalid", path+".keyword", "negative keyword must contain 1 to 50 characters")
			}
			if _, exists := negativeSeen[key]; exists {
				errorAt("negative_keyword_duplicate", path+".keyword", "negative keywords must be unique within a unit")
			}
			negativeSeen[key] = struct{}{}
			if _, exists := keywordSeen[key]; exists {
				errorAt("keyword_polarity_conflict", path+".keyword", "the same word cannot be both positive and negative")
			}
			if !oneOf(keyword.PhraseMatchType, 0, 1) {
				errorAt("negative_keyword_match_type_invalid", path+".phrase_match_type", "negative keyword phrase_match_type must be 0 or 1")
			}
		}
		if len(unit.Creativities) == 0 {
			errorAt("creativity_required", base+".creativities", "each unit requires at least one creativity")
		}
		for creativeIndex, creative := range unit.Creativities {
			path := fmt.Sprintf("%s.creativities[%d]", base, creativeIndex)
			if _, exists := creativeKeys[creative.LocalKey]; exists {
				errorAt("duplicate_local_key", path+".local_key", "creativity local_key must be unique across the draft")
			}
			creativeKeys[creative.LocalKey] = struct{}{}
			if utf8.RuneCountInString(creative.Name) < 1 || utf8.RuneCountInString(creative.Name) > 50 {
				errorAt("creativity_name_invalid", path+".name", "creativity name must contain 1 to 50 characters")
			}
			if !noteIDPattern.MatchString(creative.NoteID) {
				errorAt("note_id_invalid", path+".note_id", "note_id must be a 24-character hexadecimal ID")
			}
			allNotes[strings.ToLower(creative.NoteID)] = struct{}{}
			for urlIndex, rawURL := range append(append([]string{}, creative.ClickURLs...), creative.ExpoURLs...) {
				validateHTTPSURL(fmt.Sprintf("%s.monitor_urls[%d]", path, urlIndex), rawURL, errorAt)
			}
			validateHTTPSURL(path+".jump_url", creative.JumpURL, errorAt)
			validateHTTPSURL(path+".fallback_jump_url", creative.FallbackJumpURL, errorAt)
			validateHTTPSURL(path+".app_component_icon", creative.AppComponentIcon, errorAt)
		}
	}
	if len(allNotes) < 2 {
		warnAt("limited_creative_diversity", "units", "the draft contains fewer than two distinct creative notes")
	}
	if spec.Experiment.PrimaryMetric == "" {
		errorAt("primary_metric_required", "experiment.primary_metric", "an experiment primary metric is required")
	}
	return issues
}

func validAgeTarget(value string) bool {
	if value == "all" {
		return true
	}
	seen := map[string]bool{}
	for _, segment := range strings.Split(value, "#") {
		if !oneOfString(segment, "18-22", "23-27", "28-32", "32-100") || seen[segment] {
			return false
		}
		seen[segment] = true
	}
	return len(seen) > 0
}

func SplitIssues(issues []ValidationIssue) (errors, warnings []ValidationIssue) {
	errors = make([]ValidationIssue, 0)
	warnings = make([]ValidationIssue, 0)
	for _, issue := range issues {
		if issue.Severity == "error" {
			errors = append(errors, issue)
		} else {
			warnings = append(warnings, issue)
		}
	}
	return errors, warnings
}

func oneOf(value int, allowed ...int) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateHTTPSURL(path, raw string, add func(string, string, string)) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		add("unsafe_url", path, "URL must be an absolute HTTPS URL without embedded credentials")
	}
}
