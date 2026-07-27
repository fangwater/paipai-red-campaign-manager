package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type maituoAnalyticsStub struct {
	query  maituo.NoteCampaignAnalysisQuery
	result maituo.NoteCampaignAnalysis
	calls  int
}

func (stub *maituoAnalyticsStub) MaituoNoteCampaignAnalysis(_ context.Context, query maituo.NoteCampaignAnalysisQuery) (maituo.NoteCampaignAnalysis, error) {
	stub.calls++
	stub.query = query
	return stub.result, nil
}

func TestMaituoNoteCampaignAnalysis(t *testing.T) {
	stub := &maituoAnalyticsStub{result: maituo.NoteCampaignAnalysis{Window: "3d", Total: 1}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?window=3d&q=campaign&sort=daily_spend&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.query.Window != "3d" || stub.query.Search != "campaign" || stub.query.Sort != "daily_spend" || stub.query.Page != 2 || stub.query.PageSize != 40 {
		t.Fatalf("query = %+v", stub.query)
	}
	var payload apiResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestMaituoNoteCampaignAnalysisRejectsInvalidSort(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?sort=search_cost", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.calls)
	}
}

func TestMaituoNoteCampaignAnalysisRejectsInvalidWindow(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?window=30d", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.calls)
	}
}
