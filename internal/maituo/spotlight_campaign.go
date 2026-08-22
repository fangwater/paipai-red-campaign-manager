package maituo

import "encoding/json"

type SpotlightCampaignQuery struct {
	Search   string
	Page     int
	PageSize int
}

type SpotlightCampaignSummary struct {
	AdvertiserID        int64  `json:"advertiser_id"`
	AdvertiserName      string `json:"advertiser_name"`
	CampaignID          int64  `json:"campaign_id"`
	CampaignName        string `json:"campaign_name"`
	CampaignFilterState int    `json:"campaign_filter_state"`
	CampaignEnable      int    `json:"campaign_enable"`
	MarketingTarget     int    `json:"marketing_target"`
	Placement           int    `json:"placement"`
	BiddingStrategy     int    `json:"bidding_strategy"`
	CampaignDayBudget   int64  `json:"campaign_day_budget"`
	StartDate           string `json:"start_date,omitempty"`
	ExpireDate          string `json:"expire_date,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	SyncedAt            string `json:"synced_at"`
	UnitCount           int    `json:"unit_count"`
	CreativityCount     int    `json:"creativity_count"`
}

type SpotlightCampaignList struct {
	Total    int                        `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
	Items    []SpotlightCampaignSummary `json:"items"`
}

type SpotlightCampaignEntity struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	CampaignID  int64           `json:"campaign_id"`
	UnitID      int64           `json:"unit_id,omitempty"`
	Enable      int             `json:"enable"`
	FilterState int             `json:"filter_state"`
	CreatedAt   string          `json:"created_at,omitempty"`
	UpdatedAt   string          `json:"updated_at,omitempty"`
	SyncedAt    string          `json:"synced_at"`
	RawPayload  json.RawMessage `json:"raw_payload"`
}

type SpotlightCampaignDetail struct {
	Campaign     SpotlightCampaignSummary  `json:"campaign"`
	RawPayload   json.RawMessage           `json:"raw_payload"`
	Units        []SpotlightCampaignEntity `json:"units"`
	Creativities []SpotlightCampaignEntity `json:"creativities"`
}
