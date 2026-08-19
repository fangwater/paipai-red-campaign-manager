package xhs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MaxGatewayPayloadBytes = 2 << 20

var ErrInvalidGatewayRequest = errors.New("invalid XHS Spotlight gateway request")

type GatewayOperation string

const (
	OperationWhiteList              GatewayOperation = "account.white_list"
	OperationBalance                GatewayOperation = "account.balance"
	OperationNoteList               GatewayOperation = "asset.note_list"
	OperationSPUList                GatewayOperation = "asset.spu_list"
	OperationQualificationList      GatewayOperation = "asset.qualification_list"
	OperationEventAssetList         GatewayOperation = "asset.event_list"
	OperationTargetOptions          GatewayOperation = "target.options"
	OperationAudienceEstimate       GatewayOperation = "target.audience_estimate"
	OperationKeywordRecommend       GatewayOperation = "keyword.recommend"
	OperationKeywordWordBags        GatewayOperation = "keyword.word_bags"
	OperationNegativeKeywordList    GatewayOperation = "keyword.negative_list"
	OperationNegativeKeywordAdd     GatewayOperation = "keyword.negative_add"
	OperationNegativeKeywordDelete  GatewayOperation = "keyword.negative_delete"
	OperationUnitKeywordAdd         GatewayOperation = "keyword.unit_add"
	OperationNameDuplicateCheck     GatewayOperation = "delivery.name_check"
	OperationCampaignCreate         GatewayOperation = "campaign.create"
	OperationCampaignUpdate         GatewayOperation = "campaign.update"
	OperationCampaignStatus         GatewayOperation = "campaign.status"
	OperationCampaignList           GatewayOperation = "campaign.list"
	OperationUnitCreate             GatewayOperation = "unit.create"
	OperationUnitUpdate             GatewayOperation = "unit.update"
	OperationUnitStatus             GatewayOperation = "unit.status"
	OperationUnitBidUpdate          GatewayOperation = "unit.bid_update"
	OperationUnitList               GatewayOperation = "unit.list"
	OperationCreativityCreate       GatewayOperation = "creativity.create"
	OperationCreativityUpdate       GatewayOperation = "creativity.update"
	OperationCreativityStatus       GatewayOperation = "creativity.status"
	OperationCreativitySearch       GatewayOperation = "creativity.search"
	OperationReportOfflineAccount   GatewayOperation = "report.offline.account"
	OperationReportOfflineCampaign  GatewayOperation = "report.offline.campaign"
	OperationReportOfflineUnit      GatewayOperation = "report.offline.unit"
	OperationReportOfflineCreative  GatewayOperation = "report.offline.creativity"
	OperationReportOfflineKeyword   GatewayOperation = "report.offline.keyword"
	OperationReportRealtimeAccount  GatewayOperation = "report.realtime.account"
	OperationReportRealtimeCampaign GatewayOperation = "report.realtime.campaign"
	OperationReportRealtimeUnit     GatewayOperation = "report.realtime.unit"
	OperationReportRealtimeCreative GatewayOperation = "report.realtime.creativity"
	OperationReportRealtimeKeyword  GatewayOperation = "report.realtime.keyword"
)

type GatewayOperationSpec struct {
	Operation     GatewayOperation `json:"operation"`
	Method        string           `json:"method"`
	Path          string           `json:"path"`
	RequiredScope string           `json:"required_scope"`
	Write         bool             `json:"write"`
	Activation    bool             `json:"activation"`
}

var gatewayOperations = map[GatewayOperation]GatewayOperationSpec{
	OperationWhiteList:              gatewaySpec(OperationWhiteList, http.MethodPost, "/api/open/jg/white/list", "account_manage", false),
	OperationBalance:                gatewaySpec(OperationBalance, http.MethodGet, "/api/open/jg/account/balance/info", "account_manage", false),
	OperationNoteList:               gatewaySpec(OperationNoteList, http.MethodPost, "/api/open/jg/note/list", "ad_query", false),
	OperationSPUList:                gatewaySpec(OperationSPUList, http.MethodPost, "/api/open/jg/spu/list", "ad_query", false),
	OperationQualificationList:      gatewaySpec(OperationQualificationList, http.MethodPost, "/api/open/jg/data/qual/info", "ad_query", false),
	OperationEventAssetList:         gatewaySpec(OperationEventAssetList, http.MethodPost, "/api/open/jg/data/event/asset/info", "ad_query", false),
	OperationTargetOptions:          gatewaySpec(OperationTargetOptions, http.MethodPost, "/api/open/jg/target/get_available_target_info", "ad_query", false),
	OperationAudienceEstimate:       gatewaySpec(OperationAudienceEstimate, http.MethodPost, "/api/open/jg/crowd/estimate", "ad_query", false),
	OperationKeywordRecommend:       gatewaySpec(OperationKeywordRecommend, http.MethodPost, "/api/open/jg/keyword/common/recommend", "ad_query", false),
	OperationKeywordWordBags:        gatewaySpec(OperationKeywordWordBags, http.MethodPost, "/api/open/jg/keyword/word/bag/list", "ad_query", false),
	OperationNegativeKeywordList:    gatewaySpec(OperationNegativeKeywordList, http.MethodPost, "/api/open/jg/negative/keyword/list", "ad_query", false),
	OperationNegativeKeywordAdd:     gatewaySpec(OperationNegativeKeywordAdd, http.MethodPost, "/api/open/jg/negative/keyword/batch/add", "ad_manage", true),
	OperationNegativeKeywordDelete:  gatewaySpec(OperationNegativeKeywordDelete, http.MethodPost, "/api/open/jg/negative/keyword/batch/delete", "ad_manage", true),
	OperationUnitKeywordAdd:         gatewaySpec(OperationUnitKeywordAdd, http.MethodPost, "/api/open/jg/unit/keyword/add", "ad_manage", true),
	OperationNameDuplicateCheck:     gatewaySpec(OperationNameDuplicateCheck, http.MethodPost, "/api/open/jg/data/check/name/dup", "ad_query", false),
	OperationCampaignCreate:         gatewaySpec(OperationCampaignCreate, http.MethodPost, "/api/open/jg/campaign/create", "ad_manage", true),
	OperationCampaignUpdate:         gatewaySpec(OperationCampaignUpdate, http.MethodPost, "/api/open/jg/campaign/update", "ad_manage", true),
	OperationCampaignStatus:         gatewayActivationSpec(OperationCampaignStatus, "/api/open/jg/campaign/status/update"),
	OperationCampaignList:           gatewaySpec(OperationCampaignList, http.MethodPost, "/api/open/jg/campaign/list", "ad_query", false),
	OperationUnitCreate:             gatewaySpec(OperationUnitCreate, http.MethodPost, "/api/open/jg/unit/create", "ad_manage", true),
	OperationUnitUpdate:             gatewaySpec(OperationUnitUpdate, http.MethodPost, "/api/open/jg/unit/update", "ad_manage", true),
	OperationUnitStatus:             gatewayActivationSpec(OperationUnitStatus, "/api/open/jg/unit/status/update"),
	OperationUnitBidUpdate:          gatewaySpec(OperationUnitBidUpdate, http.MethodPost, "/api/open/jg/unit/batch/update/bid", "ad_manage", true),
	OperationUnitList:               gatewaySpec(OperationUnitList, http.MethodPost, "/api/open/jg/unit/list", "ad_query", false),
	OperationCreativityCreate:       gatewaySpec(OperationCreativityCreate, http.MethodPost, "/api/open/jg/creativity/create", "ad_manage", true),
	OperationCreativityUpdate:       gatewaySpec(OperationCreativityUpdate, http.MethodPost, "/api/open/jg/creativity/update", "ad_manage", true),
	OperationCreativityStatus:       gatewayActivationSpec(OperationCreativityStatus, "/api/open/jg/creativity/status/update"),
	OperationCreativitySearch:       gatewaySpec(OperationCreativitySearch, http.MethodPost, "/api/open/jg/creativity/search", "ad_query", false),
	OperationReportOfflineAccount:   gatewayReportSpec(OperationReportOfflineAccount, "/api/open/jg/data/report/offline/account"),
	OperationReportOfflineCampaign:  gatewayReportSpec(OperationReportOfflineCampaign, "/api/open/jg/data/report/offline/campaign"),
	OperationReportOfflineUnit:      gatewayReportSpec(OperationReportOfflineUnit, "/api/open/jg/data/report/offline/unit"),
	OperationReportOfflineCreative:  gatewayReportSpec(OperationReportOfflineCreative, "/api/open/jg/data/report/offline/creativity"),
	OperationReportOfflineKeyword:   gatewayReportSpec(OperationReportOfflineKeyword, "/api/open/jg/data/report/offline/keyword"),
	OperationReportRealtimeAccount:  gatewayReportSpec(OperationReportRealtimeAccount, "/api/open/jg/data/report/realtime/account"),
	OperationReportRealtimeCampaign: gatewayReportSpec(OperationReportRealtimeCampaign, "/api/open/jg/data/report/realtime/campaign"),
	OperationReportRealtimeUnit:     gatewayReportSpec(OperationReportRealtimeUnit, "/api/open/jg/data/report/realtime/unit"),
	OperationReportRealtimeCreative: gatewayReportSpec(OperationReportRealtimeCreative, "/api/open/jg/data/report/realtime/creativity"),
	OperationReportRealtimeKeyword:  gatewayReportSpec(OperationReportRealtimeKeyword, "/api/open/jg/data/report/realtime/keyword"),
}

func gatewaySpec(operation GatewayOperation, method, path, scope string, write bool) GatewayOperationSpec {
	return GatewayOperationSpec{Operation: operation, Method: method, Path: path, RequiredScope: scope, Write: write}
}

func gatewayActivationSpec(operation GatewayOperation, path string) GatewayOperationSpec {
	value := gatewaySpec(operation, http.MethodPost, path, "ad_manage", true)
	value.Activation = true
	return value
}

func gatewayReportSpec(operation GatewayOperation, path string) GatewayOperationSpec {
	return gatewaySpec(operation, http.MethodPost, path, "report_service", false)
}

func GatewayOperationDetails() []GatewayOperationSpec {
	result := make([]GatewayOperationSpec, 0, len(gatewayOperations))
	for _, spec := range gatewayOperations {
		result = append(result, spec)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Operation < result[j].Operation })
	return result
}

func LookupGatewayOperation(operation GatewayOperation) (GatewayOperationSpec, bool) {
	spec, ok := gatewayOperations[operation]
	return spec, ok
}

type GatewayResult struct {
	Operation   GatewayOperation `json:"operation"`
	Data        json.RawMessage  `json:"data"`
	RequestID   string           `json:"request_id,omitempty"`
	RequestHash string           `json:"request_hash"`
	LatencyMS   int64            `json:"latency_ms"`
}

type gatewayEnvelope struct {
	Success   bool            `json:"success"`
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Msg       string          `json:"msg"`
	ErrorCode int             `json:"errorCode"`
	ErrorMsg  string          `json:"errorMsg"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func (client *Client) CallGateway(ctx context.Context, accessToken string, operation GatewayOperation, payload json.RawMessage) (GatewayResult, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return GatewayResult{}, errors.New("XHS Spotlight access token is required")
	}
	spec, ok := LookupGatewayOperation(operation)
	if !ok {
		return GatewayResult{}, fmt.Errorf("%w: operation %q is not allowlisted", ErrInvalidGatewayRequest, operation)
	}
	normalized, advertiserID, err := normalizeGatewayPayload(payload)
	if err != nil {
		return GatewayResult{}, err
	}
	if operation == OperationCampaignCreate {
		normalized, err = forceCampaignPaused(normalized)
		if err != nil {
			return GatewayResult{}, err
		}
	}
	if operation == OperationCampaignStatus {
		normalized, err = normalizeCampaignStatusPayload(normalized)
		if err != nil {
			return GatewayResult{}, err
		}
	}
	hash := sha256.Sum256(normalized)
	result := GatewayResult{Operation: operation, RequestHash: hex.EncodeToString(hash[:])}

	endpoint := client.baseURL + spec.Path
	var body io.Reader
	if spec.Method == http.MethodGet {
		parsed, parseErr := url.Parse(endpoint)
		if parseErr != nil {
			return GatewayResult{}, fmt.Errorf("build XHS Spotlight gateway URL: %w", parseErr)
		}
		query := parsed.Query()
		query.Set("advertiser_id", strconv.FormatInt(advertiserID, 10))
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	} else {
		body = bytes.NewReader(normalized)
	}
	request, err := http.NewRequestWithContext(ctx, spec.Method, endpoint, body)
	if err != nil {
		return GatewayResult{}, fmt.Errorf("create XHS Spotlight %s request: %w", operation, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Access-Token", accessToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	started := time.Now()
	response, err := client.httpClient.Do(request)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		return result, fmt.Errorf("request XHS Spotlight %s: %w", operation, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
	if err != nil {
		return result, fmt.Errorf("read XHS Spotlight %s response: %w", operation, err)
	}
	if len(data) > 32<<20 {
		return result, fmt.Errorf("XHS Spotlight %s response exceeds size limit", operation)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("XHS Spotlight %s: HTTP %d", operation, response.StatusCode)
	}
	var envelope gatewayEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return result, fmt.Errorf("decode XHS Spotlight %s response: %w", operation, err)
	}
	result.RequestID = envelope.RequestID
	if !envelope.Success || envelope.Code != 0 || envelope.ErrorCode != 0 {
		message := firstGatewayMessage(envelope.ErrorMsg, envelope.Message, envelope.Msg)
		return result, fmt.Errorf("XHS Spotlight %s API: code=%d error_code=%d message=%s request_id=%s", operation, envelope.Code, envelope.ErrorCode, message, envelope.RequestID)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		envelope.Data = json.RawMessage(`{}`)
	}
	result.Data = append(json.RawMessage(nil), envelope.Data...)
	return result, nil
}

func normalizeGatewayPayload(payload json.RawMessage) ([]byte, int64, error) {
	if len(payload) == 0 || len(payload) > MaxGatewayPayloadBytes {
		return nil, 0, fmt.Errorf("%w: payload size must be between 1 byte and %d bytes", ErrInvalidGatewayRequest, MaxGatewayPayloadBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, 0, fmt.Errorf("%w: payload must be a JSON object", ErrInvalidGatewayRequest)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, 0, fmt.Errorf("%w: payload must contain one JSON object", ErrInvalidGatewayRequest)
	}
	advertiserNumber, ok := object["advertiser_id"].(json.Number)
	if !ok {
		return nil, 0, fmt.Errorf("%w: advertiser_id is required", ErrInvalidGatewayRequest)
	}
	advertiserID, err := advertiserNumber.Int64()
	if err != nil || advertiserID <= 0 {
		return nil, 0, fmt.Errorf("%w: advertiser_id must be a positive integer", ErrInvalidGatewayRequest)
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: encode payload: %v", ErrInvalidGatewayRequest, err)
	}
	return normalized, advertiserID, nil
}

func GatewayAdvertiserID(payload json.RawMessage) (int64, error) {
	_, advertiserID, err := normalizeGatewayPayload(payload)
	return advertiserID, err
}

func normalizeCampaignStatusPayload(payload []byte) ([]byte, error) {
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, fmt.Errorf("%w: decode campaign status payload", ErrInvalidGatewayRequest)
	}
	rawIDs, ok := object["campaign_ids"].([]any)
	if !ok || len(rawIDs) == 0 {
		return nil, fmt.Errorf("%w: campaign_ids must contain at least one value", ErrInvalidGatewayRequest)
	}
	if len(rawIDs) > 20 {
		return nil, fmt.Errorf("%w: campaign_ids cannot contain more than 20 values", ErrInvalidGatewayRequest)
	}
	ids := make([]int64, 0, len(rawIDs))
	seen := make(map[int64]struct{}, len(rawIDs))
	for _, raw := range rawIDs {
		number, ok := raw.(json.Number)
		if !ok {
			return nil, fmt.Errorf("%w: campaign_ids must contain only integers", ErrInvalidGatewayRequest)
		}
		campaignID, err := number.Int64()
		if err != nil || campaignID <= 0 {
			return nil, fmt.Errorf("%w: campaign_ids must contain only positive values", ErrInvalidGatewayRequest)
		}
		if _, exists := seen[campaignID]; exists {
			return nil, fmt.Errorf("%w: campaign_ids cannot contain duplicates", ErrInvalidGatewayRequest)
		}
		seen[campaignID] = struct{}{}
		ids = append(ids, campaignID)
	}
	actionNumber, ok := object["action_type"].(json.Number)
	if !ok {
		return nil, fmt.Errorf("%w: action_type is required", ErrInvalidGatewayRequest)
	}
	actionType, err := actionNumber.Int64()
	if err != nil || actionType < 1 || actionType > 3 {
		return nil, fmt.Errorf("%w: action_type must be 1, 2, or 3", ErrInvalidGatewayRequest)
	}
	object["campaign_ids"] = ids
	object["action_type"] = actionType
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("%w: encode campaign status payload", ErrInvalidGatewayRequest)
	}
	return normalized, nil
}

func forceCampaignPaused(payload []byte) ([]byte, error) {
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("%w: decode campaign create payload", ErrInvalidGatewayRequest)
	}
	object["enable"] = 0
	return json.Marshal(object)
}

func firstGatewayMessage(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "unknown error"
}
