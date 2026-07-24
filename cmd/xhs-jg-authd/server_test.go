package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/xhs"
)

func TestAuthHandlerAuthorizationLifecycle(t *testing.T) {
	var tokenCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenCalls++
		writer.Header().Set("Content-Type", "application/json")
		accessToken := "access-initial"
		refreshToken := "refresh-initial"
		if request.URL.Path == "/api/open/oauth2/refresh_token" {
			accessToken = "access-refreshed"
			refreshToken = "refresh-refreshed"
		}
		_, _ = writer.Write([]byte(`{
			"code":0,
			"success":true,
			"msg":"成功",
			"data":{
				"access_token":"` + accessToken + `",
				"access_token_expires_in":3600,
				"refresh_token":"` + refreshToken + `",
				"refresh_token_expires_in":2592000,
				"user_id":"user-1",
				"app_id":11344,
				"platform_type":1,
				"approval_advertisers":[{"advertiser_id":1234,"advertiser_name":"测试广告主"}]
			}
		}`))
	}))
	defer upstream.Close()

	client, err := xhs.NewClient(11344, "secret", xhs.WithBaseURL(upstream.URL), xhs.WithHTTPClient(upstream.Client()))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := xhs.NewTokenManager(client, filepath.Join(t.TempDir(), "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(manager, time.Second)

	assertStatusCode(t, handler, http.MethodGet, "/healthz", nil, http.StatusOK)
	assertStatusCode(t, handler, http.MethodGet, "/readyz", nil, http.StatusServiceUnavailable)

	authorizeBody := bytes.NewBufferString(`{"auth_code":"auth-code"}`)
	authorizeResponse := assertStatusCode(t, handler, http.MethodPost, "/v1/oauth/authorize", authorizeBody, http.StatusOK)
	if strings.Contains(authorizeResponse, `"access_token":"`) || strings.Contains(authorizeResponse, `"refresh_token":"`) {
		t.Fatal("authorize response exposed OAuth tokens")
	}
	assertStatusCode(t, handler, http.MethodGet, "/readyz", nil, http.StatusOK)
	statusResponse := assertStatusCode(t, handler, http.MethodGet, "/v1/oauth/status", nil, http.StatusOK)
	if !strings.Contains(statusResponse, `"authorized":true`) || !strings.Contains(statusResponse, `"advertiser_id":1234`) {
		t.Fatalf("unexpected status response: %s", statusResponse)
	}
	refreshResponse := assertStatusCode(t, handler, http.MethodPost, "/v1/oauth/refresh", nil, http.StatusOK)
	if strings.Contains(refreshResponse, `"access_token":"`) || strings.Contains(refreshResponse, `"refresh_token":"`) {
		t.Fatal("refresh response exposed OAuth tokens")
	}
	if tokenCalls != 2 {
		t.Fatalf("upstream token calls = %d, want 2", tokenCalls)
	}
}

func TestAuthHandlerRejectsInvalidRequests(t *testing.T) {
	client, err := xhs.NewClient(11344, "secret")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := xhs.NewTokenManager(client, filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(manager, time.Second)

	assertStatusCode(t, handler, http.MethodPost, "/v1/oauth/authorize", bytes.NewBufferString(`{}`), http.StatusBadRequest)
	assertStatusCode(t, handler, http.MethodPost, "/v1/oauth/refresh", nil, http.StatusConflict)
	assertStatusCode(t, handler, http.MethodPost, "/v1/oauth/status", nil, http.StatusMethodNotAllowed)
	assertStatusCode(t, handler, http.MethodPost, "/v1/campaigns/list", bytes.NewBufferString(`{"advertiser_id":0}`), http.StatusBadRequest)
	assertStatusCode(t, handler, http.MethodPost, "/v1/campaigns/all", bytes.NewBufferString(`{"advertiser_id":123}`), http.StatusConflict)
	assertStatusCode(t, handler, http.MethodGet, "/v1/sync/status", nil, http.StatusServiceUnavailable)
	assertStatusCode(t, handler, http.MethodPost, "/v1/sync/campaigns", nil, http.StatusServiceUnavailable)
	assertStatusCode(t, handler, http.MethodPost, "/v1/sync/units", nil, http.StatusServiceUnavailable)
	assertStatusCode(t, handler, http.MethodGet, "/v1/sync/creativities", nil, http.StatusMethodNotAllowed)
	for _, path := range []string{"/v1/sync/refresh", "/v1/sync/full"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("POST %s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestAuthHandlerProxiesCampaignQueries(t *testing.T) {
	var campaignRequests []xhs.CampaignListRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/open/oauth2/access_token":
			_, _ = writer.Write([]byte(`{
				"code":0,"success":true,"data":{
					"access_token":"campaign-access","access_token_expires_in":3600,
					"refresh_token":"campaign-refresh","refresh_token_expires_in":2592000,
					"user_id":"user-1","app_id":11344
				}
			}`))
		case "/api/open/jg/campaign/list":
			if token := request.Header.Get("Access-Token"); token != "campaign-access" {
				t.Errorf("Access-Token = %q", token)
			}
			var payload xhs.CampaignListRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode campaign request: %v", err)
				return
			}
			campaignRequests = append(campaignRequests, payload)
			_, _ = writer.Write([]byte(`{
				"code":0,"success":true,
				"data":{"page":{"page_index":1,"total_count":1},
				"base_campaign_dtos":[{"campaign_id":6243312,"campaign_name":"计划名称_test"}]}
			}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer upstream.Close()

	client, err := xhs.NewClient(11344, "secret", xhs.WithBaseURL(upstream.URL), xhs.WithHTTPClient(upstream.Client()))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := xhs.NewTokenManager(client, filepath.Join(t.TempDir(), "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authorize(context.Background(), "auth-code"); err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(manager, time.Second)

	listResponse := assertStatusCode(t, handler, http.MethodPost, "/v1/campaigns/list", bytes.NewBufferString(`{"advertiser_id":123,"page":{"pageIndex":2,"pageSize":50}}`), http.StatusOK)
	allResponse := assertStatusCode(t, handler, http.MethodPost, "/v1/campaigns/all", bytes.NewBufferString(`{"advertiser_id":123,"status":6}`), http.StatusOK)
	if !strings.Contains(listResponse, `"campaign_id":6243312`) || !strings.Contains(allResponse, `"total_count":1`) {
		t.Fatalf("unexpected campaign responses: list=%s all=%s", listResponse, allResponse)
	}
	if len(campaignRequests) != 2 {
		t.Fatalf("campaign requests = %d, want 2", len(campaignRequests))
	}
	if page := campaignRequests[0].Page; page == nil || page.PageIndex != 2 || page.PageSize != 50 {
		t.Fatalf("list page = %+v", page)
	}
	if page := campaignRequests[1].Page; page == nil || page.PageIndex != 1 || page.PageSize != 100 {
		t.Fatalf("all page = %+v", page)
	}
}

func assertStatusCode(t *testing.T, handler http.Handler, method, path string, body *bytes.Buffer, want int) string {
	t.Helper()
	var request *http.Request
	if body == nil {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, body)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, recorder.Code, want, recorder.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return recorder.Body.String()
}

func TestRequireLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18080", "[::1]:18080", "localhost:18080"} {
		if err := requireLoopbackAddress(address); err != nil {
			t.Errorf("requireLoopbackAddress(%q) error = %v", address, err)
		}
	}
	if err := requireLoopbackAddress("0.0.0.0:18080"); err == nil {
		t.Fatal("requireLoopbackAddress accepted a non-loopback address")
	}
}

func TestRunControlRequestHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := callDaemon(ctx, http.MethodGet, "http://127.0.0.1:1/status", nil, time.Second)
	if err == nil {
		t.Fatal("callDaemon() error = nil")
	}
}
