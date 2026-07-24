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

const creativitySearchPath = "/api/open/jg/creativity/search"

var ErrInvalidCreativityRequest = errors.New("invalid XHS Spotlight creativity request")

type CreativityListRequest struct {
	AdvertiserID  int64                `json:"advertiser_id"`
	CampaignID    int64                `json:"campaign_id,omitempty"`
	UnitID        int64                `json:"unit_id,omitempty"`
	CreativityIDs []int64              `json:"creativity_ids,omitempty"`
	Status        *int                 `json:"status,omitempty"`
	StartTime     string               `json:"start_time,omitempty"`
	EndTime       string               `json:"end_time,omitempty"`
	Page          *CampaignPageRequest `json:"page,omitempty"`
	NoteID        string               `json:"note_id,omitempty"`
}

type CreativityListData struct {
	Page         CampaignPage `json:"page"`
	Creativities []Creativity `json:"creativity_dtos"`
}

type CreativityCollection struct {
	TotalCount   int          `json:"total_count"`
	Creativities []Creativity `json:"creativity_dtos"`
}

type Creativity struct {
	AdvertiserID          int64           `json:"advertiser_id"`
	CampaignID            int64           `json:"campaign_id"`
	UnitID                int64           `json:"unit_id"`
	CreativityID          int64           `json:"creativity_id"`
	CreativityName        string          `json:"creativity_name"`
	CreativityEnable      int             `json:"creativity_enable"`
	CreativityFilterState int             `json:"creativity_filter_state"`
	CreativityCreateTime  string          `json:"creativity_create_time"`
	CreativityUpdateTime  string          `json:"creativity_update_time"`
	MaterialType          int             `json:"material_type"`
	ConversionType        int             `json:"conversion_type"`
	NoteID                string          `json:"note_id"`
	NoteType              int             `json:"note_type"`
	AuditStatus           int             `json:"audit_status"`
	CreativityAuditState  int             `json:"creativity_audit_state"`
	ItemID                string          `json:"item_id"`
	CreationType          int             `json:"creation_type"`
	RawPayload            json.RawMessage `json:"-"`
}

func (creativity *Creativity) UnmarshalJSON(data []byte) error {
	type creativityAlias Creativity
	var decoded creativityAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.RawPayload = append(json.RawMessage(nil), data...)
	*creativity = Creativity(decoded)
	return nil
}

func (creativity Creativity) MarshalJSON() ([]byte, error) {
	if len(creativity.RawPayload) > 0 {
		return append([]byte(nil), creativity.RawPayload...), nil
	}
	type creativityAlias Creativity
	return json.Marshal(creativityAlias(creativity))
}

type creativityListEnvelope struct {
	Code      int                `json:"code"`
	Success   bool               `json:"success"`
	Message   string             `json:"msg"`
	RequestID string             `json:"request_id"`
	Data      CreativityListData `json:"data"`
}

func (client *Client) ListCreativities(ctx context.Context, accessToken string, request CreativityListRequest) (CreativityListData, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return CreativityListData{}, errors.New("XHS Spotlight access token is required")
	}
	normalized, err := normalizeCreativityListRequest(request)
	if err != nil {
		return CreativityListData{}, err
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return CreativityListData{}, fmt.Errorf("encode XHS Spotlight creativity request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+creativitySearchPath, bytes.NewReader(body))
	if err != nil {
		return CreativityListData{}, fmt.Errorf("create XHS Spotlight creativity request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", accessToken)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return CreativityListData{}, fmt.Errorf("request XHS Spotlight creativities: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return CreativityListData{}, fmt.Errorf("request XHS Spotlight creativities: HTTP %d", resp.StatusCode)
	}

	var envelope creativityListEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&envelope); err != nil {
		return CreativityListData{}, fmt.Errorf("decode XHS Spotlight creativity response: %w", err)
	}
	if err := validateJGResponse("creativity", envelope.Code, envelope.Success, envelope.Message, envelope.RequestID); err != nil {
		return CreativityListData{}, err
	}
	if envelope.Data.Page.TotalCount < 0 {
		return CreativityListData{}, errors.New("XHS Spotlight creativity API returned a negative total_count")
	}
	return envelope.Data, nil
}

func (client *Client) ListAllCreativities(ctx context.Context, accessToken string, request CreativityListRequest) (CreativityCollection, error) {
	normalized, err := normalizeCreativityListRequest(request)
	if err != nil {
		return CreativityCollection{}, err
	}
	normalized.Page = &CampaignPageRequest{PageIndex: 1, PageSize: 100}
	result := CreativityCollection{Creativities: make([]Creativity, 0)}
	seen := make(map[int64]struct{})

	for {
		page, err := client.ListCreativities(ctx, accessToken, normalized)
		if err != nil {
			return CreativityCollection{}, err
		}
		if page.Page.PageIndex != normalized.Page.PageIndex {
			return CreativityCollection{}, fmt.Errorf("XHS Spotlight creativity API returned page_index %d for requested page %d", page.Page.PageIndex, normalized.Page.PageIndex)
		}
		result.TotalCount = page.Page.TotalCount
		added := 0
		for _, creativity := range page.Creativities {
			if creativity.CreativityID <= 0 {
				return CreativityCollection{}, errors.New("XHS Spotlight creativity API returned a non-positive creativity ID")
			}
			if _, exists := seen[creativity.CreativityID]; exists {
				continue
			}
			seen[creativity.CreativityID] = struct{}{}
			result.Creativities = append(result.Creativities, creativity)
			added++
		}
		if len(result.Creativities) >= result.TotalCount {
			return result, nil
		}
		if len(page.Creativities) == 0 || added == 0 {
			return CreativityCollection{}, fmt.Errorf("XHS Spotlight creativity pagination made no progress at page %d before total_count %d", normalized.Page.PageIndex, result.TotalCount)
		}
		normalized.Page.PageIndex++
	}
}

func normalizeCreativityListRequest(request CreativityListRequest) (CreativityListRequest, error) {
	request.StartTime = strings.TrimSpace(request.StartTime)
	request.EndTime = strings.TrimSpace(request.EndTime)
	request.NoteID = strings.TrimSpace(request.NoteID)
	if request.AdvertiserID <= 0 {
		return CreativityListRequest{}, invalidCreativityRequest("advertiser_id must be positive")
	}
	if request.CampaignID < 0 || request.UnitID < 0 {
		return CreativityListRequest{}, invalidCreativityRequest("campaign_id and unit_id cannot be negative")
	}
	if len(request.CreativityIDs) > 20 {
		return CreativityListRequest{}, invalidCreativityRequest("creativity_ids cannot contain more than 20 values")
	}
	for _, creativityID := range request.CreativityIDs {
		if creativityID <= 0 {
			return CreativityListRequest{}, invalidCreativityRequest("creativity_ids must contain only positive values")
		}
	}
	if request.Status != nil && !validCreativityStatus(*request.Status) {
		return CreativityListRequest{}, invalidCreativityRequest("status is not supported by the creativity API")
	}
	if err := validateCampaignDateRange("start_time", request.StartTime, "end_time", request.EndTime); err != nil {
		return CreativityListRequest{}, invalidCreativityRequest(strings.TrimPrefix(err.Error(), ErrInvalidCampaignRequest.Error()+": "))
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
		if page.PageIndex < 1 || page.PageSize < 1 {
			return CreativityListRequest{}, invalidCreativityRequest("page_index and page_size must be positive")
		}
		request.Page = &page
	}
	return request, nil
}

func validCreativityStatus(status int) bool {
	switch status {
	case 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 14, 16:
		return true
	default:
		return false
	}
}

func invalidCreativityRequest(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidCreativityRequest, message)
}
