package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type maituoImportStub struct {
	saved []maituo.SavedImport
}

func (stub *maituoImportStub) ImportMaituoCustomerDaily(context.Context, maituo.Snapshot) (maituo.ImportResult, error) {
	return maituo.ImportResult{}, nil
}

func (stub *maituoImportStub) SavedMaituoImports(context.Context) ([]maituo.SavedImport, error) {
	return stub.saved, nil
}

func TestMaituoImportEndpointListsSavedReports(t *testing.T) {
	handler := newAPIHandler(&apiServer{
		maituoImport: &maituoImportStub{saved: []maituo.SavedImport{{
			RunID: 7, FileName: "2026-07-23-MaiTuo.xlsx", FileSHA256: "abc",
			ReportDate: "2026-07-23", Fetched: 275, CompletedAt: time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC),
		}}},
		timeout: time.Second,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/imports/maituo-customer-daily", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, expected := range []string{`"report_date":"2026-07-23"`, `"file_sha256":"abc"`, `"fetched":275`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
}
