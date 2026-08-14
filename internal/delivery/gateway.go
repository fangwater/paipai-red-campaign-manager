package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Gateway interface {
	Advertisers(context.Context) ([]Advertiser, error)
	Capabilities(context.Context, int64) (Capability, error)
	Call(context.Context, string, map[string]any) (GatewayResponse, error)
}

type GatewayResponse struct {
	Operation   string         `json:"operation"`
	Data        map[string]any `json:"data"`
	RequestID   string         `json:"request_id,omitempty"`
	RequestHash string         `json:"request_hash"`
	LatencyMS   int64          `json:"latency_ms"`
}

type HTTPGateway struct {
	baseURL        string
	internalAPIKey string
	httpClient     *http.Client
	now            func() time.Time
}

func NewHTTPGateway(baseURL, internalAPIKey string, httpClient *http.Client) (*HTTPGateway, error) {
	baseURL = strings.TrimSpace(baseURL)
	internalAPIKey = strings.TrimSpace(internalAPIKey)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("XHS auth daemon URL is invalid")
	}
	if parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return nil, errors.New("XHS auth daemon URL must be loopback HTTP")
	}
	if internalAPIKey == "" {
		return nil, errors.New("XHS internal API key is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}
	return &HTTPGateway{
		baseURL: strings.TrimRight(baseURL, "/"), internalAPIKey: internalAPIKey,
		httpClient: httpClient, now: time.Now,
	}, nil
}

type gatewayEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type oauthStatus struct {
	Authorized          bool         `json:"authorized"`
	AccessTokenValid    bool         `json:"access_token_valid"`
	Scope               string       `json:"scope"`
	AdvertiserID        int64        `json:"advertiser_id"`
	ApprovalAdvertisers []Advertiser `json:"approval_advertisers"`
}

type operationCatalog struct {
	ContractVersion    string `json:"contract_version"`
	MediaWritesEnabled bool   `json:"media_writes_enabled"`
	Operations         []struct {
		Operation     string `json:"operation"`
		RequiredScope string `json:"required_scope"`
		Write         bool   `json:"write"`
		Activation    bool   `json:"activation"`
	} `json:"operations"`
}

func (gateway *HTTPGateway) Advertisers(ctx context.Context) ([]Advertiser, error) {
	var status oauthStatus
	if err := gateway.request(ctx, http.MethodGet, "/v1/oauth/status", nil, false, &status); err != nil {
		return nil, err
	}
	if !status.Authorized || !status.AccessTokenValid {
		return []Advertiser{}, nil
	}
	result := make([]Advertiser, 0, len(status.ApprovalAdvertisers)+1)
	seen := make(map[int64]struct{}, len(status.ApprovalAdvertisers)+1)
	for _, advertiser := range status.ApprovalAdvertisers {
		if advertiser.ID <= 0 {
			continue
		}
		advertiser.Name = strings.TrimSpace(advertiser.Name)
		result = append(result, advertiser)
		seen[advertiser.ID] = struct{}{}
	}
	if status.AdvertiserID > 0 {
		if _, exists := seen[status.AdvertiserID]; !exists {
			result = append(result, Advertiser{ID: status.AdvertiserID})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Name == result[right].Name {
			return result[left].ID < result[right].ID
		}
		return result[left].Name < result[right].Name
	})
	return result, nil
}

func (gateway *HTTPGateway) Capabilities(ctx context.Context, advertiserID int64) (Capability, error) {
	if advertiserID <= 0 {
		return Capability{}, errors.New("advertiser_id must be positive")
	}
	var status oauthStatus
	if err := gateway.request(ctx, http.MethodGet, "/v1/oauth/status", nil, false, &status); err != nil {
		return Capability{}, err
	}
	var catalog operationCatalog
	if err := gateway.request(ctx, http.MethodGet, "/v1/gateway/operations", nil, true, &catalog); err != nil {
		return Capability{}, err
	}
	scopes := parseScopes(status.Scope)
	scopeSet := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = true
	}
	allowed := status.AdvertiserID == advertiserID
	for _, advertiser := range status.ApprovalAdvertisers {
		allowed = allowed || advertiser.ID == advertiserID
	}
	required := []string{"ad_manage", "ad_query", "report_service", "account_manage"}
	missing := make([]string, 0)
	for _, scope := range required {
		if !scopeSet[scope] {
			missing = append(missing, scope)
		}
	}
	operations := make(map[string]any, len(catalog.Operations))
	for _, operation := range catalog.Operations {
		operations[operation.Operation] = map[string]any{
			"required_scope": operation.RequiredScope,
			"scope_granted":  scopeSet[operation.RequiredScope],
			"write":          operation.Write,
			"activation":     operation.Activation,
			"enabled":        (!operation.Write || catalog.MediaWritesEnabled) && scopeSet[operation.RequiredScope],
		}
	}
	return Capability{
		AdvertiserID: advertiserID, Authorized: status.Authorized && status.AccessTokenValid,
		AdvertiserAllowed: allowed, Scopes: scopes, RequiredScopes: required, MissingScopes: missing,
		AdvertiserCount: len(status.ApprovalAdvertisers), MediaWritesEnabled: catalog.MediaWritesEnabled,
		ContractVersion: catalog.ContractVersion, Operations: operations, CheckedAt: gateway.now().UTC(),
	}, nil
}

func (gateway *HTTPGateway) Call(ctx context.Context, operation string, payload map[string]any) (GatewayResponse, error) {
	requestPayload := map[string]any{"operation": operation, "payload": payload}
	var response struct {
		Operation   string          `json:"operation"`
		Data        json.RawMessage `json:"data"`
		RequestID   string          `json:"request_id"`
		RequestHash string          `json:"request_hash"`
		LatencyMS   int64           `json:"latency_ms"`
	}
	if err := gateway.request(ctx, http.MethodPost, "/v1/gateway/call", requestPayload, true, &response); err != nil {
		return GatewayResponse{}, err
	}
	data := map[string]any{}
	if len(response.Data) > 0 && string(response.Data) != "null" {
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return GatewayResponse{}, fmt.Errorf("decode XHS gateway data: %w", err)
		}
	}
	return GatewayResponse{
		Operation: response.Operation, Data: data, RequestID: response.RequestID,
		RequestHash: response.RequestHash, LatencyMS: response.LatencyMS,
	}, nil
}

func (gateway *HTTPGateway) request(ctx context.Context, method, path string, payload any, internal bool, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode XHS gateway request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, gateway.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create XHS gateway request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if internal {
		request.Header.Set("X-Internal-API-Key", gateway.internalAPIKey)
	}
	response, err := gateway.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call XHS gateway: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil {
		return fmt.Errorf("read XHS gateway response: %w", err)
	}
	if len(data) > 4<<20 {
		return errors.New("XHS gateway response exceeds size limit")
	}
	var envelope gatewayEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode XHS gateway response: HTTP %d: %w", response.StatusCode, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !envelope.Success {
		message := strings.TrimSpace(envelope.Error)
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		if response.StatusCode == http.StatusLocked {
			return fmt.Errorf("%w: %s", ErrWritesDisabled, message)
		}
		if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("%w: %s", ErrForbidden, message)
		}
		return fmt.Errorf("XHS gateway HTTP %d: %s", response.StatusCode, message)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode XHS gateway payload: %w", err)
	}
	return nil
}

func parseScopes(raw string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil {
		values = strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ' ' || r == '[' || r == ']' || r == '"'
		})
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
