package maituo

import (
	"time"

	"paipai-red-campaign-manager/internal/model"
)

type NoteCampaignAnalysisQuery struct {
	Window   string
	Search   string
	PlanID   string
	Sort     string
	Page     int
	PageSize int
}

type NoteContent struct {
	NoteID           string                  `json:"note_id"`
	NoteURL          string                  `json:"note_url"`
	Found            bool                    `json:"found"`
	NoteContent      string                  `json:"note_content"`
	Blocks           []model.ManuscriptBlock `json:"blocks"`
	ReferenceNoteIDs []string                `json:"reference_note_ids"`
	Providers        []string                `json:"providers"`
	Tags             NoteTags                `json:"tags"`
}

type ReferenceMaterialsQuery struct {
	Search   string
	Filters  ReferenceMaterialFilters
	Page     int
	PageSize int
}

type ReferenceMaterialFilters struct {
	Provider            string `json:"provider"`
	NoteType            string `json:"note_type"`
	CoverType           string `json:"cover_type"`
	CommercialIntensity string `json:"commercial_intensity"`
	Audience            string `json:"audience"`
	UserScenario        string `json:"user_scenario"`
}

type ReferenceMaterialTags struct {
	NoteType            []string `json:"note_type"`
	CoverType           []string `json:"cover_type"`
	CommercialIntensity []string `json:"commercial_intensity"`
	Audience            []string `json:"audience"`
	UserScenario        []string `json:"user_scenario"`
}

type ReferenceMaterialFilterOptions struct {
	Providers           []string `json:"providers"`
	NoteType            []string `json:"note_type"`
	CoverType           []string `json:"cover_type"`
	CommercialIntensity []string `json:"commercial_intensity"`
	Audience            []string `json:"audience"`
	UserScenario        []string `json:"user_scenario"`
}

type ReferenceMaterialSource struct {
	NoteID string `json:"note_id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type ReferenceMaterialItem struct {
	ReferenceNoteID   string                    `json:"reference_note_id"`
	NoteURL           string                    `json:"note_url"`
	SourceNoteIDs     []string                  `json:"source_note_ids"`
	SourceManuscripts []ReferenceMaterialSource `json:"source_manuscripts"`
	Providers         []string                  `json:"providers"`
	Tags              ReferenceMaterialTags     `json:"tags"`
	UsageCount        int                       `json:"usage_count"`
	HasContent        bool                      `json:"has_content"`
	ContentSource     string                    `json:"content_source"`
}

type ReferenceMaterialContent struct {
	ReferenceNoteID string     `json:"note_id"`
	NoteURL         string     `json:"note_url"`
	Found           bool       `json:"found"`
	NoteContent     string     `json:"note_content"`
	Source          string     `json:"source"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type ReferenceMaterialContentInput struct {
	ReferenceNoteID string
	NoteContent     string
}

type ReferenceMaterialStats struct {
	MaterialCount   int `json:"material_count"`
	SourceNoteCount int `json:"source_note_count"`
	ReferenceCount  int `json:"reference_count"`
	ProviderCount   int `json:"provider_count"`
}

type ReferenceMaterials struct {
	Search        string                         `json:"search"`
	Filters       ReferenceMaterialFilters       `json:"filters"`
	FilterOptions ReferenceMaterialFilterOptions `json:"filter_options"`
	Stats         ReferenceMaterialStats         `json:"stats"`
	Total         int                            `json:"total"`
	Page          int                            `json:"page"`
	PageSize      int                            `json:"page_size"`
	Items         []ReferenceMaterialItem        `json:"items"`
}

type NoteTags struct {
	NoteType            []string `json:"note_type"`
	CoverType           []string `json:"cover_type"`
	CommercialIntensity []string `json:"commercial_intensity"`
	Audience            []string `json:"audience"`
	UserScenario        []string `json:"user_scenario"`
	Progress            []string `json:"progress"`
	Complete            bool     `json:"complete"`
	MissingFields       []string `json:"missing_fields"`
}

type ManuscriptAsset struct {
	AssetID     string
	ContentType string
	Content     []byte
	CreatedAt   time.Time
}

type NoteCampaignPoint struct {
	ReportDate      string  `json:"report_date"`
	Spend           float64 `json:"spend"`
	SearchUsers     int64   `json:"search_users"`
	SearchCost      float64 `json:"search_cost"`
	CumulativeSpend float64 `json:"cumulative_spend"`
	CumulativeUsers int64   `json:"cumulative_search_users"`
}

type NoteCampaignAnalysisItem struct {
	NoteID           string              `json:"note_id"`
	CampaignName     string              `json:"campaign_name"`
	Placement        string              `json:"placement"`
	FirstReportDate  string              `json:"first_report_date"`
	LastReportDate   string              `json:"last_report_date"`
	ActiveDays       int                 `json:"active_days"`
	LatestSpend      float64             `json:"latest_spend"`
	TotalSpend       float64             `json:"total_spend"`
	TotalSearchUsers int64               `json:"total_search_users"`
	LatestSearchCost float64             `json:"latest_search_cost"`
	Points           []NoteCampaignPoint `json:"points"`
}

type NoteCampaignAnalysis struct {
	Window      string                     `json:"window"`
	Sort        string                     `json:"sort"`
	ReportDates []string                   `json:"report_dates"`
	Total       int                        `json:"total"`
	Page        int                        `json:"page"`
	PageSize    int                        `json:"page_size"`
	Items       []NoteCampaignAnalysisItem `json:"items"`
}

type TrafficComparisonQuery struct {
	Window   string
	Search   string
	Page     int
	PageSize int
}

type TrafficComparisonPoint struct {
	ReportDate    string  `json:"report_date"`
	Spend         float64 `json:"spend"`
	SearchUsers   int64   `json:"search_users"`
	SearchCost    float64 `json:"search_cost"`
	HasSearchCost bool    `json:"has_search_cost"`
}

type TrafficComparisonCampaign struct {
	CampaignName        string                   `json:"campaign_name"`
	FirstReportDate     string                   `json:"first_report_date"`
	LastReportDate      string                   `json:"last_report_date"`
	ActiveDays          int                      `json:"active_days"`
	LatestSpend         float64                  `json:"latest_spend"`
	LatestSearchUsers   int64                    `json:"latest_search_users"`
	LatestSearchCost    float64                  `json:"latest_search_cost"`
	HasLatestSearchCost bool                     `json:"has_latest_search_cost"`
	TotalSpend          float64                  `json:"total_spend"`
	TotalSearchUsers    int64                    `json:"total_search_users"`
	Points              []TrafficComparisonPoint `json:"points"`
}

type TrafficComparisonItem struct {
	NoteID                  string                      `json:"note_id"`
	Placement               string                      `json:"placement"`
	CampaignCount           int                         `json:"campaign_count"`
	ComparableCampaignCount int                         `json:"comparable_campaign_count"`
	LatestSearchCostMin     float64                     `json:"latest_search_cost_min"`
	LatestSearchCostMax     float64                     `json:"latest_search_cost_max"`
	SearchCostGap           float64                     `json:"search_cost_gap"`
	LatestSpend             float64                     `json:"latest_spend"`
	LatestSearchUsers       int64                       `json:"latest_search_users"`
	Campaigns               []TrafficComparisonCampaign `json:"campaigns"`
}

type TrafficComparison struct {
	Window      string                  `json:"window"`
	ReportDates []string                `json:"report_dates"`
	LatestDate  string                  `json:"latest_date"`
	Total       int                     `json:"total"`
	Page        int                     `json:"page"`
	PageSize    int                     `json:"page_size"`
	Items       []TrafficComparisonItem `json:"items"`
}

type TrafficDeliveryComparisonQuery struct {
	NoteID    string
	Placement string
}

type TrafficDeliveryCampaign struct {
	CampaignName string         `json:"campaign_name"`
	Subaccounts  []string       `json:"subaccounts"`
	Matches      []XHSLinkMatch `json:"matches"`
}

type TrafficDeliveryComparison struct {
	ReportDate string                    `json:"report_date"`
	NoteID     string                    `json:"note_id"`
	Placement  string                    `json:"placement"`
	Campaigns  []TrafficDeliveryCampaign `json:"campaigns"`
}
