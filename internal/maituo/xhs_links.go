package maituo

import (
	"encoding/json"
	"strconv"
	"strings"
)

type XHSLinkQuery struct {
	Search   string
	Page     int
	PageSize int
}

type XHSLinkCreativity struct {
	CreativityID          int64  `json:"creativity_id"`
	CreativityName        string `json:"creativity_name"`
	CreativityEnable      int    `json:"creativity_enable"`
	CreativityFilterState int    `json:"creativity_filter_state"`
	MaterialType          int    `json:"material_type"`
	ConversionType        int    `json:"conversion_type"`
	NoteID                string `json:"note_id"`
	ItemID                string `json:"item_id"`
	AuditStatus           int    `json:"audit_status"`
	CreativityAuditState  int    `json:"creativity_audit_state"`
	CreationType          int    `json:"creation_type"`
	CreatedAt             string `json:"created_at,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
	SyncedAt              string `json:"synced_at"`
}

type XHSLinkSearchKeyword struct {
	KeywordID       int64  `json:"keyword_id"`
	Keyword         string `json:"keyword"`
	Bid             int64  `json:"bid"`
	FeedBid         int64  `json:"feed_bid"`
	KeywordSource   int    `json:"keyword_source"`
	PhraseMatchType int    `json:"phrase_match_type"`
}

type XHSLinkCrowdPackage struct {
	Value      string `json:"value"`
	Name       string `json:"name"`
	GroupID    string `json:"group_id,omitempty"`
	Type       string `json:"type,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Status     int    `json:"status"`
	SyncStatus int    `json:"sync_status"`
}

type XHSLinkPremiumCrowd struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Ratio string `json:"ratio"`
}

type XHSLinkTargetConfig struct {
	Gender                string                `json:"gender"`
	Age                   string                `json:"age"`
	City                  string                `json:"city"`
	AreaCode              string                `json:"area_code"`
	Device                string                `json:"device"`
	DevicePrice           string                `json:"device_price"`
	IntelligentExpansion  int                   `json:"intelligent_expansion"`
	GeneralizationSwitch  int                   `json:"generalization_switch"`
	SearchCityIntent      string                `json:"search_city_intent"`
	InterestKeywords      []string              `json:"interest_keywords"`
	BehaviorKeywords      []string              `json:"behavior_keywords"`
	ExcludedCrowds        []string              `json:"excluded_crowds"`
	CrowdPackages         []XHSLinkCrowdPackage `json:"crowd_packages"`
	ContentInterests      []string              `json:"content_interests"`
	ShoppingInterests     []string              `json:"shopping_interests"`
	PremiumCrowds         []XHSLinkPremiumCrowd `json:"premium_crowds"`
	DandelionCrowds       []string              `json:"dandelion_crowds"`
	BrandInterestGroup    bool                  `json:"brand_interest_group"`
	BrandRecognitionGroup bool                  `json:"brand_recognition_group"`
	CategoryInterestGroup bool                  `json:"category_interest_group"`
	GoodsInterestGroup    bool                  `json:"goods_interest_group"`
}

type XHSLinkUnitDelivery struct {
	TargetTemplateID     int64                  `json:"target_template_id"`
	KeywordGenType       int                    `json:"keyword_gen_type"`
	KeywordTargetPeriod  int                    `json:"keyword_target_period"`
	KeywordTargetActions []int                  `json:"keyword_target_actions"`
	SearchKeywordCount   int                    `json:"search_keyword_count"`
	SearchKeywords       []XHSLinkSearchKeyword `json:"search_keywords"`
	Target               XHSLinkTargetConfig    `json:"target"`
}

type XHSLinkUnit struct {
	UnitID             int64               `json:"unit_id"`
	UnitName           string              `json:"unit_name"`
	UnitEnable         int                 `json:"unit_enable"`
	UnitFilterState    int                 `json:"unit_filter_state"`
	EventBid           int64               `json:"event_bid"`
	TargetType         int                 `json:"target_type"`
	NotAvailableStatus int                 `json:"not_available_status"`
	CreationType       int                 `json:"creation_type"`
	CreatedAt          string              `json:"created_at,omitempty"`
	UpdatedAt          string              `json:"updated_at,omitempty"`
	SyncedAt           string              `json:"synced_at"`
	Delivery           XHSLinkUnitDelivery `json:"delivery"`
	Creativities       []XHSLinkCreativity `json:"creativities"`
}

func ParseXHSLinkUnitDelivery(data []byte) XHSLinkUnitDelivery {
	result := XHSLinkUnitDelivery{
		KeywordTargetActions: []int{},
		SearchKeywords:       []XHSLinkSearchKeyword{},
		Target: XHSLinkTargetConfig{
			InterestKeywords: []string{}, BehaviorKeywords: []string{}, ExcludedCrowds: []string{},
			CrowdPackages: []XHSLinkCrowdPackage{}, ContentInterests: []string{}, ShoppingInterests: []string{},
			PremiumCrowds: []XHSLinkPremiumCrowd{}, DandelionCrowds: []string{},
		},
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return result
	}
	result.TargetTemplateID = rawInt64(root, "target_template_id")
	result.KeywordGenType = rawInt(root, "keyword_gen_type")
	result.KeywordTargetPeriod = rawInt(root, "keyword_target_period")
	result.KeywordTargetActions = rawIntSlice(root, "keyword_target_action")
	if raw := firstRaw(root, "keyword_with_bids", "keyword_with_bid"); len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.SearchKeywords)
	}
	result.SearchKeywordCount = len(result.SearchKeywords)
	var target map[string]json.RawMessage
	if json.Unmarshal(firstRaw(root, "target_config", "target_info"), &target) != nil {
		return result
	}
	result.Target.Gender = rawString(target, "target_gender")
	result.Target.Age = rawString(target, "target_age")
	result.Target.City = rawString(target, "target_city")
	result.Target.AreaCode = rawString(target, "target_area_code", "targetAreaCode")
	result.Target.Device = rawString(target, "target_device")
	result.Target.DevicePrice = rawString(target, "target_device_price")
	result.Target.IntelligentExpansion = rawInt(target, "intelligent_expansion")
	result.Target.GeneralizationSwitch = rawInt(target, "target_generalization_switch")
	result.Target.SearchCityIntent = rawString(target, "search_target_city_intent", "searchTargetCityIntent")
	result.Target.InterestKeywords = rawStringSlice(target, "interest_keywords")
	result.Target.BehaviorKeywords = rawStringSlice(target, "keywords")
	result.Target.ExcludedCrowds = rawStringSlice(target, "reverse_target_crowd")
	result.Target.ContentInterests = nestedNames(firstRaw(target, "industry_interest_target"), "content_interests")
	result.Target.ShoppingInterests = nestedNames(firstRaw(target, "industry_interest_target"), "shopping_interests", "shopping_Interests")
	result.Target.DandelionCrowds = collectNames(firstRaw(target, "dandelion_crowd"))
	result.Target.BrandInterestGroup = rawBool(target, "have_brand_interest_group", "haveBrandInterestGroup")
	result.Target.BrandRecognitionGroup = rawBool(target, "have_brand_recognition_group", "haveBrandRecognitionGroup")
	result.Target.CategoryInterestGroup = rawBool(target, "have_category_interest_group", "haveCategoryInterestGroup")
	result.Target.GoodsInterestGroup = rawBool(target, "have_goods_interest_group", "haveGoodsInterestGroup")
	result.Target.CrowdPackages = parseCrowdPackages(firstRaw(target, "crowd_target"))
	result.Target.PremiumCrowds = parsePremiumCrowds(firstRaw(target, "premium_target_crowd"))
	return result
}

func firstRaw(values map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, key := range keys {
		if raw, ok := values[key]; ok && len(raw) > 0 && string(raw) != "null" {
			return raw
		}
	}
	return nil
}

func rawString(values map[string]json.RawMessage, keys ...string) string {
	raw := firstRaw(values, keys...)
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func rawInt(values map[string]json.RawMessage, keys ...string) int {
	value, _ := strconv.Atoi(rawString(values, keys...))
	return value
}

func rawInt64(values map[string]json.RawMessage, keys ...string) int64 {
	var number int64
	if json.Unmarshal(firstRaw(values, keys...), &number) == nil {
		return number
	}
	number, _ = strconv.ParseInt(rawString(values, keys...), 10, 64)
	return number
}

func rawBool(values map[string]json.RawMessage, keys ...string) bool {
	var value bool
	if json.Unmarshal(firstRaw(values, keys...), &value) == nil {
		return value
	}
	return rawInt(values, keys...) == 1
}

func rawStringSlice(values map[string]json.RawMessage, keys ...string) []string {
	result := []string{}
	_ = json.Unmarshal(firstRaw(values, keys...), &result)
	return result
}

func rawIntSlice(values map[string]json.RawMessage, keys ...string) []int {
	result := []int{}
	_ = json.Unmarshal(firstRaw(values, keys...), &result)
	return result
}

func nestedNames(raw json.RawMessage, keys ...string) []string {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return []string{}
	}
	return collectNames(firstRaw(values, keys...))
}

func collectNames(raw json.RawMessage) []string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return []string{}
	}
	seen, result := map[string]struct{}{}, []string{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if name, ok := typed["name"].(string); ok {
				name = strings.TrimSpace(name)
				if name != "" {
					if _, exists := seen[name]; !exists {
						seen[name] = struct{}{}
						result = append(result, name)
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}

func parseCrowdPackages(raw json.RawMessage) []XHSLinkCrowdPackage {
	var target map[string]json.RawMessage
	if json.Unmarshal(raw, &target) != nil {
		return []XHSLinkCrowdPackage{}
	}
	var packages []map[string]json.RawMessage
	if json.Unmarshal(firstRaw(target, "crowd_pkg"), &packages) != nil {
		return []XHSLinkCrowdPackage{}
	}
	result := make([]XHSLinkCrowdPackage, 0, len(packages))
	for _, item := range packages {
		result = append(result, XHSLinkCrowdPackage{Value: rawString(item, "value"), Name: rawString(item, "name"), GroupID: rawString(item, "group_id", "groupId"), Type: rawString(item, "type"), Tag: rawString(item, "tag"), Status: rawInt(item, "status"), SyncStatus: rawInt(item, "sync_status", "syncStatus")})
	}
	return result
}

func parsePremiumCrowds(raw json.RawMessage) []XHSLinkPremiumCrowd {
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return []XHSLinkPremiumCrowd{}
	}
	result := make([]XHSLinkPremiumCrowd, 0, len(items))
	for _, item := range items {
		result = append(result, XHSLinkPremiumCrowd{ID: rawString(item, "id"), Name: rawString(item, "name"), Ratio: rawString(item, "ratio")})
	}
	return result
}

type XHSLinkMatch struct {
	AdvertiserID          int64         `json:"advertiser_id"`
	AdvertiserName        string        `json:"advertiser_name"`
	CampaignID            int64         `json:"campaign_id"`
	CampaignName          string        `json:"campaign_name"`
	CampaignFilterState   int           `json:"campaign_filter_state"`
	CampaignEnable        int           `json:"campaign_enable"`
	MarketingTarget       int           `json:"marketing_target"`
	Placement             int           `json:"placement"`
	OptimizeTarget        int           `json:"optimize_target"`
	OptimizeObjective     int           `json:"optimize_objective"`
	DeepOptimizeObjective int           `json:"deep_optimize_objective"`
	PromotionTarget       int           `json:"promotion_target"`
	BiddingStrategy       int           `json:"bidding_strategy"`
	CampaignDayBudget     int64         `json:"campaign_day_budget"`
	CampaignCreatedAt     string        `json:"campaign_created_at,omitempty"`
	CampaignUpdatedAt     string        `json:"campaign_updated_at,omitempty"`
	StartDate             string        `json:"start_date,omitempty"`
	ExpireDate            string        `json:"expire_date,omitempty"`
	SyncedAt              string        `json:"synced_at"`
	Units                 []XHSLinkUnit `json:"units"`
}

type XHSLinkItem struct {
	NoteID      string         `json:"note_id"`
	Placement   string         `json:"placement"`
	Spend       float64        `json:"spend"`
	SearchUsers int64          `json:"search_users"`
	SearchCost  float64        `json:"search_cost"`
	Matches     []XHSLinkMatch `json:"matches"`
}

type XHSLinkResult struct {
	ReportDate string        `json:"report_date"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	Items      []XHSLinkItem `json:"items"`
}
