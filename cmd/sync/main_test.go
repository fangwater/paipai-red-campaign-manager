package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/syncer"
)

type baseSyncStub struct {
	calls  int
	result model.SyncResult
	err    error
}

func (stub *baseSyncStub) Run(context.Context) (model.SyncResult, error) {
	stub.calls++
	return stub.result, stub.err
}

type manuscriptSyncStub struct {
	providerCodes []string
	result        model.ProviderSyncResult
	err           error
}

func (stub *manuscriptSyncStub) RunProviders(_ context.Context, providerCodes []string) (model.ProviderSyncResult, error) {
	stub.providerCodes = append([]string(nil), providerCodes...)
	return stub.result, stub.err
}

type dandelionStatusStub struct {
	runs []store.LarkSyncRun
	err  error
}

func (stub *dandelionStatusStub) LarkSyncRuns(context.Context, int) ([]store.LarkSyncRun, error) {
	return stub.runs, stub.err
}

type manuscriptStatusStub struct {
	tables []model.ProviderContentTable
	err    error
}

func (stub *manuscriptStatusStub) ProviderContentTables(context.Context) ([]model.ProviderContentTable, error) {
	return stub.tables, stub.err
}

func testAPIHandler(manuscripts *manuscriptSyncStub, status *manuscriptStatusStub) http.Handler {
	return testAPIHandlerTargets(&baseSyncStub{}, manuscripts, status)
}

func testAPIHandlerTargets(dandelion *baseSyncStub, manuscripts *manuscriptSyncStub, status *manuscriptStatusStub) http.Handler {
	return newAPIHandler(&apiServer{
		dandelionSync:   dandelion,
		dandelionStatus: &dandelionStatusStub{runs: []store.LarkSyncRun{}},
		manuscriptSync:  manuscripts,
		statusStore:     status,
		timeout:         time.Second,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func TestManuscriptSyncEndpointSelectsProviders(t *testing.T) {
	manuscripts := &manuscriptSyncStub{result: model.ProviderSyncResult{Providers: 2, Upserted: 12}}
	handler := testAPIHandler(manuscripts, &manuscriptStatusStub{})

	response := performRequest(t, handler, http.MethodPost, "/v1/sync/manuscripts",
		`{"provider_codes":["manjie","zhiyuan"]}`, http.StatusOK)
	if strings.Join(manuscripts.providerCodes, ",") != "manjie,zhiyuan" {
		t.Fatalf("provider codes = %v", manuscripts.providerCodes)
	}
	if !strings.Contains(response, `"providers":2`) || !strings.Contains(response, `"upserted":12`) {
		t.Fatalf("response = %s", response)
	}
}

func TestManuscriptSyncEndpointRejectsUnknownProvider(t *testing.T) {
	handler := testAPIHandler(&manuscriptSyncStub{err: syncer.ErrUnknownProvider}, &manuscriptStatusStub{})
	performRequest(t, handler, http.MethodPost, "/v1/sync/manuscripts",
		`{"provider_codes":["missing"]}`, http.StatusBadRequest)
}

func TestDandelionSyncEndpointIsSeparate(t *testing.T) {
	dandelion := &baseSyncStub{result: model.SyncResult{Tables: 1, Fetched: 3103, Upserted: 3103}}
	manuscripts := &manuscriptSyncStub{}
	handler := testAPIHandlerTargets(dandelion, manuscripts, &manuscriptStatusStub{})

	response := performRequest(t, handler, http.MethodPost, "/v1/sync/dandelion", "{}", http.StatusOK)
	if dandelion.calls != 1 || manuscripts.providerCodes != nil {
		t.Fatalf("dandelion calls=%d manuscript codes=%v", dandelion.calls, manuscripts.providerCodes)
	}
	if !strings.Contains(response, `"tables":1`) || !strings.Contains(response, `"fetched":3103`) {
		t.Fatalf("response = %s", response)
	}
}

func TestDandelionStatusEndpoint(t *testing.T) {
	handler := testAPIHandlerTargets(&baseSyncStub{}, &manuscriptSyncStub{}, &manuscriptStatusStub{})
	server := handler
	response := performRequest(t, server, http.MethodGet, "/v1/sync/dandelion/status", "", http.StatusOK)
	if !strings.Contains(response, "\"recent\":[]") {
		t.Fatalf("response = %s", response)
	}
}

func TestManuscriptStatusEndpoint(t *testing.T) {
	syncedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	handler := testAPIHandler(&manuscriptSyncStub{}, &manuscriptStatusStub{
		tables: []model.ProviderContentTable{{
			ProviderCode: "manjie", ProviderName: "曼杰", SheetName: "达人笔记执行表",
			LastSyncStatus: "succeeded", LastSyncedAt: &syncedAt,
		}},
	})

	response := performRequest(t, handler, http.MethodGet, "/v1/sync/manuscripts/status", "", http.StatusOK)
	if !strings.Contains(response, `"provider_code":"manjie"`) || !strings.Contains(response, `"status":"succeeded"`) {
		t.Fatalf("response = %s", response)
	}
}

func TestManualSyncHandlerRejectsInvalidRequests(t *testing.T) {
	handler := testAPIHandler(&manuscriptSyncStub{}, &manuscriptStatusStub{})
	performRequest(t, handler, http.MethodGet, "/v1/sync/manuscripts", "", http.StatusMethodNotAllowed)
	performRequest(t, handler, http.MethodPost, "/v1/sync/manuscripts", `{"unknown":true}`, http.StatusBadRequest)
	performRequest(t, handler, http.MethodPost, "/v1/sync/dandelion", `{"provider_codes":[]}`, http.StatusBadRequest)

	request := httptest.NewRequest(http.MethodPost, "/v1/sync/base", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("removed base endpoint status=%d want=%d", recorder.Code, http.StatusNotFound)
	}
}

func TestRequireLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18081", "[::1]:18081", "localhost:18081"} {
		if err := requireLoopbackAddress(address); err != nil {
			t.Errorf("requireLoopbackAddress(%q) error = %v", address, err)
		}
	}
	if err := requireLoopbackAddress("0.0.0.0:18081"); err == nil {
		t.Fatal("non-loopback address was accepted")
	}
}

func performRequest(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) string {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, recorder.Code, wantStatus, recorder.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return recorder.Body.String()
}

func TestWriteSyncResultMapsAlreadyRunning(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeSyncResult(recorder, model.ProviderSyncResult{}, errors.Join(syncer.ErrAlreadyRunning))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusConflict)
	}
}
