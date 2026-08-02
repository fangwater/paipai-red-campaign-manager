package model

import "time"

// GuoraiSnapshot is one complete note or plan query window ready for PostgreSQL storage.
type GuoraiSnapshot struct {
	EntityType            string
	EnterpriseID          int64
	XHSBrandID            string
	BrandName             string
	MerchantID            string
	AttributionShop       string
	WindowStart           time.Time
	WindowEnd             time.Time
	SnapshotDate          time.Time
	SourceCutoffDate      time.Time
	AttributionType       string
	AttributionModel      string
	AttributionWindowDays int
	TrafficType           string
	RequestPayload        []byte
	RawResponse           []byte
	Records               []byte
}

type GuoraiStoreResult struct {
	FetchID int64
	Rows    int
}

type GuoraiLatestQuery struct {
	EntityType string
	SPU        string
	Search     string
	Sort       string
	Page       int
	PageSize   int
}

type GuoraiLatestSnapshot struct {
	FetchID               int64      `json:"fetch_id"`
	EntityType            string     `json:"entity_type"`
	SnapshotDate          string     `json:"snapshot_date"`
	WindowStart           string     `json:"window_start"`
	WindowEnd             string     `json:"window_end"`
	WindowDays            int        `json:"window_days"`
	SourceCutoffDate      string     `json:"source_cutoff_date"`
	BrandName             string     `json:"brand_name"`
	AttributionType       string     `json:"attribution_type"`
	AttributionModel      string     `json:"attribution_model"`
	AttributionWindowDays int        `json:"attribution_window_days"`
	RowCount              int        `json:"row_count"`
	FinishedAt            *time.Time `json:"finished_at"`
}

type GuoraiMetrics struct {
	TotalPayAmount   *float64 `json:"total_pay_amount"`
	PartPayAmount    *float64 `json:"part_pay_amount"`
	AdCost           *float64 `json:"ad_cost"`
	ClickCount       *int64   `json:"click_count"`
	InteractionCount *int64   `json:"interaction_count"`
	TotalROI         *float64 `json:"total_roi"`
}

type GuoraiLatestSummary struct {
	ItemCount       int           `json:"item_count"`
	AccountCount    int           `json:"account_count"`
	LinkedCount     int           `json:"linked_count"`
	NewCount        int           `json:"new_count"`
	MetricItemCount int           `json:"metric_item_count"`
	Metrics         GuoraiMetrics `json:"metrics"`
}

type GuoraiLatestItem struct {
	ID              string        `json:"id"`
	URL             string        `json:"url"`
	Name            string        `json:"name"`
	AuthorName      string        `json:"author_name"`
	AccountName     string        `json:"account_name"`
	PublishTime     string        `json:"publish_time"`
	PictureURL      string        `json:"picture_url"`
	SPUID           string        `json:"spu_id"`
	SPUName         string        `json:"spu_name"`
	Tag             string        `json:"tag"`
	PlanType        string        `json:"plan_type"`
	NoteType        int           `json:"note_type"`
	LinkedNoteCount int           `json:"linked_note_count"`
	IsNew           bool          `json:"is_new"`
	Metrics         GuoraiMetrics `json:"metrics"`
}

type GuoraiLatestResult struct {
	EntityType string                `json:"entity_type"`
	SPU        string                `json:"spu"`
	Sort       string                `json:"sort"`
	Snapshot   *GuoraiLatestSnapshot `json:"snapshot"`
	Summary    GuoraiLatestSummary   `json:"summary"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	Items      []GuoraiLatestItem    `json:"items"`
}
