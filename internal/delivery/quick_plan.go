package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const maxQuickPlanAudiences = 100

type QuickPlanTemplates struct {
	Feed        QuickPlanTemplate `json:"feed"`
	Search      QuickPlanTemplate `json:"search"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type QuickPlanTemplate struct {
	Placement          string                   `json:"placement"`
	PlacementCode      int                      `json:"placement_code"`
	Available          bool                     `json:"available"`
	UnavailableReason  string                   `json:"unavailable_reason,omitempty"`
	SampleCount        int                      `json:"sample_count"`
	ModeSampleCount    int                      `json:"mode_sample_count"`
	Confidence         float64                  `json:"confidence"`
	IgnoredSampleCount int                      `json:"ignored_sample_count,omitempty"`
	LatestSyncedAt     *time.Time               `json:"latest_synced_at,omitempty"`
	Summary            QuickPlanTemplateSummary `json:"summary"`
	Audiences          []QuickPlanAudience      `json:"audiences"`
	KeywordDefaults    QuickPlanKeywordDefaults `json:"keyword_defaults"`
	spec               DraftSpec
	noteSPUs           map[string][]SPUNoteSpec
	noteItems          map[string][]ItemNoteSpec
}

type QuickPlanTemplateSummary struct {
	MarketingTarget int   `json:"marketing_target"`
	PromotionTarget int   `json:"promotion_target"`
	BiddingStrategy int   `json:"bidding_strategy"`
	OptimizeTarget  int   `json:"optimize_target"`
	DayBudgetFen    int64 `json:"day_budget_fen"`
	EventBidFen     int64 `json:"event_bid_fen"`
	PacingMode      int   `json:"pacing_mode"`
	TimePeriodType  int   `json:"time_period_type"`
	ConversionType  int   `json:"conversion_type"`
}

type QuickPlanKeywordDefaults struct {
	BidFen          int64 `json:"bid_fen"`
	FeedBidFen      int64 `json:"feed_bid_fen"`
	KeywordSource   int   `json:"keyword_source"`
	PhraseMatchType int   `json:"phrase_match_type"`
	SampleCount     int   `json:"sample_count"`
}

type QuickPlanAudience struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SampleCount int    `json:"sample_count"`
	TargetType  int    `json:"target_type"`
	target      TargetSpec
	position    int
	templateID  int64
}

type CreateQuickPlanDraftInput struct {
	AdvertiserID   int64               `json:"advertiser_id"`
	Placement      string              `json:"placement"`
	NoteID         string              `json:"note_id"`
	NoteTitle      string              `json:"note_title,omitempty"`
	AudienceID     string              `json:"audience_id"`
	Keywords       []string            `json:"keywords,omitempty"`
	Overrides      *QuickPlanOverrides `json:"overrides,omitempty"`
	IdempotencyKey string              `json:"idempotency_key"`
}

type QuickPlanOverrides struct {
	MarketingTarget *int   `json:"marketing_target,omitempty"`
	BiddingStrategy *int   `json:"bidding_strategy,omitempty"`
	DayBudgetFen    *int64 `json:"day_budget_fen,omitempty"`
	EventBidFen     *int64 `json:"event_bid_fen,omitempty"`
	PacingMode      *int   `json:"pacing_mode,omitempty"`
	TimePeriodType  *int   `json:"time_period_type,omitempty"`
	KeywordBidFen   *int64 `json:"keyword_bid_fen,omitempty"`
	PhraseMatchType *int   `json:"phrase_match_type,omitempty"`
}

type quickPlanDecodedSample struct {
	fixedSpec        DraftSpec
	targetType       int
	target           TargetSpec
	targetPosition   int
	targetTemplateID int64
	keywords         []KeywordBid
	noteIDs          []string
	spuNotes         []SPUNoteSpec
	itemNotes        []ItemNoteSpec
	latestSyncedAt   time.Time
}

type quickCampaignSnapshot struct {
	MarketingTarget   int             `json:"marketing_target"`
	Placement         int             `json:"placement"`
	PromotionTarget   int             `json:"promotion_target"`
	TimePeriodType    int             `json:"time_period_type"`
	TimePeriod        json.RawMessage `json:"time_period"`
	BiddingStrategy   int             `json:"bidding_strategy"`
	LimitDayBudget    int             `json:"limit_day_budget"`
	CampaignDayBudget int64           `json:"campaign_day_budget"`
	OptimizeTarget    int             `json:"optimize_target"`
	ConstraintType    int             `json:"constraint_type"`
	ConstraintValue   int64           `json:"constraint_value"`
	SmartSwitch       int             `json:"smart_switch"`
	PacingMode        int             `json:"pacing_mode"`
	FeedFlag          int             `json:"feed_flag"`
	BuildType         int             `json:"build_type"`
	EventAssetID      int64           `json:"event_asset_id"`
	AssetEvent        int             `json:"asset_event"`
	AssetEventID      int64           `json:"asset_event_id"`
	PageCategory      int             `json:"page_category"`
	SearchFlag        int             `json:"search_flag"`
	SearchBidRatio    float64         `json:"search_bid_ratio"`
	DeeplinkID        int64           `json:"deeplink_id"`
	UniversalLinkID   int64           `json:"universal_link_id"`
	DetectURLLink     string          `json:"detect_url_link"`
}

type quickUnitSnapshot struct {
	EventBid            int64                  `json:"event_bid"`
	NoteIDs             []string               `json:"note_ids"`
	TargetType          int                    `json:"target_type"`
	Target              json.RawMessage        `json:"target_config"`
	KeywordTargetPeriod int                    `json:"keyword_target_period"`
	KeywordTargetAction []int                  `json:"keyword_target_action"`
	BusinessTreeName    string                 `json:"business_tree_name"`
	SPUNotes            []SPUNoteSpec          `json:"spu_note_info"`
	ItemID              string                 `json:"item_id"`
	ItemNotes           []ItemNoteSpec         `json:"item_note_info"`
	Keywords            []quickKeywordSnapshot `json:"keyword_with_bids"`
	SubstitutedUserID   string                 `json:"substituted_user_id"`
	KeywordGenType      int                    `json:"keyword_gen_type"`
	TargetTemplateID    int64                  `json:"target_template_id"`
	TargetPosition      int                    `json:"target_position"`
	PromotionTargetMode int                    `json:"promotion_target_mode"`
	SearchBidRatio      float64                `json:"search_bid_ratio"`
	LandingPageType     int                    `json:"landing_page_type"`
	NoteRecType         int                    `json:"note_rec_type"`
	PhraseMatchUpgrade  int                    `json:"phrase_match_type_upgrade"`
	AIGCNoteBlackRec    int                    `json:"aigc_note_black_rec"`
	BizUnitType         int                    `json:"biz_unit_type"`
	TargetGoal          int                    `json:"target_goal"`
	CreationType        int                    `json:"creation_type"`
}

type quickKeywordSnapshot struct {
	Keyword         string `json:"keyword"`
	Bid             int64  `json:"bid"`
	FeedBid         int64  `json:"feed_bid"`
	KeywordSource   int    `json:"keyword_source"`
	PhraseMatchType int    `json:"phrase_match_type"`
}

type quickTargetSnapshot struct {
	Gender                         string                 `json:"target_gender"`
	Age                            string                 `json:"target_age"`
	Device                         string                 `json:"target_device"`
	DevicePrice                    string                 `json:"target_device_price"`
	Cities                         string                 `json:"target_city"`
	CityType                       int                    `json:"target_city_type"`
	AreaCode                       string                 `json:"targetAreaCode"`
	SearchCityIntent               string                 `json:"searchTargetCityIntent"`
	PremiumTargetType              int                    `json:"premium_target_type"`
	GeneralizationSwitch           int                    `json:"target_generalization_switch"`
	IndustryInterests              quickIndustryInterests `json:"industry_interest_target"`
	CrowdTarget                    quickCrowdTarget       `json:"crowd_target"`
	BehaviorKeywords               []string               `json:"keywords"`
	InterestKeywords               []string               `json:"interest_keywords"`
	KeywordTargetPeriod            int                    `json:"keyword_target_period"`
	KeywordTargetActions           []int                  `json:"keyword_target_action"`
	IntelligentExpansion           int                    `json:"intelligent_expansion"`
	ExcludeBloggerFans             bool                   `json:"haveReverseBloggerFanTarget"`
	ExcludeBloggerFansSnake        bool                   `json:"have_reverse_blogger_fan_target"`
	ExcludeBloggerPurchasers       bool                   `json:"haveReverseBloggerPurchasedTarget"`
	ExcludeBloggerPurchasersSnake  bool                   `json:"have_reverse_blogger_purchased_target"`
	IncludeBrandRecognition        bool                   `json:"haveBrandRecognitionGroup"`
	IncludeBrandRecognitionSnake   bool                   `json:"have_brand_recognition_group"`
	IncludeCategoryInterested      bool                   `json:"haveCategoryInterestGroup"`
	IncludeCategoryInterestedSnake bool                   `json:"have_category_interest_group"`
}

type quickIndustryInterests struct {
	Content  []CodeName `json:"content_interest"`
	Shopping []CodeName `json:"shopping_interest"`
}

type quickCrowdTarget struct {
	Packages []quickCrowdPackage `json:"crowd_pkg"`
}

type quickCrowdPackage struct {
	Value          string `json:"value"`
	Name           string `json:"name"`
	GroupID        string `json:"group_id"`
	Type           string `json:"type"`
	SyncState      int    `json:"syncStatus"`
	SyncStateSnake int    `json:"sync_status"`
	Status         int    `json:"status"`
}

type quickCreativeSnapshot struct {
	ExpoURLs                 []string `json:"expo_urls"`
	MaskPrefer               bool     `json:"mask_prfer"`
	TitleMaskPrefer          bool     `json:"title_mask_prefer"`
	ConversionType           int      `json:"conversion_type"`
	JumpURL                  string   `json:"jump_url"`
	LandingPageType          int      `json:"landing_page_type"`
	BarContent               string   `json:"bar_content"`
	ConversionComponentTypes []int    `json:"conversion_component_types"`
	Comment                  string   `json:"comment"`
}

func (service *Service) QuickPlanTemplates(ctx context.Context) (QuickPlanTemplates, error) {
	samples, err := service.store.QuickPlanTemplateSamples(ctx)
	if err != nil {
		return QuickPlanTemplates{}, err
	}
	return BuildQuickPlanTemplates(samples, service.now().UTC()), nil
}

func BuildQuickPlanTemplates(samples []QuickPlanTemplateSample, generatedAt time.Time) QuickPlanTemplates {
	return QuickPlanTemplates{
		Feed:        buildQuickPlanTemplate("feed", 1, samples),
		Search:      buildQuickPlanTemplate("search", 2, samples),
		GeneratedAt: generatedAt,
	}
}

func buildQuickPlanTemplate(placement string, placementCode int, samples []QuickPlanTemplateSample) QuickPlanTemplate {
	result := QuickPlanTemplate{
		Placement: placement, PlacementCode: placementCode,
		UnavailableReason: "全部广告主中没有符合条件的在投样本",
		Audiences:         []QuickPlanAudience{}, noteSPUs: map[string][]SPUNoteSpec{},
		noteItems: map[string][]ItemNoteSpec{},
	}
	type fixedBucket struct {
		signature string
		count     int
		latest    time.Time
		spec      DraftSpec
	}
	type audienceBucket struct {
		signature string
		count     int
		value     QuickPlanAudience
	}
	type keywordBucket struct {
		count int
		value QuickPlanKeywordDefaults
	}
	fixed := map[string]*fixedBucket{}
	audiences := map[string]*audienceBucket{}
	keywords := map[string]*keywordBucket{}
	validSamples := 0
	for _, sample := range samples {
		if sample.Placement != placementCode {
			continue
		}
		decoded, err := decodeQuickPlanSample(0, placement, sample)
		if err != nil {
			result.IgnoredSampleCount++
			continue
		}
		validSamples++
		encoded, err := json.Marshal(decoded.fixedSpec)
		if err != nil {
			result.IgnoredSampleCount++
			validSamples--
			continue
		}
		signature := string(encoded)
		bucket := fixed[signature]
		if bucket == nil {
			bucket = &fixedBucket{signature: signature, spec: decoded.fixedSpec}
			fixed[signature] = bucket
		}
		bucket.count++
		if decoded.latestSyncedAt.After(bucket.latest) {
			bucket.latest = decoded.latestSyncedAt
			bucket.spec = decoded.fixedSpec
		}
		if result.LatestSyncedAt == nil || decoded.latestSyncedAt.After(*result.LatestSyncedAt) {
			latest := decoded.latestSyncedAt
			result.LatestSyncedAt = &latest
		}

		audienceSignature := quickAudienceSignature(decoded)
		audience := audiences[audienceSignature]
		if audience == nil {
			audience = &audienceBucket{
				signature: audienceSignature,
				value: QuickPlanAudience{
					ID: quickSignatureID("aud", audienceSignature), Name: quickAudienceName(decoded.targetType, decoded.target),
					Description: quickAudienceDescription(decoded.target), TargetType: decoded.targetType,
					target: decoded.target, position: decoded.targetPosition, templateID: decoded.targetTemplateID,
				},
			}
			audiences[audienceSignature] = audience
		}
		audience.count++

		for _, keyword := range decoded.keywords {
			keywordSignature := fmt.Sprintf("%d:%d:%d:%d", keyword.BidFen, keyword.FeedBidFen, keyword.KeywordSource, keyword.PhraseMatchType)
			keywordMode := keywords[keywordSignature]
			if keywordMode == nil {
				keywordMode = &keywordBucket{value: QuickPlanKeywordDefaults{
					BidFen: keyword.BidFen, FeedBidFen: keyword.FeedBidFen,
					KeywordSource: keyword.KeywordSource, PhraseMatchType: keyword.PhraseMatchType,
				}}
				keywords[keywordSignature] = keywordMode
			}
			keywordMode.count++
		}
		for _, spu := range decoded.spuNotes {
			for _, noteID := range spu.NoteIDs {
				noteID = strings.ToLower(strings.TrimSpace(noteID))
				if noteID == "" || strings.TrimSpace(spu.SPUID) == "" {
					continue
				}
				if _, exists := result.noteSPUs[noteID]; !exists {
					result.noteSPUs[noteID] = []SPUNoteSpec{{SPUID: spu.SPUID}}
				}
			}
		}
		for _, item := range decoded.itemNotes {
			for _, noteID := range item.NoteIDs {
				noteID = strings.ToLower(strings.TrimSpace(noteID))
				if noteID == "" || strings.TrimSpace(item.ItemID) == "" {
					continue
				}
				if _, exists := result.noteItems[noteID]; !exists {
					result.noteItems[noteID] = []ItemNoteSpec{{ItemID: item.ItemID}}
				}
			}
		}
	}
	result.SampleCount = validSamples
	if validSamples == 0 {
		return result
	}

	fixedList := make([]*fixedBucket, 0, len(fixed))
	for _, bucket := range fixed {
		fixedList = append(fixedList, bucket)
	}
	sort.Slice(fixedList, func(left, right int) bool {
		if fixedList[left].count != fixedList[right].count {
			return fixedList[left].count > fixedList[right].count
		}
		if !fixedList[left].latest.Equal(fixedList[right].latest) {
			return fixedList[left].latest.After(fixedList[right].latest)
		}
		return fixedList[left].signature < fixedList[right].signature
	})
	selected := fixedList[0]
	result.Available = true
	result.UnavailableReason = ""
	result.ModeSampleCount = selected.count
	result.Confidence = roundScore(float64(selected.count) / float64(validSamples))
	result.spec = selected.spec
	result.Summary = quickTemplateSummary(selected.spec)

	audienceList := make([]*audienceBucket, 0, len(audiences))
	for _, bucket := range audiences {
		bucket.value.SampleCount = bucket.count
		audienceList = append(audienceList, bucket)
	}
	sort.Slice(audienceList, func(left, right int) bool {
		if audienceList[left].count != audienceList[right].count {
			return audienceList[left].count > audienceList[right].count
		}
		if audienceList[left].value.Name != audienceList[right].value.Name {
			return audienceList[left].value.Name < audienceList[right].value.Name
		}
		return audienceList[left].signature < audienceList[right].signature
	})
	for index, bucket := range audienceList {
		if index >= maxQuickPlanAudiences {
			break
		}
		result.Audiences = append(result.Audiences, bucket.value)
	}

	keywordList := make([]*keywordBucket, 0, len(keywords))
	for _, bucket := range keywords {
		bucket.value.SampleCount = bucket.count
		keywordList = append(keywordList, bucket)
	}
	sort.Slice(keywordList, func(left, right int) bool {
		if keywordList[left].count != keywordList[right].count {
			return keywordList[left].count > keywordList[right].count
		}
		leftValue, rightValue := keywordList[left].value, keywordList[right].value
		if leftValue.BidFen != rightValue.BidFen {
			return leftValue.BidFen < rightValue.BidFen
		}
		return leftValue.PhraseMatchType < rightValue.PhraseMatchType
	})
	if len(keywordList) > 0 {
		result.KeywordDefaults = keywordList[0].value
	} else {
		bid := result.Summary.EventBidFen
		if bid <= 0 {
			bid = 100
		}
		result.KeywordDefaults = QuickPlanKeywordDefaults{BidFen: bid, PhraseMatchType: 1}
	}
	result.spec.Budget.MaxBidFen = quickMaxInt64(result.spec.Budget.MaxBidFen, result.KeywordDefaults.BidFen, result.KeywordDefaults.FeedBidFen)
	return result
}

func decodeQuickPlanSample(advertiserID int64, placement string, sample QuickPlanTemplateSample) (quickPlanDecodedSample, error) {
	var campaign quickCampaignSnapshot
	if err := json.Unmarshal(sample.CampaignPayload, &campaign); err != nil {
		return quickPlanDecodedSample{}, fmt.Errorf("decode quick-plan campaign sample: %w", err)
	}
	var unit quickUnitSnapshot
	if err := json.Unmarshal(sample.UnitPayload, &unit); err != nil {
		return quickPlanDecodedSample{}, fmt.Errorf("decode quick-plan unit sample: %w", err)
	}
	var creative quickCreativeSnapshot
	if len(sample.CreativePayload) > 0 {
		if err := json.Unmarshal(sample.CreativePayload, &creative); err != nil {
			return quickPlanDecodedSample{}, fmt.Errorf("decode quick-plan creative sample: %w", err)
		}
	}
	var timePeriod any
	if len(campaign.TimePeriod) > 0 && string(campaign.TimePeriod) != "null" {
		if err := json.Unmarshal(campaign.TimePeriod, &timePeriod); err != nil {
			return quickPlanDecodedSample{}, fmt.Errorf("decode quick-plan schedule sample: %w", err)
		}
	}
	var targetSnapshot quickTargetSnapshot
	if len(unit.Target) > 0 {
		if err := json.Unmarshal(unit.Target, &targetSnapshot); err != nil {
			return quickPlanDecodedSample{}, fmt.Errorf("decode quick-plan target sample: %w", err)
		}
	}
	target := quickTargetSpec(targetSnapshot, unit.Target)
	keywords := make([]KeywordBid, 0, len(unit.Keywords))
	for _, keyword := range unit.Keywords {
		if strings.TrimSpace(keyword.Keyword) == "" || keyword.Bid <= 0 {
			continue
		}
		keywords = append(keywords, KeywordBid{
			Keyword: keyword.Keyword, BidFen: keyword.Bid, FeedBidFen: keyword.FeedBid,
			KeywordSource: keyword.KeywordSource, PhraseMatchType: keyword.PhraseMatchType,
		})
	}
	spuNotes := make([]SPUNoteSpec, 0, len(unit.SPUNotes))
	for _, spu := range unit.SPUNotes {
		if strings.TrimSpace(spu.SPUID) == "" {
			continue
		}
		spuNotes = append(spuNotes, SPUNoteSpec{SPUID: spu.SPUID})
	}
	itemNotes := make([]ItemNoteSpec, 0, len(unit.ItemNotes))
	for _, item := range unit.ItemNotes {
		if strings.TrimSpace(item.ItemID) == "" {
			continue
		}
		itemNotes = append(itemNotes, ItemNoteSpec{ItemID: item.ItemID})
	}
	dayBudget := campaign.CampaignDayBudget
	if campaign.LimitDayBudget != 1 || dayBudget < 10_000 || dayBudget >= 99_999_900 {
		return quickPlanDecodedSample{}, errors.New("quick-plan sample is outside the approved daily budget guardrail")
	}
	totalBudget := dayBudget * 30
	if totalBudget < dayBudget {
		totalBudget = dayBudget
	}
	eventBid := unit.EventBid
	fixedSpec := DraftSpec{
		AdvertiserID: advertiserID, Objective: quickObjective(campaign.MarketingTarget), Placement: placement,
		Budget: BudgetPolicy{DailyLimitFen: dayBudget, TotalLimitFen: totalBudget, MaxBidFen: eventBid},
		Campaign: CampaignSpec{
			LocalKey: "campaign", MarketingTarget: campaign.MarketingTarget, Placement: campaign.Placement,
			PromotionTarget: campaign.PromotionTarget, Enable: 0, TimeType: 0,
			TimePeriodType: campaign.TimePeriodType, TimePeriod: timePeriod,
			BiddingStrategy: campaign.BiddingStrategy, LimitDayBudget: campaign.LimitDayBudget,
			DayBudgetFen: dayBudget, OptimizeTarget: campaign.OptimizeTarget,
			ConstraintType: intPointer(campaign.ConstraintType), ConstraintValueFen: int64Pointer(campaign.ConstraintValue),
			SmartSwitch: intPointer(campaign.SmartSwitch), PacingMode: campaign.PacingMode,
			FeedFlag: intPointer(campaign.FeedFlag), BuildType: intPointer(campaign.BuildType),
			EventAssetID: campaign.EventAssetID, AssetEvent: campaign.AssetEvent,
			AssetEventID: campaign.AssetEventID, PageCategory: campaign.PageCategory,
			SearchFlag: intPointer(campaign.SearchFlag), SearchBidRatio: campaign.SearchBidRatio,
			DeeplinkID: campaign.DeeplinkID, UniversalLinkID: campaign.UniversalLinkID,
			DetectURLLink: campaign.DetectURLLink,
		},
		Units: []UnitSpec{{
			LocalKey: "unit-1", EventBidFen: eventBid, PromotionTarget: campaign.PromotionTarget,
			Target: TargetSpec{}, KeywordTargetPeriod: unit.KeywordTargetPeriod,
			KeywordTargetAction: append([]int(nil), unit.KeywordTargetAction...),
			BusinessTreeName:    unit.BusinessTreeName, SPUNotes: spuNotes,
			ItemID: unit.ItemID, ItemNotes: itemNotes,
			SubstitutedUserID: unit.SubstitutedUserID, KeywordGenType: unit.KeywordGenType,
			TargetPosition: intPointer(unit.TargetPosition), PromotionTargetMode: intPointer(unit.PromotionTargetMode),
			SearchBidRatio: floatPointer(unit.SearchBidRatio), LandingPageType: intPointer(unit.LandingPageType),
			NoteRecType: intPointer(unit.NoteRecType), PhraseMatchUpgrade: intPointer(unit.PhraseMatchUpgrade),
			AIGCNoteBlackRec: intPointer(unit.AIGCNoteBlackRec), BizUnitType: intPointer(unit.BizUnitType),
			TargetGoal: intPointer(unit.TargetGoal), CreationType: intPointer(unit.CreationType),
			Creativities: []CreativitySpec{{
				LocalKey: "creative-1-1", ExpoURLs: append([]string(nil), creative.ExpoURLs...),
				MaskPrefer: boolInt(creative.MaskPrefer), TitleMaskPrefer: boolInt(creative.TitleMaskPrefer),
				ConversionType: creative.ConversionType, JumpURL: creative.JumpURL,
				LandingPageType: creative.LandingPageType, BarContent: creative.BarContent,
				ConversionComponentTypes: append([]int(nil), creative.ConversionComponentTypes...),
				Comment:                  creative.Comment,
			}},
		}},
		Experiment: ExperimentSpec{
			PrimaryMetric: fmt.Sprintf("optimize_target_%d", campaign.OptimizeTarget),
			Variables:     []string{"note", "audience", "keyword"},
			HoldConstant:  []string{"campaign_configuration", "budget", "bid"},
		},
	}
	return quickPlanDecodedSample{
		fixedSpec: fixedSpec, targetType: unit.TargetType, target: target,
		targetPosition: unit.TargetPosition, targetTemplateID: unit.TargetTemplateID,
		keywords: keywords, noteIDs: append([]string(nil), unit.NoteIDs...),
		spuNotes:  append([]SPUNoteSpec(nil), unit.SPUNotes...),
		itemNotes: append([]ItemNoteSpec(nil), unit.ItemNotes...), latestSyncedAt: sample.LatestSyncedAt,
	}, nil
}

func quickTargetSpec(value quickTargetSnapshot, raw json.RawMessage) TargetSpec {
	crowds := make([]CrowdPackage, 0, len(value.CrowdTarget.Packages))
	for _, crowd := range value.CrowdTarget.Packages {
		syncState := crowd.SyncState
		if syncState == 0 {
			syncState = crowd.SyncStateSnake
		}
		crowds = append(crowds, CrowdPackage{
			Value: crowd.Value, Name: crowd.Name, GroupID: crowd.GroupID,
			Type: crowd.Type, SyncState: syncState, Status: crowd.Status,
		})
	}
	return TargetSpec{
		TemplateConfig: quickTemplateTargetConfig(raw),
		Gender:         value.Gender, Age: value.Age, Device: value.Device, DevicePrice: value.DevicePrice,
		Cities: value.Cities, CityType: intPointer(value.CityType), AreaCode: value.AreaCode,
		SearchCityIntent: value.SearchCityIntent, PremiumTargetType: intPointer(value.PremiumTargetType),
		GeneralizationSwitch: intPointer(value.GeneralizationSwitch),
		ContentInterests:     append([]CodeName(nil), value.IndustryInterests.Content...),
		ShoppingInterests:    append([]CodeName(nil), value.IndustryInterests.Shopping...),
		CrowdPackages:        crowds, BehaviorKeywords: append([]string(nil), value.BehaviorKeywords...),
		InterestKeywords:          append([]string(nil), value.InterestKeywords...),
		KeywordTargetPeriod:       value.KeywordTargetPeriod,
		KeywordTargetActions:      append([]int(nil), value.KeywordTargetActions...),
		IntelligentExpansion:      value.IntelligentExpansion,
		ExcludeBloggerFans:        value.ExcludeBloggerFans || value.ExcludeBloggerFansSnake,
		ExcludeBloggerPurchasers:  value.ExcludeBloggerPurchasers || value.ExcludeBloggerPurchasersSnake,
		IncludeBrandRecognition:   value.IncludeBrandRecognition || value.IncludeBrandRecognitionSnake,
		IncludeCategoryInterested: value.IncludeCategoryInterested || value.IncludeCategoryInterestedSnake,
	}
}

func quickTemplateTargetConfig(raw json.RawMessage) json.RawMessage {
	var decoded map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &decoded) != nil {
		return nil
	}
	filtered := make(map[string]any, len(decoded))
	for key, value := range decoded {
		if quickTargetConfigFields[key] {
			filtered[key] = value
		}
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return nil
	}
	return encoded
}

func (service *Service) CreateQuickPlanDraft(ctx context.Context, input CreateQuickPlanDraftInput, actor Actor) (Draft, error) {
	input.Placement = strings.TrimSpace(input.Placement)
	input.NoteID = strings.ToLower(strings.TrimSpace(input.NoteID))
	input.AudienceID = strings.TrimSpace(input.AudienceID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.AdvertiserID <= 0 {
		return Draft{}, errors.New("advertiser_id must be positive")
	}
	if input.Placement != "feed" && input.Placement != "search" {
		return Draft{}, errors.New("placement must be feed or search")
	}
	if !noteIDPattern.MatchString(input.NoteID) {
		return Draft{}, errors.New("note_id must be a 24-character hexadecimal ID")
	}
	if input.AudienceID == "" {
		return Draft{}, errors.New("audience_id is required")
	}
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 160 {
		return Draft{}, errors.New("idempotency_key must contain 8 to 160 characters")
	}
	keywords, err := normalizeQuickKeywords(input.Keywords)
	if err != nil {
		return Draft{}, err
	}
	if input.Placement == "search" && len(keywords) == 0 {
		return Draft{}, errors.New("search quick plans require at least one keyword")
	}
	templates, err := service.QuickPlanTemplates(ctx)
	if err != nil {
		return Draft{}, err
	}
	template := templates.Feed
	if input.Placement == "search" {
		template = templates.Search
	}
	if !template.Available || len(template.spec.Units) == 0 {
		return Draft{}, fmt.Errorf("%w: %s", ErrValidation, template.UnavailableReason)
	}
	var selectedAudience *QuickPlanAudience
	for index := range template.Audiences {
		if template.Audiences[index].ID == input.AudienceID {
			selectedAudience = &template.Audiences[index]
			break
		}
	}
	if selectedAudience == nil {
		return Draft{}, errors.New("audience_id is not part of the current active template")
	}

	selectedSPUNotes := quickPlanSPUNotes(template, input.NoteID)
	selectedItemNotes := quickPlanItemNotes(template, input.NoteID)
	spec := template.spec
	spec.AdvertiserID = input.AdvertiserID
	keywordDefaults := template.KeywordDefaults
	if input.Overrides != nil {
		if err := applyQuickPlanOverrides(&spec, &keywordDefaults, *input.Overrides); err != nil {
			return Draft{}, err
		}
	}
	spec.Notes = []string{input.NoteID}
	unit := &spec.Units[0]
	unit.NoteIDs = []string{input.NoteID}
	unit.TargetType = selectedAudience.TargetType
	unit.Target = cloneQuickTarget(selectedAudience.target)
	unit.TargetPosition = intPointer(selectedAudience.position)
	unit.TargetTemplateID = int64Pointer(selectedAudience.templateID)
	unit.SPUNotes = nil
	unit.ItemNotes = nil
	unit.ItemID = ""
	if spec.Campaign.MarketingTarget == 13 {
		unit.ItemNotes = selectedItemNotes
		if len(unit.ItemNotes) > 0 {
			unit.ItemID = unit.ItemNotes[0].ItemID
		}
	} else {
		unit.SPUNotes = selectedSPUNotes
	}
	unit.Keywords = nil
	if input.Placement == "search" {
		unit.Keywords = make([]KeywordBid, 0, len(keywords))
		for _, keyword := range keywords {
			unit.Keywords = append(unit.Keywords, KeywordBid{
				Keyword: keyword, BidFen: keywordDefaults.BidFen,
				FeedBidFen:      keywordDefaults.FeedBidFen,
				KeywordSource:   keywordDefaults.KeywordSource,
				PhraseMatchType: keywordDefaults.PhraseMatchType,
			})
		}
	} else {
		unit.Target.BehaviorKeywords = keywords
	}
	if len(unit.Creativities) == 0 {
		unit.Creativities = []CreativitySpec{{LocalKey: "creative-1-1"}}
	} else {
		unit.Creativities = unit.Creativities[:1]
	}
	unit.Creativities[0].NoteID = input.NoteID
	name := quickPlanName(input.Placement, input.NoteTitle, input.NoteID, input.IdempotencyKey, service.now())
	spec.Campaign.Name = name
	unit.Name = limitRunes(name+"-单元", 50)
	unit.Creativities[0].Name = limitRunes(name+"-创意", 50)
	spec.Budget.MaxBidFen = quickMaxInt64(spec.Budget.MaxBidFen, unit.EventBidFen, keywordDefaults.BidFen, keywordDefaults.FeedBidFen)
	return service.CreateDraft(ctx, CreateDraftInput{
		Spec: spec, IdempotencyKey: input.IdempotencyKey,
		ChangeReason: "快速新建计划：按当前在投模板生成",
	}, actor)
}

func applyQuickPlanOverrides(spec *DraftSpec, keywordDefaults *QuickPlanKeywordDefaults, overrides QuickPlanOverrides) error {
	if len(spec.Units) == 0 {
		return errors.New("quick-plan template has no unit to override")
	}
	campaign := &spec.Campaign
	unit := &spec.Units[0]
	if overrides.MarketingTarget != nil {
		if !oneOf(*overrides.MarketingTarget, 4, 13) {
			return errors.New("overrides.marketing_target must be 4 or 13")
		}
		campaign.MarketingTarget = *overrides.MarketingTarget
		spec.Objective = quickObjective(*overrides.MarketingTarget)
	}
	if overrides.BiddingStrategy != nil {
		if !oneOf(*overrides.BiddingStrategy, 2, 3, 7) {
			return errors.New("overrides.bidding_strategy must be 2, 3, or 7")
		}
		campaign.BiddingStrategy = *overrides.BiddingStrategy
	}
	if overrides.DayBudgetFen != nil {
		if *overrides.DayBudgetFen < 10_000 || *overrides.DayBudgetFen >= 99_999_900 {
			return errors.New("overrides.day_budget_fen is outside the platform range")
		}
		campaign.DayBudgetFen = *overrides.DayBudgetFen
		spec.Budget.DailyLimitFen = *overrides.DayBudgetFen
		spec.Budget.TotalLimitFen = *overrides.DayBudgetFen * 30
	}
	if overrides.EventBidFen != nil {
		if *overrides.EventBidFen < 0 || *overrides.EventBidFen >= 99_999_900 {
			return errors.New("overrides.event_bid_fen is outside the platform range")
		}
		unit.EventBidFen = *overrides.EventBidFen
		if campaign.ConstraintValueFen != nil {
			campaign.ConstraintValueFen = int64Pointer(*overrides.EventBidFen)
		}
	}
	if overrides.PacingMode != nil {
		if !oneOf(*overrides.PacingMode, 0, 1, 2) {
			return errors.New("overrides.pacing_mode must be 0, 1, or 2")
		}
		campaign.PacingMode = *overrides.PacingMode
	}
	if overrides.TimePeriodType != nil {
		if !oneOf(*overrides.TimePeriodType, 0, 1) {
			return errors.New("overrides.time_period_type must be 0 or 1")
		}
		campaign.TimePeriodType = *overrides.TimePeriodType
	}
	if overrides.KeywordBidFen != nil {
		if *overrides.KeywordBidFen <= 0 || *overrides.KeywordBidFen >= 99_999_900 {
			return errors.New("overrides.keyword_bid_fen is outside the platform range")
		}
		keywordDefaults.BidFen = *overrides.KeywordBidFen
	}
	if overrides.PhraseMatchType != nil {
		if !oneOf(*overrides.PhraseMatchType, 0, 1, 2, 3) {
			return errors.New("overrides.phrase_match_type must be 0, 1, 2, or 3")
		}
		keywordDefaults.PhraseMatchType = *overrides.PhraseMatchType
	}
	return nil
}

func quickAudienceSignature(value quickPlanDecodedSample) string {
	encoded, _ := json.Marshal(struct {
		TargetType int        `json:"target_type"`
		Target     TargetSpec `json:"target"`
		Position   int        `json:"target_position"`
		TemplateID int64      `json:"target_template_id"`
	}{value.targetType, value.target, value.targetPosition, value.targetTemplateID})
	return string(encoded)
}

func quickSignatureID(prefix, signature string) string {
	digest := sha256.Sum256([]byte(signature))
	return prefix + "_" + hex.EncodeToString(digest[:6])
}

func quickAudienceName(targetType int, target TargetSpec) string {
	names := make([]string, 0, len(target.CrowdPackages))
	for _, crowd := range target.CrowdPackages {
		if name := strings.TrimSpace(crowd.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return limitRunes(strings.Join(names, " + "), 40)
	}
	switch targetType {
	case 0:
		return "默认定向"
	case 1:
		return "通投人群"
	case 2:
		return "智能定向"
	default:
		if target.Age != "" && target.Age != "all" {
			return limitRunes("高级定向 · "+strings.ReplaceAll(target.Age, "#", " / "), 40)
		}
		return "高级定向"
	}
}

func quickAudienceDescription(target TargetSpec) string {
	age := target.Age
	if age == "" || age == "all" {
		age = "年龄不限"
	} else {
		age = strings.ReplaceAll(age, "#", " / ")
	}
	gender := map[string]string{"0": "男性", "1": "女性", "all": "性别不限"}[target.Gender]
	if gender == "" {
		gender = "性别不限"
	}
	region := "全国"
	if target.CityType != nil && *target.CityType != 0 && strings.TrimSpace(target.Cities) != "" {
		region = fmt.Sprintf("%d 个地域", len(strings.Split(target.Cities, "#")))
	}
	return limitRunes(strings.Join([]string{age, gender, region}, " · "), 80)
}

func quickTemplateSummary(spec DraftSpec) QuickPlanTemplateSummary {
	result := QuickPlanTemplateSummary{
		MarketingTarget: spec.Campaign.MarketingTarget, PromotionTarget: spec.Campaign.PromotionTarget,
		BiddingStrategy: spec.Campaign.BiddingStrategy, OptimizeTarget: spec.Campaign.OptimizeTarget,
		DayBudgetFen: spec.Campaign.DayBudgetFen, PacingMode: spec.Campaign.PacingMode,
		TimePeriodType: spec.Campaign.TimePeriodType,
	}
	if len(spec.Units) > 0 {
		result.EventBidFen = spec.Units[0].EventBidFen
		if len(spec.Units[0].Creativities) > 0 {
			result.ConversionType = spec.Units[0].Creativities[0].ConversionType
		}
	}
	return result
}

func quickObjective(marketingTarget int) string {
	if value := map[int]string{3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词", 13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广"}[marketingTarget]; value != "" {
		return value
	}
	return fmt.Sprintf("聚光营销目标 %d", marketingTarget)
}

func normalizeQuickKeywords(values []string) ([]string, error) {
	if len(values) > 200 {
		return nil, errors.New("keywords cannot contain more than 200 values")
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > 50 {
			return nil, errors.New("each keyword must contain at most 50 characters")
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func quickPlanSPUNotes(template QuickPlanTemplate, noteID string) []SPUNoteSpec {
	spus := template.noteSPUs[strings.ToLower(noteID)]
	if len(spus) == 0 && len(template.spec.Units) > 0 {
		spus = template.spec.Units[0].SPUNotes
	}
	if len(spus) == 0 || strings.TrimSpace(spus[0].SPUID) == "" {
		return nil
	}
	return []SPUNoteSpec{{SPUID: spus[0].SPUID, NoteIDs: []string{noteID}}}
}

func quickPlanItemNotes(template QuickPlanTemplate, noteID string) []ItemNoteSpec {
	items := template.noteItems[strings.ToLower(noteID)]
	if len(items) == 0 && len(template.spec.Units) > 0 {
		items = template.spec.Units[0].ItemNotes
	}
	if len(items) == 0 || strings.TrimSpace(items[0].ItemID) == "" {
		return nil
	}
	return []ItemNoteSpec{{ItemID: items[0].ItemID, NoteIDs: []string{noteID}}}
}

func cloneQuickTarget(value TargetSpec) TargetSpec {
	value.TemplateConfig = append(json.RawMessage(nil), value.TemplateConfig...)
	value.ContentInterests = append([]CodeName(nil), value.ContentInterests...)
	value.ShoppingInterests = append([]CodeName(nil), value.ShoppingInterests...)
	value.CrowdPackages = append([]CrowdPackage(nil), value.CrowdPackages...)
	value.BehaviorKeywords = append([]string(nil), value.BehaviorKeywords...)
	value.InterestKeywords = append([]string(nil), value.InterestKeywords...)
	value.KeywordTargetActions = append([]int(nil), value.KeywordTargetActions...)
	return value
}

func quickPlanName(placement, title, noteID, idempotencyKey string, now time.Time) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "笔记" + noteID[len(noteID)-6:]
	}
	label := "信息流"
	if placement == "search" {
		label = "搜索"
	}
	digest := sha256.Sum256([]byte(idempotencyKey))
	suffix := hex.EncodeToString(digest[:2])
	return limitRunes(fmt.Sprintf("%s-%s-%s-%s", now.Format("0102"), label, title, suffix), 50)
}

func intPointer(value int) *int {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}

func floatPointer(value float64) *float64 {
	return &value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func quickMaxInt64(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}
