package delivery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RecommendationSchemaVersion = "delivery-recommendation/v1"
	RulesVersion                = "delivery-rules/2026-08-13"
	MediaContractVersion        = "xhs-jg/2026-05-candidate"
)

var (
	ErrNotFound          = errors.New("delivery resource not found")
	ErrConflict          = errors.New("delivery resource conflict")
	ErrForbidden         = errors.New("delivery operation forbidden")
	ErrValidation        = errors.New("delivery validation failed")
	ErrWritesDisabled    = errors.New("delivery media writes are disabled")
	ErrApprovalStale     = errors.New("delivery approval is stale")
	ErrApprovalRequired  = errors.New("delivery approvals are incomplete")
	ErrCapabilityExpired = errors.New("delivery capability snapshot is unavailable or expired")
)

type Actor struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type Advertiser struct {
	ID   int64  `json:"advertiser_id"`
	Name string `json:"advertiser_name"`
}

type BudgetPolicy struct {
	DailyLimitFen          int64 `json:"daily_limit_fen"`
	TotalLimitFen          int64 `json:"total_limit_fen"`
	AdvertiserDailyCapFen  int64 `json:"advertiser_daily_cap_fen,omitempty"`
	MaxBidFen              int64 `json:"max_bid_fen,omitempty"`
	StopLossSpendFen       int64 `json:"stop_loss_spend_fen,omitempty"`
	StopLossConversionsMin int64 `json:"stop_loss_conversions_min,omitempty"`
}

type CampaignSpec struct {
	LocalKey              string            `json:"local_key"`
	Name                  string            `json:"name"`
	MarketingTarget       int               `json:"marketing_target"`
	Placement             int               `json:"placement"`
	PromotionTarget       int               `json:"promotion_target"`
	Enable                int               `json:"enable"`
	TimeType              int               `json:"time_type"`
	StartTime             string            `json:"start_time,omitempty"`
	ExpireTime            string            `json:"expire_time,omitempty"`
	TimePeriodType        int               `json:"time_period_type"`
	TimePeriod            map[string]string `json:"time_period,omitempty"`
	BiddingStrategy       int               `json:"bidding_strategy"`
	LimitDayBudget        int               `json:"limit_day_budget"`
	DayBudgetFen          int64             `json:"day_budget_fen,omitempty"`
	OptimizeTarget        int               `json:"optimize_target"`
	ConstraintType        *int              `json:"constraint_type,omitempty"`
	SmartSwitch           *int              `json:"smart_switch,omitempty"`
	PacingMode            int               `json:"pacing_mode,omitempty"`
	FeedFlag              *int              `json:"feed_flag,omitempty"`
	BuildType             *int              `json:"build_type,omitempty"`
	EventAssetID          int64             `json:"event_asset_id,omitempty"`
	AssetEvent            int               `json:"asset_event,omitempty"`
	AssetEventID          int64             `json:"asset_event_id,omitempty"`
	PageCategory          int               `json:"page_category,omitempty"`
	SearchFlag            *int              `json:"search_flag,omitempty"`
	TargetExtensionSwitch int               `json:"target_extension_switch,omitempty"`
	SearchBidRatio        float64           `json:"search_bid_ratio,omitempty"`
	DeeplinkID            int64             `json:"deeplink_id,omitempty"`
	UniversalLinkID       int64             `json:"universal_link_id,omitempty"`
	DetectURLLink         string            `json:"detect_url_link,omitempty"`
}

type CodeName struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CrowdPackage struct {
	Value     string `json:"value"`
	Name      string `json:"name,omitempty"`
	GroupID   string `json:"group_id,omitempty"`
	Type      string `json:"type,omitempty"`
	SyncState int    `json:"sync_status,omitempty"`
	Status    int    `json:"status,omitempty"`
}

type TargetSpec struct {
	Gender                    string         `json:"gender,omitempty"`
	Age                       string         `json:"age,omitempty"`
	Device                    string         `json:"device,omitempty"`
	Cities                    string         `json:"cities,omitempty"`
	ContentInterests          []CodeName     `json:"content_interests,omitempty"`
	ShoppingInterests         []CodeName     `json:"shopping_interests,omitempty"`
	CrowdPackages             []CrowdPackage `json:"crowd_packages,omitempty"`
	BehaviorKeywords          []string       `json:"behavior_keywords,omitempty"`
	InterestKeywords          []string       `json:"interest_keywords,omitempty"`
	KeywordTargetPeriod       int            `json:"keyword_target_period,omitempty"`
	KeywordTargetActions      []int          `json:"keyword_target_actions,omitempty"`
	IntelligentExpansion      int            `json:"intelligent_expansion,omitempty"`
	ExcludeBloggerFans        bool           `json:"exclude_blogger_fans,omitempty"`
	ExcludeBloggerPurchasers  bool           `json:"exclude_blogger_purchasers,omitempty"`
	IncludeBrandRecognition   bool           `json:"include_brand_recognition,omitempty"`
	IncludeCategoryInterested bool           `json:"include_category_interested,omitempty"`
}

type KeywordBid struct {
	Keyword         string `json:"keyword"`
	BidFen          int64  `json:"bid_fen"`
	FeedBidFen      int64  `json:"feed_bid_fen,omitempty"`
	KeywordSource   int    `json:"keyword_source,omitempty"`
	PhraseMatchType int    `json:"phrase_match_type"`
}

type NegativeKeyword struct {
	Keyword         string `json:"keyword"`
	PhraseMatchType int    `json:"phrase_match_type"`
}

type SPUNoteSpec struct {
	SPUID   string   `json:"spu_id"`
	NoteIDs []string `json:"note_ids"`
}

type UnitSpec struct {
	LocalKey            string            `json:"local_key"`
	Name                string            `json:"name"`
	EventBidFen         int64             `json:"event_bid_fen,omitempty"`
	NoteIDs             []string          `json:"note_ids,omitempty"`
	PromotionTarget     int               `json:"promotion_target"`
	TargetType          int               `json:"target_type"`
	Target              TargetSpec        `json:"target"`
	KeywordTargetPeriod int               `json:"keyword_target_period,omitempty"`
	KeywordTargetAction []int             `json:"keyword_target_actions,omitempty"`
	BusinessTreeName    string            `json:"business_tree_name,omitempty"`
	SPUNotes            []SPUNoteSpec     `json:"spu_notes,omitempty"`
	Keywords            []KeywordBid      `json:"keywords,omitempty"`
	NegativeKeywords    []NegativeKeyword `json:"negative_keywords,omitempty"`
	SubstitutedUserID   string            `json:"substituted_user_id,omitempty"`
	KeywordGenType      int               `json:"keyword_gen_type,omitempty"`
	PageID              string            `json:"page_id,omitempty"`
	LandingPageURL      string            `json:"landing_page_url,omitempty"`
	ExternalPageURL     string            `json:"external_page_url,omitempty"`
	LandingPageDesc     string            `json:"landing_page_desc,omitempty"`
	TargetTemplateID    string            `json:"target_template_id,omitempty"`
	Creativities        []CreativitySpec  `json:"creativities"`
}

type QualificationSpec struct {
	ApplyID           string  `json:"apply_id,omitempty"`
	ProductQualIDList []int64 `json:"product_qual_id_list,omitempty"`
	BrandQualIDList   []int64 `json:"brand_qual_id_list,omitempty"`
}

type CreativitySpec struct {
	LocalKey                 string             `json:"local_key"`
	Name                     string             `json:"name"`
	NoteID                   string             `json:"note_id"`
	ClickURLs                []string           `json:"click_urls,omitempty"`
	ExpoURLs                 []string           `json:"expo_urls,omitempty"`
	MaskPrefer               int                `json:"mask_prefer,omitempty"`
	TitleMaskPrefer          int                `json:"title_mask_prefer,omitempty"`
	ConversionType           int                `json:"conversion_type,omitempty"`
	JumpURL                  string             `json:"jump_url,omitempty"`
	LandingPageType          int                `json:"landing_page_type,omitempty"`
	BarContent               string             `json:"bar_content,omitempty"`
	ConversionComponentTypes []int              `json:"conversion_component_types,omitempty"`
	Comment                  string             `json:"comment,omitempty"`
	AppComponentIcon         string             `json:"app_component_icon,omitempty"`
	FallbackJumpURL          string             `json:"fallback_jump_url,omitempty"`
	Qualification            *QualificationSpec `json:"qualification,omitempty"`
}

type ExperimentSpec struct {
	PrimaryMetric string   `json:"primary_metric"`
	Guardrails    []string `json:"guardrails,omitempty"`
	Variables     []string `json:"variables,omitempty"`
	HoldConstant  []string `json:"hold_constant,omitempty"`
}

type DraftSpec struct {
	AdvertiserID int64          `json:"advertiser_id"`
	Objective    string         `json:"objective"`
	Placement    string         `json:"placement"`
	Budget       BudgetPolicy   `json:"budget"`
	Notes        []string       `json:"notes,omitempty"`
	Campaign     CampaignSpec   `json:"campaign"`
	Units        []UnitSpec     `json:"units"`
	Experiment   ExperimentSpec `json:"experiment"`
}

type CreateDraftInput struct {
	Spec           DraftSpec `json:"spec"`
	IdempotencyKey string    `json:"idempotency_key"`
	ChangeReason   string    `json:"change_reason,omitempty"`
}

type UpdateDraftInput struct {
	Spec            DraftSpec `json:"spec"`
	ExpectedVersion int       `json:"expected_version"`
	ChangeReason    string    `json:"change_reason"`
}

type Draft struct {
	ID             string    `json:"id"`
	AdvertiserID   int64     `json:"advertiser_id"`
	Status         string    `json:"status"`
	CurrentVersion int       `json:"current_version"`
	Spec           DraftSpec `json:"spec"`
	SpecHash       string    `json:"spec_hash"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedBy      string    `json:"created_by"`
	UpdatedBy      string    `json:"updated_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Recommendation struct {
	ID            string         `json:"id"`
	DraftID       string         `json:"draft_id"`
	DraftVersion  int            `json:"draft_version"`
	SchemaVersion string         `json:"schema_version"`
	LLMProvider   string         `json:"llm_provider"`
	LLMModel      string         `json:"llm_model"`
	RankerFamily  string         `json:"ranker_family"`
	RankerVersion string         `json:"ranker_version"`
	RulesVersion  string         `json:"rules_version"`
	Payload       map[string]any `json:"payload"`
	Warnings      []string       `json:"warnings"`
	CreatedBy     string         `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
}

type ValidationIssue struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type Validation struct {
	ID                 string            `json:"id"`
	DraftID            string            `json:"draft_id"`
	DraftVersion       int               `json:"draft_version"`
	SpecHash           string            `json:"spec_hash"`
	RulesVersion       string            `json:"rules_version"`
	ContractVersion    string            `json:"contract_version"`
	Valid              bool              `json:"valid"`
	Errors             []ValidationIssue `json:"errors"`
	Warnings           []ValidationIssue `json:"warnings"`
	CapabilitySnapshot map[string]any    `json:"capability_snapshot"`
	ValidUntil         time.Time         `json:"valid_until"`
	CreatedBy          string            `json:"created_by"`
	CreatedAt          time.Time         `json:"created_at"`
}

type ApprovalInput struct {
	Role              string `json:"role"`
	Decision          string `json:"decision"`
	Comment           string `json:"comment,omitempty"`
	ApprovedBudgetFen int64  `json:"approved_budget_fen"`
	ExpiresInMinutes  int    `json:"expires_in_minutes,omitempty"`
}

type Approval struct {
	ID                string    `json:"id"`
	DraftID           string    `json:"draft_id"`
	DraftVersion      int       `json:"draft_version"`
	SpecHash          string    `json:"spec_hash"`
	Role              string    `json:"role"`
	Decision          string    `json:"decision"`
	Actor             string    `json:"actor"`
	Comment           string    `json:"comment"`
	ApprovedBudgetFen int64     `json:"approved_budget_fen"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
}

type PublishInput struct {
	Mode           string `json:"mode"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PublishJob struct {
	ID             string         `json:"id"`
	DraftID        string         `json:"draft_id"`
	DraftVersion   int            `json:"draft_version"`
	AdvertiserID   int64          `json:"advertiser_id"`
	Mode           string         `json:"mode"`
	Status         string         `json:"status"`
	CurrentStep    string         `json:"current_step"`
	IdempotencyKey string         `json:"idempotency_key"`
	RequestPreview map[string]any `json:"request_preview"`
	Result         map[string]any `json:"result"`
	ErrorCode      string         `json:"error_code,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	RetryCount     int            `json:"retry_count"`
	RequestedBy    string         `json:"requested_by"`
	RequestedRole  string         `json:"requested_role"`
	CreatedAt      time.Time      `json:"created_at"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Workflow struct {
	Draft          Draft           `json:"draft"`
	Recommendation *Recommendation `json:"recommendation,omitempty"`
	Validation     *Validation     `json:"validation,omitempty"`
	Approvals      []Approval      `json:"approvals"`
	Jobs           []PublishJob    `json:"jobs"`
	Entities       []MediaEntity   `json:"entities"`
}

type MediaEntity struct {
	ID              string         `json:"id"`
	JobID           string         `json:"job_id"`
	DraftID         string         `json:"draft_id"`
	AdvertiserID    int64          `json:"advertiser_id"`
	EntityType      string         `json:"entity_type"`
	LocalKey        string         `json:"local_key"`
	ParentLocalKey  string         `json:"parent_local_key,omitempty"`
	MediaID         int64          `json:"media_id"`
	ParentMediaID   int64          `json:"parent_media_id,omitempty"`
	DesiredStatus   string         `json:"desired_status"`
	ObservedStatus  string         `json:"observed_status"`
	UpstreamPayload map[string]any `json:"upstream_payload"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type Capability struct {
	AdvertiserID       int64          `json:"advertiser_id"`
	Authorized         bool           `json:"authorized"`
	AdvertiserAllowed  bool           `json:"advertiser_allowed"`
	Scopes             []string       `json:"scopes"`
	RequiredScopes     []string       `json:"required_scopes"`
	MissingScopes      []string       `json:"missing_scopes"`
	AdvertiserCount    int            `json:"advertiser_count"`
	MediaWritesEnabled bool           `json:"media_writes_enabled"`
	ContractVersion    string         `json:"contract_version"`
	Operations         map[string]any `json:"operations"`
	CheckedAt          time.Time      `json:"checked_at"`
}

type PerformanceQuery struct {
	AdvertiserID int64          `json:"advertiser_id"`
	Level        string         `json:"level"`
	Realtime     bool           `json:"realtime"`
	StartDate    string         `json:"start_date"`
	EndDate      string         `json:"end_date"`
	Page         int            `json:"page,omitempty"`
	PageSize     int            `json:"page_size,omitempty"`
	SplitColumns []string       `json:"split_columns,omitempty"`
	Filters      map[string]any `json:"filters,omitempty"`
}

func NewID(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate delivery ID: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(bytes[:]), nil
}

func HashSpec(spec DraftSpec) (string, []byte, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", nil, fmt.Errorf("encode delivery spec: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), encoded, nil
}

func NormalizeSpec(spec DraftSpec) DraftSpec {
	spec.Objective = strings.TrimSpace(spec.Objective)
	spec.Placement = strings.TrimSpace(spec.Placement)
	spec.Campaign.LocalKey = defaultKey(spec.Campaign.LocalKey, "campaign")
	spec.Campaign.Name = strings.TrimSpace(spec.Campaign.Name)
	// A newly published campaign is always paused. Activation is a separate audited action.
	spec.Campaign.Enable = 0
	for index := range spec.Notes {
		spec.Notes[index] = strings.TrimSpace(spec.Notes[index])
	}
	for unitIndex := range spec.Units {
		unit := &spec.Units[unitIndex]
		unit.LocalKey = defaultKey(unit.LocalKey, fmt.Sprintf("unit-%d", unitIndex+1))
		unit.Name = strings.TrimSpace(unit.Name)
		for keywordIndex := range unit.Keywords {
			unit.Keywords[keywordIndex].Keyword = strings.TrimSpace(unit.Keywords[keywordIndex].Keyword)
		}
		for keywordIndex := range unit.NegativeKeywords {
			unit.NegativeKeywords[keywordIndex].Keyword = strings.TrimSpace(unit.NegativeKeywords[keywordIndex].Keyword)
		}
		for creativeIndex := range unit.Creativities {
			creative := &unit.Creativities[creativeIndex]
			creative.LocalKey = defaultKey(creative.LocalKey, fmt.Sprintf("creative-%d-%d", unitIndex+1, creativeIndex+1))
			creative.Name = strings.TrimSpace(creative.Name)
			creative.NoteID = strings.TrimSpace(creative.NoteID)
		}
	}
	spec.Experiment.PrimaryMetric = strings.TrimSpace(spec.Experiment.PrimaryMetric)
	return spec
}

func defaultKey(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
