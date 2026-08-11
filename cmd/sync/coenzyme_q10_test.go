package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"
	"paipai-red-campaign-manager/internal/store"
)

type coenzymeQ10SyncStub struct {
	calls  int
	result coenzyme.SyncResult
	err    error
}

func (stub *coenzymeQ10SyncStub) Run(context.Context) (coenzyme.SyncResult, error) {
	stub.calls++
	return stub.result, stub.err
}

type coenzymeQ10StatusStub struct {
	status store.CoenzymeQ10SyncStatus
	err    error
}

func (stub *coenzymeQ10StatusStub) CoenzymeQ10SyncStatus(context.Context, int) (store.CoenzymeQ10SyncStatus, error) {
	return stub.status, stub.err
}

func TestCoenzymeQ10SyncAndStatusEndpoints(t *testing.T) {
	syncStub := &coenzymeQ10SyncStub{result: coenzyme.SyncResult{
		RunID: 8, SheetName: coenzyme.SheetName, Fetched: 35, Inserted: 2,
		Updated: 1, Unchanged: 32, EarliestDate: "2026-07-07", LatestDate: "2026-08-10",
	}}
	statusStub := &coenzymeQ10StatusStub{status: store.CoenzymeQ10SyncStatus{
		RecordCount: 35, EarliestDate: "2026-07-07", LatestDate: "2026-08-10",
		Recent: []store.CoenzymeQ10SyncRun{{RunID: 8, Status: "succeeded", Fetched: 35}},
	}}
	handler := newAPIHandler(&apiServer{
		coenzymeQ10Sync: syncStub, coenzymeQ10Status: statusStub,
		timeout: time.Second, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response := performRequest(t, handler, http.MethodPost, "/v1/sync/coenzyme-q10", "{}", http.StatusOK)
	if syncStub.calls != 1 || !strings.Contains(response, `"inserted":2`) || !strings.Contains(response, `"unchanged":32`) {
		t.Fatalf("calls=%d response=%s", syncStub.calls, response)
	}
	response = performRequest(t, handler, http.MethodGet, "/v1/sync/coenzyme-q10/status", "", http.StatusOK)
	if !strings.Contains(response, `"record_count":35`) || !strings.Contains(response, `"latest_date":"2026-08-10"`) {
		t.Fatalf("status response=%s", response)
	}
}
