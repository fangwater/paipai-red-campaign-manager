package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

type guoraiAnalyticsStub struct {
	query  model.GuoraiLatestQuery
	result model.GuoraiLatestResult
	err    error
}

func (stub *guoraiAnalyticsStub) GuoraiLatest(_ context.Context, query model.GuoraiLatestQuery) (model.GuoraiLatestResult, error) {
	stub.query = query
	return stub.result, stub.err
}

type businessOverviewStub struct {
	days   int
	result model.BusinessOverview
	err    error
}

func (stub *businessOverviewStub) BusinessOverview(_ context.Context, days int) (model.BusinessOverview, error) {
	stub.days = days
	return stub.result, stub.err
}

func TestGuoraiLatestHandler(t *testing.T) {
	stub := &guoraiAnalyticsStub{result: model.GuoraiLatestResult{
		EntityType: "plan",
		Total:      41,
		Page:       2,
		PageSize:   25,
		Items:      []model.GuoraiLatestItem{},
	}}
	server := &apiServer{guoraiAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/guorai/latest?type=plan&spu=%E7%A3%B7%E8%99%BE%E6%B2%B9&q=summer&sort=roi&page=2&page_size=25", nil)
	response := httptest.NewRecorder()

	server.guoraiLatest(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stub.query.EntityType != "plan" || stub.query.SPU != "磷虾油" || stub.query.Search != "summer" || stub.query.Sort != "roi" || stub.query.Page != 2 || stub.query.PageSize != 25 {
		t.Fatalf("query=%+v", stub.query)
	}
	if !strings.Contains(response.Body.String(), `"total":41`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestGuoraiLatestHandlerRejectsInvalidSPU(t *testing.T) {
	server := &apiServer{guoraiAnalytics: &guoraiAnalyticsStub{}, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/guorai/latest?type=note&spu=other", nil)
	response := httptest.NewRecorder()

	server.guoraiLatest(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGuoraiLatestHandlerRejectsInvalidType(t *testing.T) {
	server := &apiServer{guoraiAnalytics: &guoraiAnalyticsStub{}, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/guorai/latest?type=campaign", nil)
	response := httptest.NewRecorder()

	server.guoraiLatest(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBusinessOverviewHandler(t *testing.T) {
	stub := &businessOverviewStub{result: model.BusinessOverview{Days: 14, SPU: "辅酶"}}
	server := &apiServer{businessOverview: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview?days=14", nil)
	response := httptest.NewRecorder()

	server.businessOverviewHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stub.days != 14 || !strings.Contains(response.Body.String(), `"spu":"辅酶"`) {
		t.Fatalf("days=%d body=%s", stub.days, response.Body.String())
	}
}

func TestBusinessOverviewHandlerRejectsUnsupportedPeriod(t *testing.T) {
	server := &apiServer{businessOverview: &businessOverviewStub{}, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview?days=21", nil)
	response := httptest.NewRecorder()

	server.businessOverviewHandler(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
