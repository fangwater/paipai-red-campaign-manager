package xhs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const campaignListPath = "/api/open/jg/campaign/list"

var ErrInvalidCampaignRequest = errors.New("invalid XHS Spotlight campaign request")

type CampaignPageRequest struct {
	PageIndex int `json:"-"`
	PageSize  int `json:"-"`
}

func (page CampaignPageRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PageIndex int `json:"page_index,omitempty"`
		PageSize  int `json:"page_size,omitempty"`
	}{PageIndex: page.PageIndex, PageSize: page.PageSize})
}

func (page *CampaignPageRequest) UnmarshalJSON(data []byte) error {
	var value struct {
		PageIndex      *int `json:"page_index"`
		PageSize       *int `json:"page_size"`
		PageIndexCamel *int `json:"pageIndex"`
		PageSizeCamel  *int `json:"pageSize"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if value.PageIndex != nil && value.PageIndexCamel != nil || value.PageSize != nil && value.PageSizeCamel != nil {
		return errors.New("page fields cannot use snake_case and camelCase together")
	}
	if value.PageIndex != nil {
		page.PageIndex = *value.PageIndex
	} else if value.PageIndexCamel != nil {
		page.PageIndex = *value.PageIndexCamel
	}
	if value.PageSize != nil {
		page.PageSize = *value.PageSize
	} else if value.PageSizeCamel != nil {
		page.PageSize = *value.PageSizeCamel
	}
	return nil
}

type CampaignListRequest struct {
	AdvertiserID    int64                `json:"advertiser_id"`
	CampaignIDs     []int64              `json:"campaign_ids,omitempty"`
	StartTime       string               `json:"start_time,omitempty"`
	ExpireTime      string               `json:"expire_time,omitempty"`
	Status          *int                 `json:"status,omitempty"`
	Page            *CampaignPageRequest `json:"page,omitempty"`
	UpdateStartDate string               `json:"update_start_date,omitempty"`
	UpdateEndDate   string               `json:"update_end_date,omitempty"`
}

type CampaignPage struct {
	PageIndex  int `json:"page_index"`
	TotalCount int `json:"total_count"`
}

type CampaignListData struct {
	Page      CampaignPage `json:"page"`
	Campaigns []Campaign   `json:"base_campaign_dtos"`
}

type CampaignCollection struct {
	TotalCount int        `json:"total_count"`
	Campaigns  []Campaign `json:"base_campaign_dtos"`
}

type Campaign struct {
	CampaignID            int64                  `json:"campaign_id"`
	CampaignName          string                 `json:"campaign_name"`
	CampaignFilterState   int                    `json:"campaign_filter_state"`
	CampaignCreateTime    string                 `json:"campaign_create_time"`
	CampaignUpdateTime    string                 `json:"campaign_update_time"`
	CampaignEnable        int                    `json:"campaign_enable"`
	MarketingTarget       int                    `json:"marketing_target"`
	Placement             int                    `json:"placement"`
	OptimizeTarget        int                    `json:"optimize_target"`
	PromotionTarget       int                    `json:"promotion_target"`
	BiddingStrategy       int                    `json:"bidding_strategy"`
	ConstraintType        int                    `json:"constraint_type"`
	ConstraintValue       int64                  `json:"constraint_value"`
	LimitDayBudget        int                    `json:"limit_day_budget"`
	CampaignDayBudget     int64                  `json:"campaign_day_budget"`
	BudgetState           int                    `json:"budget_state"`
	SmartSwitch           int                    `json:"smart_switch"`
	Platform              int                    `json:"platform"`
	PacingMode            int                    `json:"pacing_mode"`
	StartTime             string                 `json:"start_time"`
	ExpireTime            string                 `json:"expire_time"`
	TimePeriod            string                 `json:"time_period"`
	TimePeriodType        int                    `json:"time_period_type"`
	FeedFlag              int                    `json:"feed_flag"`
	BuildType             int                    `json:"build_type"`
	CreativityState       int                    `json:"creativity_state"`
	EventAssetID          int64                  `json:"event_asset_id"`
	AssetEvent            int64                  `json:"asset_event"`
	AssetEventID          int64                  `json:"asset_event_id"`
	PageCategory          int                    `json:"page_category"`
	SearchFlag            int                    `json:"search_flag"`
	SearchBidRatio        float64                `json:"search_bid_ratio"`
	DeeplinkID            int64                  `json:"deeplink_id"`
	UniversalLinkID       int64                  `json:"universal_link_id"`
	DetectURLLink         string                 `json:"detect_url_link"`
	NotAvailableStatus    int                    `json:"not_available_status"`
	OptimizeObjective     int                    `json:"optimize_objective"`
	DeepOptimizeObjective int                    `json:"deep_optimize_objective"`
	ExploreConfig         *CampaignExploreConfig `json:"explore_config,omitempty"`
	CreationType          int                    `json:"creation_type"`
	MarketingIndustry     int                    `json:"marketing_industry"`
}

type CampaignExploreConfig struct {
	CampaignDayBudget int64               `json:"campaign_day_budget"`
	TimePeriod        *CampaignTimePeriod `json:"time_period,omitempty"`
	TimePeriodType    int                 `json:"time_period_type"`
	StartTime         int64               `json:"start_time"`
	ExpireHour        int64               `json:"expire_hour"`
}

type CampaignTimePeriod struct {
	Monday    string `json:"mon"`
	Tuesday   string `json:"tues"`
	Wednesday string `json:"wed"`
	Thursday  string `json:"thur"`
	Friday    string `json:"fri"`
	Saturday  string `json:"sat"`
	Sunday    string `json:"sun"`
}

type campaignListEnvelope struct {
	Code      int              `json:"code"`
	Success   bool             `json:"success"`
	Message   string           `json:"msg"`
	RequestID string           `json:"request_id"`
	Data      CampaignListData `json:"data"`
}

func (client *Client) ListCampaigns(ctx context.Context, accessToken string, request CampaignListRequest) (CampaignListData, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return CampaignListData{}, errors.New("XHS Spotlight access token is required")
	}
	normalized, err := normalizeCampaignListRequest(request)
	if err != nil {
		return CampaignListData{}, err
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return CampaignListData{}, fmt.Errorf("encode XHS Spotlight campaign request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+campaignListPath, bytes.NewReader(body))
	if err != nil {
		return CampaignListData{}, fmt.Errorf("create XHS Spotlight campaign request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", accessToken)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return CampaignListData{}, fmt.Errorf("request XHS Spotlight campaigns: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return CampaignListData{}, fmt.Errorf("request XHS Spotlight campaigns: HTTP %d", resp.StatusCode)
	}

	var envelope campaignListEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return CampaignListData{}, fmt.Errorf("decode XHS Spotlight campaign response: %w", err)
	}
	if !envelope.Success || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = "unknown error"
		}
		if envelope.RequestID != "" {
			return CampaignListData{}, fmt.Errorf("XHS Spotlight campaign API: code=%d message=%s request_id=%s", envelope.Code, message, envelope.RequestID)
		}
		return CampaignListData{}, fmt.Errorf("XHS Spotlight campaign API: code=%d message=%s", envelope.Code, message)
	}
	if envelope.Data.Page.TotalCount < 0 {
		return CampaignListData{}, errors.New("XHS Spotlight campaign API returned a negative total_count")
	}
	return envelope.Data, nil
}

func (client *Client) ListAllCampaigns(ctx context.Context, accessToken string, request CampaignListRequest) (CampaignCollection, error) {
	normalized, err := normalizeCampaignListRequest(request)
	if err != nil {
		return CampaignCollection{}, err
	}
	normalized.Page = &CampaignPageRequest{PageIndex: 1, PageSize: 100}
	result := CampaignCollection{Campaigns: make([]Campaign, 0)}

	for {
		page, err := client.ListCampaigns(ctx, accessToken, normalized)
		if err != nil {
			return CampaignCollection{}, err
		}
		if page.Page.PageIndex != normalized.Page.PageIndex {
			return CampaignCollection{}, fmt.Errorf("XHS Spotlight campaign API returned page_index %d for requested page %d", page.Page.PageIndex, normalized.Page.PageIndex)
		}
		result.TotalCount = page.Page.TotalCount
		result.Campaigns = append(result.Campaigns, page.Campaigns...)
		if len(result.Campaigns) >= result.TotalCount {
			return result, nil
		}
		if len(page.Campaigns) == 0 {
			return CampaignCollection{}, fmt.Errorf("XHS Spotlight campaign pagination stopped at page %d before total_count %d", normalized.Page.PageIndex, result.TotalCount)
		}
		normalized.Page.PageIndex++
	}
}

func normalizeCampaignListRequest(request CampaignListRequest) (CampaignListRequest, error) {
	request.StartTime = strings.TrimSpace(request.StartTime)
	request.ExpireTime = strings.TrimSpace(request.ExpireTime)
	request.UpdateStartDate = strings.TrimSpace(request.UpdateStartDate)
	request.UpdateEndDate = strings.TrimSpace(request.UpdateEndDate)
	if request.AdvertiserID <= 0 {
		return CampaignListRequest{}, invalidCampaignRequest("advertiser_id must be positive")
	}
	if len(request.CampaignIDs) > 20 {
		return CampaignListRequest{}, invalidCampaignRequest("campaign_ids cannot contain more than 20 values")
	}
	for _, campaignID := range request.CampaignIDs {
		if campaignID <= 0 {
			return CampaignListRequest{}, invalidCampaignRequest("campaign_ids must contain only positive values")
		}
	}
	if request.Status != nil && (*request.Status < 1 || *request.Status > 11) {
		return CampaignListRequest{}, invalidCampaignRequest("status must be between 1 and 11")
	}
	if err := validateCampaignDateRange("start_time", request.StartTime, "expire_time", request.ExpireTime); err != nil {
		return CampaignListRequest{}, err
	}
	if err := validateCampaignDateRange("update_start_date", request.UpdateStartDate, "update_end_date", request.UpdateEndDate); err != nil {
		return CampaignListRequest{}, err
	}
	if request.Page == nil {
		request.Page = &CampaignPageRequest{PageIndex: 1, PageSize: 20}
	} else {
		page := *request.Page
		if page.PageIndex == 0 {
			page.PageIndex = 1
		}
		if page.PageSize == 0 {
			page.PageSize = 20
		}
		if page.PageIndex < 1 {
			return CampaignListRequest{}, invalidCampaignRequest("page.pageIndex must be positive")
		}
		if page.PageSize < 1 || page.PageSize > 100 {
			return CampaignListRequest{}, invalidCampaignRequest("page.pageSize must be between 1 and 100")
		}
		request.Page = &page
	}
	return request, nil
}

func validateCampaignDateRange(startName, start, endName, end string) error {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if (start == "") != (end == "") {
		return invalidCampaignRequest(startName + " and " + endName + " must be provided together")
	}
	if start == "" {
		return nil
	}
	startDate, err := time.Parse(time.DateOnly, start)
	if err != nil {
		return invalidCampaignRequest(startName + " must use YYYY-MM-DD")
	}
	endDate, err := time.Parse(time.DateOnly, end)
	if err != nil {
		return invalidCampaignRequest(endName + " must use YYYY-MM-DD")
	}
	if endDate.Before(startDate) {
		return invalidCampaignRequest(endName + " must not be before " + startName)
	}
	return nil
}

func invalidCampaignRequest(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidCampaignRequest, message)
}
