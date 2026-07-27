package maituo

import "time"

const (
	SheetKPI        = "总览KPI"
	SheetNotes      = "笔记明细"
	SheetSPU        = "分SPU总览"
	SheetSubaccount = "分子账户"
	SheetTrend      = "淘搜趋势"
)

var WorkbookSheets = []string{SheetKPI, SheetNotes, SheetSPU, SheetSubaccount, SheetTrend}

type Snapshot struct {
	FileName      string
	FileSHA256    string
	ReportDate    time.Time
	PresentSheets []string
	KPIs          []KPI
	Notes         []NoteDetail
	SPUs          []SPUOverview
	Subaccounts   []SubaccountOverview
	Trends        []SearchTrend
}

func (snapshot Snapshot) HasSheet(name string) bool {
	for _, sheet := range snapshot.PresentSheets {
		if sheet == name {
			return true
		}
	}
	return false
}

func MissingSheets(present []string) []string {
	seen := make(map[string]struct{}, len(present))
	for _, sheet := range present {
		seen[sheet] = struct{}{}
	}
	missing := make([]string, 0, len(WorkbookSheets)-len(present))
	for _, sheet := range WorkbookSheets {
		if _, ok := seen[sheet]; !ok {
			missing = append(missing, sheet)
		}
	}
	return missing
}

type RowMetadata struct {
	SourceRow   int    `json:"-"`
	ContentHash string `json:"-"`
}

type KPI struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	DataBasis string  `json:"data_basis"`
	RowMetadata
}

type NoteDetail struct {
	NoteID                string   `json:"note_id"`
	NoteURL               string   `json:"note_url"`
	Category              string   `json:"category"`
	Subaccount            string   `json:"subaccount"`
	CampaignName          string   `json:"campaign_name"`
	Placement             string   `json:"placement"`
	KeywordCategoryNote   *string  `json:"keyword_category_note"`
	Spend                 float64  `json:"spend"`
	SearchUsers           int64    `json:"search_users"`
	SearchCost            *float64 `json:"search_cost"`
	EstimatedPostbackCost *float64 `json:"estimated_postback_cost"`
	SearchRatePct         *float64 `json:"search_rate_pct"`
	CPC                   float64  `json:"cpc"`
	CTRPct                float64  `json:"ctr_pct"`
	RowMetadata
}

type SPUOverview struct {
	SPU           string  `json:"spu"`
	AuctionSpend  float64 `json:"auction_spend"`
	SearchUsers   int64   `json:"search_users"`
	SearchCost    float64 `json:"search_cost"`
	SearchRatePct float64 `json:"search_rate_pct"`
	CPC           float64 `json:"cpc"`
	CTRPct        float64 `json:"ctr_pct"`
	NoteCount     int64   `json:"note_count"`
	RowMetadata
}

type SubaccountOverview struct {
	SPU                   string   `json:"spu"`
	Subaccount            string   `json:"subaccount"`
	Placement             string   `json:"placement"`
	SearchCost            *float64 `json:"search_cost"`
	EstimatedPostbackCost *float64 `json:"estimated_postback_cost"`
	Spend                 float64  `json:"spend"`
	SearchUsers           int64    `json:"search_users"`
	SearchRatePct         *float64 `json:"search_rate_pct"`
	CPC                   *float64 `json:"cpc"`
	CTRPct                *float64 `json:"ctr_pct"`
	NoteCount             int64    `json:"note_count"`
	RowMetadata
}

type SearchTrend struct {
	Date                  time.Time `json:"date"`
	CoenzymeSpend         *float64  `json:"coenzyme_spend"`
	CoenzymeSearchUV      *int64    `json:"coenzyme_search_uv"`
	CoenzymeOrderUV       *int64    `json:"coenzyme_order_uv"`
	CoenzymeSearchCost    *float64  `json:"coenzyme_search_cost"`
	KrillOilSpend         *float64  `json:"krill_oil_spend"`
	KrillOilSearchUV      *int64    `json:"krill_oil_search_uv"`
	KrillOilOrderUV       *int64    `json:"krill_oil_order_uv"`
	KrillOilSearchCost    *float64  `json:"krill_oil_search_cost"`
	TotalSearchUV         *int64    `json:"total_search_uv"`
	TotalOrderUV          *int64    `json:"total_order_uv"`
	TotalSearchCost       *float64  `json:"total_search_cost"`
	TotalSpend            *float64  `json:"total_spend"`
	TotalRecallSearchCost *float64  `json:"total_recall_search_cost"`
	RowMetadata
}

type TableResult struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Fetched   int    `json:"fetched"`
	Inserted  int    `json:"inserted"`
	Updated   int    `json:"updated"`
	Unchanged int    `json:"unchanged"`
	Deleted   int64  `json:"deleted"`
}

type ImportResult struct {
	RunID         int64         `json:"run_id"`
	FileName      string        `json:"file_name"`
	FileSHA256    string        `json:"file_sha256"`
	ReportDate    string        `json:"report_date"`
	AlreadySaved  bool          `json:"already_saved"`
	PresentSheets []string      `json:"present_sheets"`
	MissingSheets []string      `json:"missing_sheets"`
	TableCount    int           `json:"table_count"`
	Fetched       int           `json:"fetched"`
	Inserted      int           `json:"inserted"`
	Updated       int           `json:"updated"`
	Unchanged     int           `json:"unchanged"`
	Deleted       int64         `json:"deleted"`
	Tables        []TableResult `json:"tables"`
}

type SavedImport struct {
	RunID         int64     `json:"run_id"`
	FileName      string    `json:"file_name"`
	FileSHA256    string    `json:"file_sha256"`
	ReportDate    string    `json:"report_date"`
	Fetched       int       `json:"fetched"`
	PresentSheets []string  `json:"present_sheets"`
	MissingSheets []string  `json:"missing_sheets"`
	CompletedAt   time.Time `json:"completed_at"`
}
