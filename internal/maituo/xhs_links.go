package maituo

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
	Creativities       []XHSLinkCreativity `json:"creativities"`
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
	NoteID       string         `json:"note_id"`
	CampaignName string         `json:"campaign_name"`
	Placement    string         `json:"placement"`
	Subaccounts  []string       `json:"subaccounts"`
	Spend        float64        `json:"spend"`
	SearchUsers  int64          `json:"search_users"`
	SearchCost   float64        `json:"search_cost"`
	Matches      []XHSLinkMatch `json:"matches"`
}

type XHSLinkResult struct {
	ReportDate string        `json:"report_date"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	Items      []XHSLinkItem `json:"items"`
}
