package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type maituoXHSLinkStub struct {
	query  maituo.XHSLinkQuery
	result maituo.XHSLinkResult
	calls  int
}

func (stub *maituoXHSLinkStub) MaituoXHSLinks(_ context.Context, query maituo.XHSLinkQuery) (maituo.XHSLinkResult, error) {
	stub.calls++
	stub.query = query
	return stub.result, nil
}

func TestMaituoXHSLinks(t *testing.T) {
	stub := &maituoXHSLinkStub{result: maituo.XHSLinkResult{ReportDate: "2026-07-23", Total: 243, Items: []maituo.XHSLinkItem{}}}
	server := &apiServer{maituoXHSLinksStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/xhs-links?q=campaign&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.maituoXHSLinks(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.calls != 1 || stub.query.Search != "campaign" || stub.query.Page != 2 || stub.query.PageSize != 40 {
		t.Fatalf("calls = %d, query = %+v", stub.calls, stub.query)
	}
}

func TestMaituoXHSLinksRejectsInvalidPagination(t *testing.T) {
	stub := &maituoXHSLinkStub{}
	server := &apiServer{maituoXHSLinksStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/xhs-links?page_size=101", nil)
	response := httptest.NewRecorder()

	server.maituoXHSLinks(response, request)
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.calls)
	}
}
