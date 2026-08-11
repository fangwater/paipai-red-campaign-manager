package coenzyme

import "time"

const SheetName = "辅酶q10日数据"

type DailyRecord struct {
	ReportDate        time.Time
	Spend             *float64
	Impressions       *int64
	Clicks            *int64
	CTR               *float64
	CPC               *float64
	CPM               *float64
	AllTransactionGMV *float64
	AllStoreROI       *float64
	PostRefundGMV     *float64
	PostRefundROI     *float64
	CoenzymeGMV       *float64
	CoenzymeROI       *float64
	SameDayGMV        *float64
	SameDayROI        *float64
	SearchSpend       *float64
	SearchGMV         *float64
	SearchROI         *float64
	SearchSpendRatio  *float64
	SourceRowNumber   int
	ContentHash       string
}

type Snapshot struct {
	WikiToken        string
	SpreadsheetToken string
	SheetID          string
	SheetName        string
	Records          []DailyRecord
}

type SyncResult struct {
	RunID        int64  `json:"run_id"`
	SheetName    string `json:"sheet_name"`
	Fetched      int    `json:"fetched"`
	Inserted     int    `json:"inserted"`
	Updated      int    `json:"updated"`
	Unchanged    int    `json:"unchanged"`
	EarliestDate string `json:"earliest_date,omitempty"`
	LatestDate   string `json:"latest_date,omitempty"`

	SpreadsheetToken string `json:"-"`
	SheetID          string `json:"-"`
}
