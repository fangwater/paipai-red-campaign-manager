package maituo

type NoteCampaignAnalysisQuery struct {
	Window   string
	Search   string
	Sort     string
	Page     int
	PageSize int
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
