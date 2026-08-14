package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/delivery"
)

type handlerDeliveryGateway struct{}

func (handlerDeliveryGateway) Advertisers(context.Context) ([]delivery.Advertiser, error) {
	return []delivery.Advertiser{{ID: 1234, Name: "测试广告主"}, {ID: 9999, Name: "其他广告主"}}, nil
}

func (handlerDeliveryGateway) Capabilities(_ context.Context, advertiserID int64) (delivery.Capability, error) {
	return delivery.Capability{
		AdvertiserID: advertiserID, Authorized: true, AdvertiserAllowed: true,
		Scopes:             []string{"ad_manage", "ad_query", "report_service", "account_manage"},
		RequiredScopes:     []string{"ad_manage", "ad_query", "report_service", "account_manage"},
		MediaWritesEnabled: false, ContractVersion: delivery.MediaContractVersion,
		Operations: map[string]any{}, CheckedAt: time.Now(),
	}, nil
}
func (handlerDeliveryGateway) Call(context.Context, string, map[string]any) (delivery.GatewayResponse, error) {
	return delivery.GatewayResponse{}, nil
}

// Minimal Store implementation for HTTP authentication and algorithm routing tests.
type handlerStore struct{}

func (handlerStore) CreateDraft(context.Context, delivery.CreateDraftInput, delivery.Actor) (delivery.Draft, error) {
	return delivery.Draft{}, nil
}
func (handlerStore) UpdateDraft(context.Context, string, delivery.UpdateDraftInput, delivery.Actor) (delivery.Draft, error) {
	return delivery.Draft{}, nil
}
func (handlerStore) Draft(context.Context, string) (delivery.Draft, error) {
	return delivery.Draft{}, delivery.ErrNotFound
}
func (handlerStore) Drafts(context.Context, int64, int) ([]delivery.Draft, error) {
	return []delivery.Draft{}, nil
}
func (handlerStore) SaveRecommendation(context.Context, delivery.Recommendation) (delivery.Recommendation, error) {
	return delivery.Recommendation{}, nil
}
func (handlerStore) LatestRecommendation(context.Context, string, int) (delivery.Recommendation, error) {
	return delivery.Recommendation{}, delivery.ErrNotFound
}
func (handlerStore) SaveValidation(context.Context, delivery.Validation) (delivery.Validation, error) {
	return delivery.Validation{}, nil
}
func (handlerStore) LatestValidation(context.Context, string, int) (delivery.Validation, error) {
	return delivery.Validation{}, delivery.ErrNotFound
}
func (handlerStore) SaveApproval(context.Context, delivery.Approval) (delivery.Approval, error) {
	return delivery.Approval{}, nil
}
func (handlerStore) Approvals(context.Context, string, int) ([]delivery.Approval, error) {
	return nil, nil
}
func (handlerStore) CreatePublishJob(context.Context, delivery.PublishJob) (delivery.PublishJob, error) {
	return delivery.PublishJob{}, nil
}
func (handlerStore) PublishJobByIdempotency(context.Context, string) (delivery.PublishJob, error) {
	return delivery.PublishJob{}, delivery.ErrNotFound
}
func (handlerStore) PublishJob(context.Context, string) (delivery.PublishJob, error) {
	return delivery.PublishJob{}, delivery.ErrNotFound
}
func (handlerStore) PublishJobs(context.Context, string, int, int) ([]delivery.PublishJob, error) {
	return []delivery.PublishJob{}, nil
}
func (handlerStore) ClaimPublishJob(context.Context) (delivery.PublishJob, bool, error) {
	return delivery.PublishJob{}, false, nil
}
func (handlerStore) UpdatePublishJob(context.Context, delivery.PublishJob) error { return nil }
func (handlerStore) SaveMediaEntity(context.Context, delivery.MediaEntity) (delivery.MediaEntity, error) {
	return delivery.MediaEntity{}, nil
}
func (handlerStore) MediaEntity(context.Context, int64, string, int64) (delivery.MediaEntity, error) {
	return delivery.MediaEntity{}, delivery.ErrNotFound
}
func (handlerStore) MediaEntities(context.Context, string) ([]delivery.MediaEntity, error) {
	return []delivery.MediaEntity{}, nil
}
func (handlerStore) UpdateMediaEntityStatus(context.Context, string, string) error { return nil }
func (handlerStore) SaveAPIAttempt(context.Context, delivery.APIAttempt) error     { return nil }
func (handlerStore) SavePerformanceSnapshot(context.Context, delivery.PerformanceQuery, map[string]any, string) error {
	return nil
}
func (handlerStore) Assets(context.Context, delivery.AssetQuery) (delivery.Assets, error) {
	return delivery.Assets{}, nil
}
func (handlerStore) RecommendationCandidates(context.Context, []string) ([]delivery.CandidateNote, error) {
	return nil, nil
}
func (handlerStore) Audit(context.Context, delivery.Actor, string, string, string, int64, map[string]any) error {
	return nil
}

func TestDeliveryRoutesAllowDirectConsoleAndRejectInvalidExplicitKey(t *testing.T) {
	service, err := delivery.NewService(handlerStore{}, handlerDeliveryGateway{}, delivery.RuleSemanticAdvisor{}, delivery.HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := parseDeliveryCredentials(`[{"key":"viewer-key-12345678901234567890123456789","actor":"reader","role":"viewer","all_advertisers":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{delivery: service, deliveryCredentials: credentials, timeout: time.Second})

	request := httptest.NewRequest(http.MethodGet, "/v1/delivery/capabilities?advertiser_id=1234", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("direct console status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/delivery/capabilities?advertiser_id=1234", nil)
	request.Header.Set("X-Delivery-API-Key", "invalid-explicit-key")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid explicit key status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/delivery/session", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":"delivery-console"`) || !strings.Contains(recorder.Body.String(), `"role":"operator"`) || !strings.Contains(recorder.Body.String(), `"all_advertisers":true`) {
		t.Fatalf("direct session status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestParseDeliveryCredentialsAllowsEmptyOptionalConfig(t *testing.T) {
	credentials, err := parseDeliveryCredentials("")
	if err != nil || len(credentials) != 0 {
		t.Fatalf("empty credentials=%v error=%v", credentials, err)
	}
	_, err = parseDeliveryCredentials(`[{"key":"reserved-actor-key-123456789012345678901","actor":"delivery-console","role":"operator","all_advertisers":true}]`)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved direct actor error=%v", err)
	}
}

func TestDeliveryExplicitCredentialPreservesScope(t *testing.T) {
	service, err := delivery.NewService(handlerStore{}, handlerDeliveryGateway{}, delivery.RuleSemanticAdvisor{}, delivery.HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	const apiKey = "scoped-session-key-123456789012345678901234"
	credentials, err := parseDeliveryCredentials(`[{"key":"` + apiKey + `","actor":"browser-reader","role":"viewer","advertiser_ids":[1234]}]`)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{delivery: service, deliveryCredentials: credentials, timeout: time.Second})
	request := httptest.NewRequest(http.MethodGet, "/v1/delivery/session", nil)
	request.Header.Set("X-Delivery-API-Key", apiKey)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("explicit identity status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"role":"viewer"`) || !strings.Contains(recorder.Body.String(), `"advertiser_id":1234`) || strings.Contains(recorder.Body.String(), `"advertiser_id":9999`) {
		t.Fatalf("explicit identity did not preserve advertiser scope: %s", recorder.Body.String())
	}
}

func TestDeliveryOpenAPIIsPublicAndDocumentsEveryMountedRoute(t *testing.T) {
	service, err := delivery.NewService(handlerStore{}, handlerDeliveryGateway{}, delivery.RuleSemanticAdvisor{}, delivery.HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{delivery: service, timeout: time.Second})
	request := httptest.NewRequest(http.MethodGet, "/v1/delivery/openapi.json", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("openapi status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	var contract struct {
		OpenAPI string `json:"openapi"`
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &contract); err != nil {
		t.Fatal(err)
	}
	if contract.OpenAPI != "3.1.0" {
		t.Fatalf("openapi version = %q", contract.OpenAPI)
	}
	if len(contract.Servers) != 2 || contract.Servers[0].URL != "https://pangutech.online/paipai/api" || contract.Servers[1].URL != "http://127.0.0.1:18081/v1" {
		t.Fatalf("unexpected OpenAPI servers: %+v", contract.Servers)
	}
	for _, path := range []string{
		"/delivery", "/delivery/capabilities", "/delivery/assets", "/delivery/drafts", "/delivery/drafts/{draft_id}",
		"/delivery/drafts/{draft_id}/publish", "/delivery/jobs/{job_id}",
		"/delivery/entities/{entity_type}/{media_id}/status", "/delivery/performance",
		"/delivery/intelligence/bandit-shadow",
	} {
		if _, ok := contract.Paths[path]; !ok {
			t.Errorf("OpenAPI is missing %s", path)
		}
	}
}

func TestDeliveryAlgorithmRoutesStayNonExecutable(t *testing.T) {
	service, err := delivery.NewService(handlerStore{}, handlerDeliveryGateway{}, delivery.RuleSemanticAdvisor{}, delivery.HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := parseDeliveryCredentials(`[{"key":"analyst-key-1234567890123456789012345678","actor":"analyst","role":"analyst","all_advertisers":true},{"key":"viewer-key-12345678901234567890123456789","actor":"viewer","role":"viewer","all_advertisers":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{delivery: service, deliveryCredentials: credentials, timeout: time.Second})
	body := `{"total_fen":20000,"arms":[{"key":"a","min_fen":10000,"max_fen":20000,"expected_value":1}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/delivery/intelligence/optimize-budget", strings.NewReader(body))
	request.Header.Set("X-Delivery-API-Key", "analyst-key-1234567890123456789012345678")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"executable":false`) {
		t.Fatalf("optimizer status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/delivery/intelligence/optimize-budget", strings.NewReader(body))
	request.Header.Set("X-Delivery-API-Key", "viewer-key-12345678901234567890123456789")
	request.Header.Set("X-Delivery-Actor", "forged-operator")
	request.Header.Set("X-Delivery-Role", "operator")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("viewer optimizer status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDeliveryCredentialEnforcesAdvertiserScope(t *testing.T) {
	service, err := delivery.NewService(handlerStore{}, handlerDeliveryGateway{}, delivery.RuleSemanticAdvisor{}, delivery.HeuristicRanker{}, false)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := parseDeliveryCredentials(`[{"key":"scoped-key-12345678901234567890123456789","actor":"scoped-reader","role":"viewer","advertiser_ids":[1234]}]`)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{delivery: service, deliveryCredentials: credentials, timeout: time.Second})
	for _, test := range []struct {
		advertiserID int64
		want         int
	}{{1234, http.StatusOK}, {9999, http.StatusForbidden}} {
		request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/delivery/capabilities?advertiser_id=%d", test.advertiserID), nil)
		request.Header.Set("X-Delivery-API-Key", "scoped-key-12345678901234567890123456789")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != test.want {
			t.Errorf("advertiser %d status=%d body=%s", test.advertiserID, recorder.Code, recorder.Body.String())
		}
	}
}
