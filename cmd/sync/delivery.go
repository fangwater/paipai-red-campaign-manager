package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/delivery"
)

const deliveryBodyLimit = 2 << 20

const deliveryDirectActorID = "delivery-console"

type deliveryToolRequest struct {
	AdvertiserID int64          `json:"advertiser_id"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type deliveryNegativeKeywordRequest struct {
	AdvertiserID int64          `json:"advertiser_id"`
	Action       string         `json:"action"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type deliveryEntityStatusRequest struct {
	AdvertiserID int64  `json:"advertiser_id"`
	Status       string `json:"status"`
}

type deliveryCredentialConfig struct {
	Key            string  `json:"key"`
	Actor          string  `json:"actor"`
	Role           string  `json:"role"`
	AdvertiserIDs  []int64 `json:"advertiser_ids,omitempty"`
	AllAdvertisers bool    `json:"all_advertisers,omitempty"`
}

type deliveryCredential struct {
	keyHash        [sha256.Size]byte
	actor          delivery.Actor
	advertiserIDs  map[int64]struct{}
	allAdvertisers bool
}

type deliverySessionView struct {
	Actor          delivery.Actor        `json:"actor"`
	Advertisers    []delivery.Advertiser `json:"advertisers"`
	AllAdvertisers bool                  `json:"all_advertisers"`
}

func parseDeliveryCredentials(raw string) ([]deliveryCredential, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var configs []deliveryCredentialConfig
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configs); err != nil {
		return nil, fmt.Errorf("DELIVERY_API_CREDENTIALS_JSON must be a JSON array: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("DELIVERY_API_CREDENTIALS_JSON must contain exactly one JSON array")
	}
	validRoles := map[string]bool{
		"viewer": true, "analyst": true, "operator": true, "budget_owner": true, "admin": true,
	}
	seenKeys := make(map[[sha256.Size]byte]bool, len(configs))
	seenActors := make(map[string]bool, len(configs))
	credentials := make([]deliveryCredential, 0, len(configs))
	for index, config := range configs {
		config.Key = strings.TrimSpace(config.Key)
		config.Actor = strings.TrimSpace(config.Actor)
		config.Role = strings.TrimSpace(config.Role)
		if len(config.Key) < 32 {
			return nil, fmt.Errorf("credential %d key must contain at least 32 characters", index)
		}
		if config.Actor == "" || len(config.Actor) > 120 {
			return nil, fmt.Errorf("credential %d actor must contain 1 to 120 characters", index)
		}
		if config.Actor == deliveryDirectActorID {
			return nil, fmt.Errorf("credential %d actor %q is reserved for direct console access", index, config.Actor)
		}
		if !validRoles[config.Role] {
			return nil, fmt.Errorf("credential %d role is invalid", index)
		}
		if config.AllAdvertisers && len(config.AdvertiserIDs) > 0 {
			return nil, fmt.Errorf("credential %d cannot combine all_advertisers with advertiser_ids", index)
		}
		if !config.AllAdvertisers && len(config.AdvertiserIDs) == 0 {
			return nil, fmt.Errorf("credential %d must declare advertiser_ids or all_advertisers", index)
		}
		if len(config.AdvertiserIDs) > 500 {
			return nil, fmt.Errorf("credential %d advertiser_ids cannot contain more than 500 values", index)
		}
		advertiserIDs := make(map[int64]struct{}, len(config.AdvertiserIDs))
		for _, advertiserID := range config.AdvertiserIDs {
			if advertiserID <= 0 {
				return nil, fmt.Errorf("credential %d advertiser_ids must be positive", index)
			}
			if _, exists := advertiserIDs[advertiserID]; exists {
				return nil, fmt.Errorf("credential %d repeats advertiser_id %d", index, advertiserID)
			}
			advertiserIDs[advertiserID] = struct{}{}
		}
		hash := sha256.Sum256([]byte(config.Key))
		if seenKeys[hash] {
			return nil, fmt.Errorf("credential %d reuses a key", index)
		}
		if seenActors[config.Actor] {
			return nil, fmt.Errorf("credential %d reuses an actor identity", index)
		}
		seenKeys[hash] = true
		seenActors[config.Actor] = true
		credentials = append(credentials, deliveryCredential{
			keyHash: hash, actor: delivery.Actor{ID: config.Actor, Role: config.Role},
			advertiserIDs: advertiserIDs, allAdvertisers: config.AllAdvertisers,
		})
	}
	return credentials, nil
}

func (server *apiServer) deliveryOverview(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]any{
		"api_version":      "delivery/v1",
		"openapi":          "/paipai/api/delivery/openapi.json",
		"internal_openapi": "/v1/delivery/openapi.json",
		"contract_version": delivery.MediaContractVersion,
		"rules_version":    delivery.RulesVersion,
		"safety": map[string]any{
			"new_campaign_status": "paused",
			"double_approval":     true,
			"idempotent_publish":  true,
			"llm_can_execute":     false,
			"bandit_shadow_only":  true,
		},
		"endpoints": []map[string]string{
			{"method": "GET", "path": "/v1/delivery/session", "domain": "控制台直通上下文与广告主发现"},
			{"method": "GET", "path": "/v1/delivery/capabilities", "domain": "OAuth、广告主与上游能力"},
			{"method": "GET", "path": "/v1/delivery/assets", "domain": "本地稿件候选"},
			{"method": "POST", "path": "/v1/delivery/assets/platform", "domain": "平台笔记、SPU、资质、事件资产"},
			{"method": "POST", "path": "/v1/delivery/target-options", "domain": "实时可用定向"},
			{"method": "POST", "path": "/v1/delivery/keyword-candidates", "domain": "关键词推荐与词包"},
			{"method": "POST", "path": "/v1/delivery/negative-keywords", "domain": "否定词查询（变更仅经审批发布）"},
			{"method": "POST", "path": "/v1/delivery/audience-estimates", "domain": "人群规模预估"},
			{"method": "POST", "path": "/v1/delivery/campaigns/query", "domain": "计划查询"},
			{"method": "POST", "path": "/v1/delivery/units/query", "domain": "单元查询"},
			{"method": "POST", "path": "/v1/delivery/creativities/query", "domain": "创意查询"},
			{"method": "GET|POST", "path": "/v1/delivery/drafts", "domain": "版本化草稿"},
			{"method": "GET|PUT", "path": "/v1/delivery/drafts/{id}", "domain": "草稿详情与修订"},
			{"method": "GET", "path": "/v1/delivery/drafts/{id}/workflow", "domain": "当前版本工作流状态"},
			{"method": "POST", "path": "/v1/delivery/drafts/{id}/recommendations", "domain": "LLM 与排序建议"},
			{"method": "POST", "path": "/v1/delivery/drafts/{id}/validate", "domain": "确定性与平台校验"},
			{"method": "POST", "path": "/v1/delivery/drafts/{id}/approve", "domain": "运营与预算双审批"},
			{"method": "POST", "path": "/v1/delivery/drafts/{id}/publish", "domain": "计划、单元、关键词、创意异步 Saga"},
			{"method": "GET", "path": "/v1/delivery/jobs/{id}", "domain": "发布作业与恢复点"},
			{"method": "POST", "path": "/v1/delivery/entities/{type}/{id}/status", "domain": "人工启停"},
			{"method": "GET|POST", "path": "/v1/delivery/performance", "domain": "账户至关键词五层报表"},
			{"method": "GET", "path": "/v1/delivery/intelligence/capabilities", "domain": "算法职责边界"},
			{"method": "POST", "path": "/v1/delivery/intelligence/bayesian", "domain": "贝叶斯后验"},
			{"method": "POST", "path": "/v1/delivery/intelligence/optimize-budget", "domain": "约束预算建议"},
			{"method": "POST", "path": "/v1/delivery/intelligence/bandit-shadow", "domain": "Bandit 影子建议"},
		},
	}})
}

func (server *apiServer) deliveryOpenAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(delivery.OpenAPIJSON)
}

func (server *apiServer) deliverySession(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	credential, ok := server.deliveryCredentialForRequest(request)
	if !ok {
		writeJSON(writer, http.StatusUnauthorized, apiResponse{Success: false, Error: "invalid delivery credentials"})
		return
	}
	view, err := server.deliverySessionView(request.Context(), credential)
	server.writeDeliveryResult(writer, view, err)
}

func (server *apiServer) deliverySessionView(ctx context.Context, credential deliveryCredential) (deliverySessionView, error) {
	if server.delivery == nil {
		return deliverySessionView{}, errors.New("delivery service is not configured")
	}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	advertisers, err := server.delivery.Advertisers(requestContext)
	if err != nil {
		return deliverySessionView{}, err
	}
	visible := make([]delivery.Advertiser, 0, len(advertisers))
	for _, advertiser := range advertisers {
		if credential.allAdvertisers {
			visible = append(visible, advertiser)
			continue
		}
		if _, allowed := credential.advertiserIDs[advertiser.ID]; allowed {
			visible = append(visible, advertiser)
		}
	}
	return deliverySessionView{
		Actor: credential.actor, Advertisers: visible,
		AllAdvertisers: credential.allAdvertisers,
	}, nil
}

func (server *apiServer) deliveryCapabilities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	advertiserID, ok := deliveryQueryInt64(writer, request, "advertiser_id", true)
	if !ok {
		return
	}
	if _, ok := server.deliveryActorForAdvertiser(writer, request, advertiserID, "viewer", "analyst", "operator", "budget_owner", "admin"); !ok {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.Capabilities(ctx, advertiserID)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryAssets(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	advertiserID, ok := deliveryQueryInt64(writer, request, "advertiser_id", true)
	if !ok {
		return
	}
	if _, ok := server.deliveryActorForAdvertiser(writer, request, advertiserID, "viewer", "analyst", "operator", "budget_owner", "admin"); !ok {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "limit must be between 1 and 200"})
			return
		}
		limit = value
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.Assets(ctx, delivery.AssetQuery{
		AdvertiserID: advertiserID, Search: request.URL.Query().Get("search"), Limit: limit,
	})
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryPlatformAssets(writer http.ResponseWriter, request *http.Request) {
	server.deliveryTool(writer, request, map[string]string{
		"notes": "asset.note_list", "spus": "asset.spu_list",
		"qualifications": "asset.qualification_list", "events": "asset.event_list",
	}, "asset_type")
}

func (server *apiServer) deliveryTargetOptions(writer http.ResponseWriter, request *http.Request) {
	server.deliveryFixedTool(writer, request, "target.options")
}

func (server *apiServer) deliveryAudienceEstimates(writer http.ResponseWriter, request *http.Request) {
	server.deliveryFixedTool(writer, request, "target.audience_estimate")
}

func (server *apiServer) deliveryCampaignQuery(writer http.ResponseWriter, request *http.Request) {
	server.deliveryFixedTool(writer, request, "campaign.list")
}

func (server *apiServer) deliveryUnitQuery(writer http.ResponseWriter, request *http.Request) {
	server.deliveryFixedTool(writer, request, "unit.list")
}

func (server *apiServer) deliveryCreativityQuery(writer http.ResponseWriter, request *http.Request) {
	server.deliveryFixedTool(writer, request, "creativity.search")
}

func (server *apiServer) deliveryKeywordCandidates(writer http.ResponseWriter, request *http.Request) {
	server.deliveryTool(writer, request, map[string]string{
		"recommend": "keyword.recommend", "word_bags": "keyword.word_bags",
	}, "source")
}

func (server *apiServer) deliveryNegativeKeywords(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	actor, authorized := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !authorized {
		return
	}
	var input deliveryNegativeKeywordRequest
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	operation, ok := map[string]string{
		"list": "keyword.negative_list", "add": "keyword.negative_add", "delete": "keyword.negative_delete",
	}[strings.TrimSpace(input.Action)]
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "action must be list, add, or delete"})
		return
	}
	if input.Action != "list" {
		writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "negative keyword mutations are only available through an approved draft publish"})
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, input.AdvertiserID) {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.PlatformTool(ctx, operation, input.AdvertiserID, input.Payload, actor)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryDrafts(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
		if !ok {
			return
		}
		advertiserID, ok := deliveryQueryInt64(writer, request, "advertiser_id", false)
		if !ok {
			return
		}
		if advertiserID == 0 {
			if !server.deliveryAllAdvertisersAllowed(writer, actor) {
				return
			}
		} else if !server.deliveryAdvertiserAllowed(writer, actor, advertiserID) {
			return
		}
		limit := 50
		if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 200 {
				writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "limit must be between 1 and 200"})
				return
			}
			limit = value
		}
		ctx, cancel := server.deliveryContext(request)
		defer cancel()
		result, err := server.delivery.Drafts(ctx, advertiserID, limit)
		server.writeDeliveryResult(writer, map[string]any{"items": result, "count": len(result)}, err)
	case http.MethodPost:
		actor, ok := server.deliveryActor(writer, request, "operator", "admin")
		if !ok {
			return
		}
		var input delivery.CreateDraftInput
		if !decodeDeliveryJSON(writer, request, &input, false) {
			return
		}
		if !server.deliveryAdvertiserAllowed(writer, actor, input.Spec.AdvertiserID) {
			return
		}
		ctx, cancel := server.deliveryContext(request)
		defer cancel()
		result, err := server.delivery.CreateDraft(ctx, input, actor)
		if err == nil {
			writeJSON(writer, http.StatusCreated, apiResponse{Success: true, Data: result})
			return
		}
		server.writeDeliveryResult(writer, result, err)
	default:
		writer.Header().Set("Allow", "GET, POST")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
	}
}

func (server *apiServer) deliveryDraft(writer http.ResponseWriter, request *http.Request) {
	path := strings.Trim(strings.TrimPrefix(request.URL.Path, "/v1/delivery/drafts/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" || !strings.HasPrefix(parts[0], "drf_") {
		http.NotFound(writer, request)
		return
	}
	draftID := parts[0]
	if len(parts) == 1 {
		server.deliveryDraftResource(writer, request, draftID)
		return
	}
	if len(parts) == 2 && parts[1] == "workflow" {
		server.deliveryDraftWorkflow(writer, request, draftID)
		return
	}
	if len(parts) != 2 || request.Method != http.MethodPost {
		http.NotFound(writer, request)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	draft, err := server.delivery.Draft(ctx, draftID)
	if err != nil {
		server.writeDeliveryResult(writer, nil, err)
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, draft.AdvertiserID) {
		return
	}
	switch parts[1] {
	case "recommendations":
		if actor.Role == "budget_owner" {
			server.writeDeliveryResult(writer, nil, delivery.ErrForbidden)
			return
		}
		if !decodeDeliveryJSON(writer, request, &struct{}{}, true) {
			return
		}
		result, err := server.delivery.Recommend(ctx, draftID, actor)
		server.writeDeliveryResult(writer, result, err)
	case "validate":
		if actor.Role == "budget_owner" {
			server.writeDeliveryResult(writer, nil, delivery.ErrForbidden)
			return
		}
		if !decodeDeliveryJSON(writer, request, &struct{}{}, true) {
			return
		}
		result, err := server.delivery.Validate(ctx, draftID, actor)
		server.writeDeliveryResult(writer, result, err)
	case "approve":
		var input delivery.ApprovalInput
		if !decodeDeliveryJSON(writer, request, &input, false) {
			return
		}
		result, err := server.delivery.Approve(ctx, draftID, input, actor)
		server.writeDeliveryResult(writer, result, err)
	case "publish":
		if actor.Role == "budget_owner" {
			server.writeDeliveryResult(writer, nil, delivery.ErrForbidden)
			return
		}
		var input delivery.PublishInput
		if !decodeDeliveryJSON(writer, request, &input, false) {
			return
		}
		result, err := server.delivery.Publish(ctx, draftID, input, actor)
		if err == nil {
			status := http.StatusOK
			if result.Mode == "execute" && result.Status == "queued" {
				status = http.StatusAccepted
			}
			writeJSON(writer, status, apiResponse{Success: true, Data: result})
			return
		}
		server.writeDeliveryResult(writer, result, err)
	default:
		http.NotFound(writer, request)
	}
}

func (server *apiServer) deliveryDraftWorkflow(writer http.ResponseWriter, request *http.Request, draftID string) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.Workflow(ctx, draftID)
	if err == nil && !server.deliveryAdvertiserAllowed(writer, actor, result.Draft.AdvertiserID) {
		return
	}
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryDraftResource(writer http.ResponseWriter, request *http.Request, draftID string) {
	switch request.Method {
	case http.MethodGet:
		actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
		if !ok {
			return
		}
		ctx, cancel := server.deliveryContext(request)
		defer cancel()
		result, err := server.delivery.Draft(ctx, draftID)
		if err == nil && !server.deliveryAdvertiserAllowed(writer, actor, result.AdvertiserID) {
			return
		}
		server.writeDeliveryResult(writer, result, err)
	case http.MethodPut:
		actor, ok := server.deliveryActor(writer, request, "operator", "admin")
		if !ok {
			return
		}
		var input delivery.UpdateDraftInput
		if !decodeDeliveryJSON(writer, request, &input, false) {
			return
		}
		ctx, cancel := server.deliveryContext(request)
		defer cancel()
		current, loadErr := server.delivery.Draft(ctx, draftID)
		if loadErr != nil {
			server.writeDeliveryResult(writer, nil, loadErr)
			return
		}
		if !server.deliveryAdvertiserAllowed(writer, actor, current.AdvertiserID) || !server.deliveryAdvertiserAllowed(writer, actor, input.Spec.AdvertiserID) {
			return
		}
		result, err := server.delivery.UpdateDraft(ctx, draftID, input, actor)
		server.writeDeliveryResult(writer, result, err)
	default:
		writer.Header().Set("Allow", "GET, PUT")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
	}
}

func (server *apiServer) deliveryJob(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	jobID := strings.Trim(strings.TrimPrefix(request.URL.Path, "/v1/delivery/jobs/"), "/")
	if !strings.HasPrefix(jobID, "job_") || strings.Contains(jobID, "/") {
		http.NotFound(writer, request)
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.Job(ctx, jobID)
	if err == nil && !server.deliveryAdvertiserAllowed(writer, actor, result.AdvertiserID) {
		return
	}
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryEntity(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "operator", "admin")
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(request.URL.Path, "/v1/delivery/entities/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[2] != "status" {
		http.NotFound(writer, request)
		return
	}
	mediaID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || mediaID <= 0 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "media entity ID must be positive"})
		return
	}
	var input deliveryEntityStatusRequest
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, input.AdvertiserID) {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, callErr := server.delivery.UpdateEntityStatus(ctx, input.AdvertiserID, parts[0], mediaID, input.Status, actor)
	server.writeDeliveryResult(writer, result, callErr)
}

func (server *apiServer) deliveryPerformance(writer http.ResponseWriter, request *http.Request) {
	actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	if request.Method == http.MethodPost {
		var input delivery.PerformanceQuery
		if !decodeDeliveryJSON(writer, request, &input, false) {
			return
		}
		if !server.deliveryAdvertiserAllowed(writer, actor, input.AdvertiserID) {
			return
		}
		ctx, cancel := server.deliveryContext(request)
		defer cancel()
		result, err := server.delivery.Performance(ctx, input, actor)
		server.writeDeliveryResult(writer, result, err)
		return
	}
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", "GET, POST")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
		return
	}
	advertiserID, ok := deliveryQueryInt64(writer, request, "advertiser_id", true)
	if !ok {
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, advertiserID) {
		return
	}
	page, pageSize := 1, 100
	for name, target := range map[string]*int{"page": &page, "page_size": &pageSize} {
		if raw := strings.TrimSpace(request.URL.Query().Get(name)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value <= 0 {
				writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: name + " must be positive"})
				return
			}
			*target = value
		}
	}
	splitColumns := []string{}
	for _, value := range strings.Split(request.URL.Query().Get("split_columns"), ",") {
		if value = strings.TrimSpace(value); value != "" {
			splitColumns = append(splitColumns, value)
		}
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.Performance(ctx, delivery.PerformanceQuery{
		AdvertiserID: advertiserID, Level: request.URL.Query().Get("level"),
		Realtime: request.URL.Query().Get("realtime") == "true", StartDate: request.URL.Query().Get("start_date"),
		EndDate: request.URL.Query().Get("end_date"), Page: page, PageSize: pageSize, SplitColumns: splitColumns,
	}, actor)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryIntelligenceCapabilities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	if _, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin"); !ok {
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: server.delivery.IntelligenceCapabilities()})
}

func (server *apiServer) deliveryBayesian(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if _, ok := server.deliveryActor(writer, request, "analyst", "operator", "admin"); !ok {
		return
	}
	var input delivery.BayesianInput
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	result, err := delivery.BetaBinomialUpdate(input)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryOptimizeBudget(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if _, ok := server.deliveryActor(writer, request, "analyst", "operator", "admin"); !ok {
		return
	}
	var input delivery.BudgetOptimizationInput
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	result, err := delivery.OptimizeBudget(input)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryBanditShadow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if _, ok := server.deliveryActor(writer, request, "analyst", "operator", "admin"); !ok {
		return
	}
	var input delivery.BanditShadowInput
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	result, err := delivery.BanditShadowSuggestion(input)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryFixedTool(writer http.ResponseWriter, request *http.Request, operation string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	var input deliveryToolRequest
	if !decodeDeliveryJSON(writer, request, &input, false) {
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, input.AdvertiserID) {
		return
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.PlatformTool(ctx, operation, input.AdvertiserID, input.Payload, actor)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryTool(writer http.ResponseWriter, request *http.Request, operations map[string]string, discriminator string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	actor, ok := server.deliveryActor(writer, request, "viewer", "analyst", "operator", "budget_owner", "admin")
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if !decodeDeliveryJSON(writer, request, &raw, false) {
		return
	}
	var advertiserID int64
	if err := json.Unmarshal(raw["advertiser_id"], &advertiserID); err != nil || advertiserID <= 0 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "advertiser_id must be positive"})
		return
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, advertiserID) {
		return
	}
	var selected string
	if err := json.Unmarshal(raw[discriminator], &selected); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: discriminator + " is required"})
		return
	}
	operation, exists := operations[selected]
	if !exists {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: discriminator + " is not supported"})
		return
	}
	payload := map[string]any{}
	if value := raw["payload"]; len(value) > 0 && string(value) != "null" {
		if err := json.Unmarshal(value, &payload); err != nil {
			writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "payload must be a JSON object"})
			return
		}
	}
	ctx, cancel := server.deliveryContext(request)
	defer cancel()
	result, err := server.delivery.PlatformTool(ctx, operation, advertiserID, payload, actor)
	server.writeDeliveryResult(writer, result, err)
}

func (server *apiServer) deliveryActor(writer http.ResponseWriter, request *http.Request, roles ...string) (delivery.Actor, bool) {
	if server.delivery == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "delivery service is not configured"})
		return delivery.Actor{}, false
	}
	credential, ok := server.deliveryCredentialForRequest(request)
	if !ok {
		writeJSON(writer, http.StatusUnauthorized, apiResponse{Success: false, Error: "invalid or expired delivery credentials"})
		return delivery.Actor{}, false
	}
	actor := credential.actor
	for _, role := range roles {
		if actor.Role == role {
			return actor, true
		}
	}
	writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "delivery role is not allowed for this operation"})
	return delivery.Actor{}, false
}

func (server *apiServer) deliveryCredentialForRequest(request *http.Request) (deliveryCredential, bool) {
	if provided := strings.TrimSpace(request.Header.Get("X-Delivery-API-Key")); provided != "" {
		credential, ok := server.deliveryCredentialByKey(provided)
		return credential, ok
	}
	return directDeliveryCredential(), true
}

func directDeliveryCredential() deliveryCredential {
	return deliveryCredential{
		actor:          delivery.Actor{ID: deliveryDirectActorID, Role: "operator"},
		allAdvertisers: true,
	}
}

func (server *apiServer) deliveryCredentialForActor(actor delivery.Actor) (deliveryCredential, bool) {
	direct := directDeliveryCredential()
	if actor.ID == direct.actor.ID && actor.Role == direct.actor.Role {
		return direct, true
	}
	for _, credential := range server.deliveryCredentials {
		if credential.actor.ID == actor.ID && credential.actor.Role == actor.Role {
			return credential, true
		}
	}
	return deliveryCredential{}, false
}

func (server *apiServer) deliveryCredentialByKey(provided string) (deliveryCredential, bool) {
	provided = strings.TrimSpace(provided)
	providedHash := sha256.Sum256([]byte(provided))
	matched := -1
	for index := range server.deliveryCredentials {
		if subtle.ConstantTimeCompare(server.deliveryCredentials[index].keyHash[:], providedHash[:]) == 1 {
			matched = index
		}
	}
	if provided == "" || matched < 0 {
		return deliveryCredential{}, false
	}
	return server.deliveryCredentials[matched], true
}

func (server *apiServer) deliveryActorForAdvertiser(writer http.ResponseWriter, request *http.Request, advertiserID int64, roles ...string) (delivery.Actor, bool) {
	actor, ok := server.deliveryActor(writer, request, roles...)
	if !ok {
		return delivery.Actor{}, false
	}
	if !server.deliveryAdvertiserAllowed(writer, actor, advertiserID) {
		return delivery.Actor{}, false
	}
	return actor, true
}

func (server *apiServer) deliveryAdvertiserAllowed(writer http.ResponseWriter, actor delivery.Actor, advertiserID int64) bool {
	credential, ok := server.deliveryCredentialForActor(actor)
	if !ok {
		writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "delivery credential scope is unavailable"})
		return false
	}
	if credential.allAdvertisers {
		return true
	}
	if _, allowed := credential.advertiserIDs[advertiserID]; allowed {
		return true
	}
	writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "delivery credential is not authorized for this advertiser"})
	return false
}

func (server *apiServer) deliveryAllAdvertisersAllowed(writer http.ResponseWriter, actor delivery.Actor) bool {
	credential, ok := server.deliveryCredentialForActor(actor)
	if ok && credential.allAdvertisers {
		return true
	}
	writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "advertiser_id is required for an advertiser-scoped credential"})
	return false
}

func (server *apiServer) deliveryContext(request *http.Request) (context.Context, context.CancelFunc) {
	timeout := server.timeout
	if timeout <= 0 || timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	return context.WithTimeout(request.Context(), timeout)
}

func (server *apiServer) writeDeliveryResult(writer http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: value})
		return
	}
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, delivery.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, delivery.ErrConflict), errors.Is(err, delivery.ErrApprovalStale), errors.Is(err, delivery.ErrApprovalRequired), errors.Is(err, delivery.ErrCapabilityExpired):
		status = http.StatusConflict
	case errors.Is(err, delivery.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, delivery.ErrWritesDisabled):
		status = http.StatusLocked
	case errors.Is(err, delivery.ErrValidation):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case strings.Contains(err.Error(), "XHS gateway") || strings.Contains(err.Error(), "XHS Spotlight"):
		status = http.StatusBadGateway
	}
	writeJSON(writer, status, apiResponse{Success: false, Data: value, Error: err.Error()})
}

func decodeDeliveryJSON(writer http.ResponseWriter, request *http.Request, target any, optional bool) bool {
	decoder := json.NewDecoder(io.LimitReader(request.Body, deliveryBodyLimit+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request: " + err.Error()})
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "request body must contain one JSON object"})
		return false
	}
	return true
}

func deliveryQueryInt64(writer http.ResponseWriter, request *http.Request, name string, required bool) (int64, bool) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" && !required {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: fmt.Sprintf("%s must be a positive integer", name)})
		return 0, false
	}
	return value, true
}
