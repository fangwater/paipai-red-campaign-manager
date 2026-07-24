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
)

const unitListPath = "/api/open/jg/unit/list"

var ErrInvalidUnitRequest = errors.New("invalid XHS Spotlight unit request")

type UnitListRequest struct {
	AdvertiserID    int64   `json:"advertiser_id"`
	CampaignID      int64   `json:"campaign_id,omitempty"`
	UnitIDs         []int64 `json:"unit_ids,omitempty"`
	Status          *int    `json:"status,omitempty"`
	UnitName        string  `json:"unit_name,omitempty"`
	StartDate       string  `json:"start_date,omitempty"`
	EndDate         string  `json:"end_date,omitempty"`
	Page            int     `json:"page,omitempty"`
	PageSize        int     `json:"page_size,omitempty"`
	UpdateStartDate string  `json:"update_start_date,omitempty"`
	UpdateEndDate   string  `json:"update_end_date,omitempty"`
}

type UnitListData struct {
	TotalCount int    `json:"total_count"`
	Units      []Unit `json:"unit_infos"`
}

type UnitCollection struct {
	TotalCount int    `json:"total_count"`
	Units      []Unit `json:"unit_infos"`
}

type Unit struct {
	UnitID             int64           `json:"id"`
	CampaignID         int64           `json:"campaign_id"`
	Name               string          `json:"name"`
	Enable             int             `json:"enable"`
	UnitFilterState    int             `json:"unit_filter_state"`
	EventBid           int64           `json:"event_bid"`
	TargetType         int             `json:"target_type"`
	CreateTime         string          `json:"create_time"`
	UpdateTime         string          `json:"update_time"`
	NotAvailableStatus int             `json:"not_available_status"`
	CreationType       int             `json:"creation_type"`
	RawPayload         json.RawMessage `json:"-"`
}

func (unit *Unit) UnmarshalJSON(data []byte) error {
	type unitAlias Unit
	var decoded unitAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.RawPayload = append(json.RawMessage(nil), data...)
	*unit = Unit(decoded)
	return nil
}

func (unit Unit) MarshalJSON() ([]byte, error) {
	if len(unit.RawPayload) > 0 {
		return append([]byte(nil), unit.RawPayload...), nil
	}
	type unitAlias Unit
	return json.Marshal(unitAlias(unit))
}

type unitListEnvelope struct {
	Code      int          `json:"code"`
	Success   bool         `json:"success"`
	Message   string       `json:"msg"`
	RequestID string       `json:"request_id"`
	Data      UnitListData `json:"data"`
}

func (client *Client) ListUnits(ctx context.Context, accessToken string, request UnitListRequest) (UnitListData, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return UnitListData{}, errors.New("XHS Spotlight access token is required")
	}
	normalized, err := normalizeUnitListRequest(request)
	if err != nil {
		return UnitListData{}, err
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return UnitListData{}, fmt.Errorf("encode XHS Spotlight unit request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+unitListPath, bytes.NewReader(body))
	if err != nil {
		return UnitListData{}, fmt.Errorf("create XHS Spotlight unit request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", accessToken)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return UnitListData{}, fmt.Errorf("request XHS Spotlight units: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return UnitListData{}, fmt.Errorf("request XHS Spotlight units: HTTP %d", resp.StatusCode)
	}

	var envelope unitListEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&envelope); err != nil {
		return UnitListData{}, fmt.Errorf("decode XHS Spotlight unit response: %w", err)
	}
	if err := validateJGResponse("unit", envelope.Code, envelope.Success, envelope.Message, envelope.RequestID); err != nil {
		return UnitListData{}, err
	}
	if envelope.Data.TotalCount < 0 {
		return UnitListData{}, errors.New("XHS Spotlight unit API returned a negative total_count")
	}
	return envelope.Data, nil
}

func (client *Client) ListAllUnits(ctx context.Context, accessToken string, request UnitListRequest) (UnitCollection, error) {
	normalized, err := normalizeUnitListRequest(request)
	if err != nil {
		return UnitCollection{}, err
	}
	normalized.Page = 1
	normalized.PageSize = 100
	result := UnitCollection{Units: make([]Unit, 0)}
	seen := make(map[int64]struct{})

	for {
		page, err := client.ListUnits(ctx, accessToken, normalized)
		if err != nil {
			return UnitCollection{}, err
		}
		result.TotalCount = page.TotalCount
		added := 0
		for _, unit := range page.Units {
			if unit.UnitID <= 0 {
				return UnitCollection{}, errors.New("XHS Spotlight unit API returned a non-positive unit ID")
			}
			if _, exists := seen[unit.UnitID]; exists {
				continue
			}
			seen[unit.UnitID] = struct{}{}
			result.Units = append(result.Units, unit)
			added++
		}
		if len(result.Units) >= result.TotalCount {
			return result, nil
		}
		if len(page.Units) == 0 || added == 0 {
			return UnitCollection{}, fmt.Errorf("XHS Spotlight unit pagination made no progress at page %d before total_count %d", normalized.Page, result.TotalCount)
		}
		normalized.Page++
	}
}

func normalizeUnitListRequest(request UnitListRequest) (UnitListRequest, error) {
	request.UnitName = strings.TrimSpace(request.UnitName)
	request.StartDate = strings.TrimSpace(request.StartDate)
	request.EndDate = strings.TrimSpace(request.EndDate)
	request.UpdateStartDate = strings.TrimSpace(request.UpdateStartDate)
	request.UpdateEndDate = strings.TrimSpace(request.UpdateEndDate)
	if request.AdvertiserID <= 0 {
		return UnitListRequest{}, invalidUnitRequest("advertiser_id must be positive")
	}
	if request.CampaignID < 0 {
		return UnitListRequest{}, invalidUnitRequest("campaign_id cannot be negative")
	}
	if len(request.UnitIDs) > 10 {
		return UnitListRequest{}, invalidUnitRequest("unit_ids cannot contain more than 10 values")
	}
	for _, unitID := range request.UnitIDs {
		if unitID <= 0 {
			return UnitListRequest{}, invalidUnitRequest("unit_ids must contain only positive values")
		}
	}
	if request.Status != nil && *request.Status != 1 && *request.Status != 2 {
		return UnitListRequest{}, invalidUnitRequest("status must be 1 or 2")
	}
	if err := validateCampaignDateRange("start_date", request.StartDate, "end_date", request.EndDate); err != nil {
		return UnitListRequest{}, invalidUnitRequest(strings.TrimPrefix(err.Error(), ErrInvalidCampaignRequest.Error()+": "))
	}
	if err := validateCampaignDateRange("update_start_date", request.UpdateStartDate, "update_end_date", request.UpdateEndDate); err != nil {
		return UnitListRequest{}, invalidUnitRequest(strings.TrimPrefix(err.Error(), ErrInvalidCampaignRequest.Error()+": "))
	}
	if request.Page == 0 {
		request.Page = 1
	}
	if request.PageSize == 0 {
		request.PageSize = 20
	}
	if request.Page < 1 {
		return UnitListRequest{}, invalidUnitRequest("page must be positive")
	}
	if request.PageSize < 1 {
		return UnitListRequest{}, invalidUnitRequest("page_size must be positive")
	}
	return request, nil
}

func invalidUnitRequest(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidUnitRequest, message)
}

func validateJGResponse(resource string, code int, success bool, message, requestID string) error {
	if success && code == 0 {
		return nil
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "unknown error"
	}
	if requestID != "" {
		return fmt.Errorf("XHS Spotlight %s API: code=%d message=%s request_id=%s", resource, code, message, requestID)
	}
	return fmt.Errorf("XHS Spotlight %s API: code=%d message=%s", resource, code, message)
}
