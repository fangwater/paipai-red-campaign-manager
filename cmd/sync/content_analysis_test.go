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

type contentAnalysisStub struct {
	query  model.ContentAnalysisQuery
	result model.ContentAnalysis
	err    error
}

func (stub *contentAnalysisStub) ContentAnalysis(_ context.Context, query model.ContentAnalysisQuery) (model.ContentAnalysis, error) {
	stub.query = query
	return stub.result, stub.err
}

func TestContentAnalysisHandler(t *testing.T) {
	stub := &contentAnalysisStub{result: model.ContentAnalysis{SPU: "磷虾油", Agency: "曼杰", Dimension: "scenario"}}
	server := &apiServer{contentAnalysis: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/content-analysis?spu=%E7%A3%B7%E8%99%BE%E6%B2%B9&agency=%E6%9B%BC%E6%9D%B0&dimension=scenario&published_start_date=2026-07-01&published_end_date=2026-07-31", nil)
	response := httptest.NewRecorder()

	server.contentAnalysisHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stub.query.SPU != "磷虾油" || stub.query.Agency != "曼杰" || stub.query.Dimension != "scenario" ||
		stub.query.PublishedStartDate != "2026-07-01" || stub.query.PublishedEndDate != "2026-07-31" {
		t.Fatalf("query=%+v", stub.query)
	}
	if !strings.Contains(response.Body.String(), `"dimension":"scenario"`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestContentAnalysisHandlerRejectsInvalidFilters(t *testing.T) {
	server := &apiServer{contentAnalysis: &contentAnalysisStub{}, timeout: time.Second}
	for _, target := range []string{
		"/v1/analytics/content-analysis?spu=other",
		"/v1/analytics/content-analysis?agency=other",
		"/v1/analytics/content-analysis?dimension=other",
		"/v1/analytics/content-analysis?published_start_date=2026-02-30",
		"/v1/analytics/content-analysis?published_end_date=07-31-2026",
		"/v1/analytics/content-analysis?published_start_date=2026-08-01&published_end_date=2026-07-01",
	} {
		response := httptest.NewRecorder()
		server.contentAnalysisHandler(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, response.Code, response.Body.String())
		}
	}
}
