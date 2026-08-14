package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/xhs"
	"paipai-red-campaign-manager/internal/xhssync"
)

type authServer struct {
	manager            *xhs.TokenManager
	requestTimeout     time.Duration
	syncContext        context.Context
	syncService        *xhssync.Service
	internalAPIKey     string
	mediaWritesEnabled bool
}

type authorizeRequest struct {
	AuthCode string `json:"auth_code"`
}

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func newAuthHandler(manager *xhs.TokenManager, requestTimeout time.Duration, syncOptions ...func(*authServer)) http.Handler {
	server := &authServer{manager: manager, requestTimeout: requestTimeout, syncContext: context.Background()}
	for _, option := range syncOptions {
		option(server)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.health)
	mux.HandleFunc("/readyz", server.ready)
	mux.HandleFunc("/v1/oauth/status", server.status)
	mux.HandleFunc("/v1/oauth/authorize", server.authorize)
	mux.HandleFunc("/v1/oauth/refresh", server.refresh)
	mux.HandleFunc("/v1/campaigns/list", server.listCampaigns)
	mux.HandleFunc("/v1/campaigns/all", server.listAllCampaigns)
	mux.HandleFunc("/v1/units/list", server.listUnits)
	mux.HandleFunc("/v1/units/all", server.listAllUnits)
	mux.HandleFunc("/v1/creativities/list", server.listCreativities)
	mux.HandleFunc("/v1/creativities/all", server.listAllCreativities)
	mux.HandleFunc("/v1/gateway/operations", server.gatewayOperationList)
	mux.HandleFunc("/v1/gateway/call", server.gatewayCall)
	mux.HandleFunc("/v1/sync/status", server.syncStatus)
	mux.HandleFunc("/v1/sync/campaigns", server.syncCampaigns)
	mux.HandleFunc("/v1/sync/units", server.syncUnits)
	mux.HandleFunc("/v1/sync/creativities", server.syncCreativities)
	return noStoreHeaders(mux)
}

func withGatewayPolicy(internalAPIKey string, mediaWritesEnabled bool) func(*authServer) {
	return func(server *authServer) {
		server.internalAPIKey = strings.TrimSpace(internalAPIKey)
		server.mediaWritesEnabled = mediaWritesEnabled
	}
}

func withSyncService(ctx context.Context, service *xhssync.Service) func(*authServer) {
	return func(server *authServer) {
		server.syncContext = ctx
		server.syncService = service
	}
}

func (server *authServer) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]string{"status": "ok"}})
}

func (server *authServer) ready(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	status := server.manager.Status()
	code := http.StatusOK
	if !status.AccessTokenValid {
		code = http.StatusServiceUnavailable
	}
	writeJSON(writer, code, apiResponse{Success: status.AccessTokenValid, Data: status})
}

func (server *authServer) status(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: server.manager.Status()})
}

func (server *authServer) authorize(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload authorizeRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return
	}
	payload.AuthCode = strings.TrimSpace(payload.AuthCode)
	if payload.AuthCode == "" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "auth_code is required"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	if _, err := server.manager.Authorize(ctx, payload.AuthCode); err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: server.manager.Status()})
}

func (server *authServer) refresh(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	if _, err := server.manager.Refresh(ctx); err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, xhs.ErrNotAuthorized) {
			code = http.StatusConflict
		}
		writeJSON(writer, code, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: server.manager.Status()})
}

func (server *authServer) listCampaigns(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.CampaignListRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListCampaigns(ctx, payload)
	if err != nil {
		writeCampaignError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

func (server *authServer) listAllCampaigns(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.CampaignListRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListAllCampaigns(ctx, payload)
	if err != nil {
		writeCampaignError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

func (server *authServer) listUnits(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.UnitListRequest
	if !decodeListRequest(writer, request, &payload) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListUnits(ctx, payload)
	if err != nil {
		writeJGListError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

func (server *authServer) listAllUnits(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.UnitListRequest
	if !decodeListRequest(writer, request, &payload) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListAllUnits(ctx, payload)
	if err != nil {
		writeJGListError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

func (server *authServer) listCreativities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.CreativityListRequest
	if !decodeListRequest(writer, request, &payload) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListCreativities(ctx, payload)
	if err != nil {
		writeJGListError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

func (server *authServer) listAllCreativities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload xhs.CreativityListRequest
	if !decodeListRequest(writer, request, &payload) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	data, err := server.manager.ListAllCreativities(ctx, payload)
	if err != nil {
		writeJGListError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: data})
}

type gatewayCallRequest struct {
	Operation xhs.GatewayOperation `json:"operation"`
	Payload   json.RawMessage      `json:"payload"`
}

func (server *authServer) gatewayOperationList(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	if !server.authorizeInternal(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]interface{}{
		"contract_version":     "xhs-jg/2026-05-candidate",
		"media_writes_enabled": server.mediaWritesEnabled,
		"operations":           xhs.GatewayOperationDetails(),
	}})
}

func (server *authServer) gatewayCall(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if !server.authorizeInternal(writer, request) {
		return
	}
	var payload gatewayCallRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, xhs.MaxGatewayPayloadBytes+64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "request body must contain one JSON object"})
		return
	}
	spec, ok := xhs.LookupGatewayOperation(payload.Operation)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "gateway operation is not allowlisted"})
		return
	}
	if spec.Write && !server.mediaWritesEnabled {
		writeJSON(writer, http.StatusLocked, apiResponse{Success: false, Error: "XHS Spotlight media writes are disabled"})
		return
	}
	advertiserID, err := xhs.GatewayAdvertiserID(payload.Payload)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: err.Error()})
		return
	}
	status := server.manager.Status()
	if !status.Authorized || !status.AccessTokenValid {
		writeJSON(writer, http.StatusConflict, apiResponse{Success: false, Error: xhs.ErrNotAuthorized.Error()})
		return
	}
	if !authorizedAdvertiser(status, advertiserID) {
		writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "advertiser is not present in the OAuth authorization"})
		return
	}
	if !scopeGranted(status.Scope, spec.RequiredScope) {
		writeJSON(writer, http.StatusForbidden, apiResponse{Success: false, Error: "required OAuth scope is not granted"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	result, err := server.manager.CallGateway(ctx, payload.Operation, payload.Payload)
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, xhs.ErrInvalidGatewayRequest) {
			code = http.StatusBadRequest
		} else if errors.Is(err, xhs.ErrNotAuthorized) {
			code = http.StatusConflict
		}
		writeJSON(writer, code, apiResponse{Success: false, Data: result, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *authServer) authorizeInternal(writer http.ResponseWriter, request *http.Request) bool {
	if server.internalAPIKey == "" {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "internal gateway authentication is not configured"})
		return false
	}
	provided := strings.TrimSpace(request.Header.Get("X-Internal-API-Key"))
	expectedHash := sha256.Sum256([]byte(server.internalAPIKey))
	providedHash := sha256.Sum256([]byte(provided))
	if provided == "" || subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
		writeJSON(writer, http.StatusUnauthorized, apiResponse{Success: false, Error: "invalid internal API key"})
		return false
	}
	return true
}

func authorizedAdvertiser(status xhs.ManagerStatus, advertiserID int64) bool {
	if status.AdvertiserID == advertiserID {
		return true
	}
	for _, advertiser := range status.ApprovalAdvertisers {
		if advertiser.ID == advertiserID {
			return true
		}
	}
	return false
}

func scopeGranted(raw, required string) bool {
	if required == "" {
		return true
	}
	var scopes []string
	if json.Unmarshal([]byte(raw), &scopes) != nil {
		scopes = strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ' ' || r == '[' || r == ']' || r == '"'
		})
	}
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == required {
			return true
		}
	}
	return false
}

type syncRequest struct {
	AdvertiserID int64  `json:"advertiser_id,omitempty"`
	Mode         string `json:"mode,omitempty"`
}

func (server *authServer) syncStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	if server.syncService == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "sync service is unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	status, err := server.syncService.Status(ctx)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: status})
}

func (server *authServer) syncCampaigns(writer http.ResponseWriter, request *http.Request) {
	server.triggerSync(writer, request, xhssync.TargetCampaigns, xhssync.ModeIncremental)
}

func (server *authServer) syncUnits(writer http.ResponseWriter, request *http.Request) {
	server.triggerSync(writer, request, xhssync.TargetUnits, xhssync.ModeIncremental)
}

func (server *authServer) syncCreativities(writer http.ResponseWriter, request *http.Request) {
	server.triggerSync(writer, request, xhssync.TargetCreativities, xhssync.ModeFull)
}

func (server *authServer) triggerSync(writer http.ResponseWriter, request *http.Request, target xhssync.Target, defaultMode xhssync.Mode) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if server.syncService == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "sync service is unavailable"})
		return
	}
	var payload syncRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return
	}
	mode := defaultMode
	if value := strings.TrimSpace(payload.Mode); value != "" {
		mode = xhssync.Mode(value)
	}
	if payload.AdvertiserID < 0 || mode != xhssync.ModeIncremental && mode != xhssync.ModeFull ||
		target == xhssync.TargetCreativities && mode != xhssync.ModeFull {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid sync target, mode, or advertiser_id"})
		return
	}
	run, err := server.syncService.Trigger(server.syncContext, target, mode, "api", payload.AdvertiserID)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, xhssync.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusAccepted, apiResponse{Success: true, Data: run})
}

func decodeListRequest(writer http.ResponseWriter, request *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return false
	}
	return true
}

func writeCampaignError(writer http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if errors.Is(err, xhs.ErrInvalidCampaignRequest) {
		status = http.StatusBadRequest
	} else if errors.Is(err, xhs.ErrNotAuthorized) {
		status = http.StatusConflict
	}
	writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
}

func writeJGListError(writer http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if errors.Is(err, xhs.ErrInvalidUnitRequest) || errors.Is(err, xhs.ErrInvalidCreativityRequest) {
		status = http.StatusBadRequest
	} else if errors.Is(err, xhs.ErrNotAuthorized) {
		status = http.StatusConflict
	}
	writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
}

func noStoreHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}

func methodNotAllowed(writer http.ResponseWriter, allowed string) {
	writer.Header().Set("Allow", allowed)
	writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
}

func writeJSON(writer http.ResponseWriter, status int, value apiResponse) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
