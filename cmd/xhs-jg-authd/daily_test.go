package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhssync"
)

func TestRunDailySyncRunsAllTargetsInOrder(t *testing.T) {
	var mutex sync.Mutex
	var paths []string
	var runs []store.XHSJGSyncRun
	var nextRunID int64

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/v1/sync/status" {
			mutex.Lock()
			recent := append([]store.XHSJGSyncRun(nil), runs...)
			mutex.Unlock()
			writeJSON(writer, http.StatusOK, apiResponse{
				Success: true,
				Data:    xhssync.Status{Recent: recent},
			})
			return
		}
		if request.Method != http.MethodPost {
			http.NotFound(writer, request)
			return
		}
		var payload syncRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode sync request: %v", err)
		}
		if payload.Mode != string(xhssync.ModeFull) {
			t.Errorf("mode = %q, want full", payload.Mode)
		}

		mutex.Lock()
		paths = append(paths, request.URL.Path)
		nextRunID++
		runID := nextRunID
		target := strings.TrimPrefix(request.URL.Path, "/v1/sync/")
		runs = append([]store.XHSJGSyncRun{{
			RunID:            runID,
			Target:           target,
			Mode:             string(xhssync.ModeFull),
			Status:           "succeeded",
			AdvertisersCount: 59,
		}}, runs...)
		mutex.Unlock()

		writeJSON(writer, http.StatusAccepted, apiResponse{
			Success: true,
			Data: store.XHSJGSyncRun{
				RunID:  runID,
				Target: target,
				Mode:   string(xhssync.ModeFull),
				Status: "running",
			},
		})
	}))
	defer server.Close()

	var output bytes.Buffer
	err := runDailySync(context.Background(), []string{
		"--url", server.URL,
		"--timeout", time.Second.String(),
		"--poll-interval", time.Millisecond.String(),
		"--request-timeout", time.Second.String(),
	}, &output)
	if err != nil {
		t.Fatal(err)
	}

	mutex.Lock()
	gotPaths := append([]string(nil), paths...)
	mutex.Unlock()
	wantPaths := []string{
		"/v1/sync/campaigns",
		"/v1/sync/units",
		"/v1/sync/creativities",
	}
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("sync paths = %v, want %v", gotPaths, wantPaths)
	}
	if count := strings.Count(output.String(), "completed XHS Spotlight"); count != 3 {
		t.Fatalf("completed output count = %d; output = %s", count, output.String())
	}
}

func TestWaitForDailySyncRunWaitsUntilServiceIsIdle(t *testing.T) {
	const runID int64 = 73
	var mutex sync.Mutex
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/sync/status" {
			http.NotFound(writer, request)
			return
		}
		mutex.Lock()
		checks++
		running := checks == 1
		mutex.Unlock()
		writeJSON(writer, http.StatusOK, apiResponse{
			Success: true,
			Data: xhssync.Status{
				Running: running,
				Recent: []store.XHSJGSyncRun{{
					RunID:  runID,
					Target: string(xhssync.TargetCampaigns),
					Mode:   string(xhssync.ModeFull),
					Status: "succeeded",
				}},
			},
		})
	}))
	defer server.Close()

	completed, err := waitForDailySyncRun(
		context.Background(),
		server.Client(),
		server.URL,
		runID,
		time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.RunID != runID {
		t.Fatalf("completed run ID = %d, want %d", completed.RunID, runID)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if checks < 2 {
		t.Fatalf("status checks = %d, want at least 2", checks)
	}
}

func TestRunDailySyncReturnsFailedRun(t *testing.T) {
	failedRun := store.XHSJGSyncRun{
		RunID:        41,
		Target:       string(xhssync.TargetCampaigns),
		Mode:         string(xhssync.ModeFull),
		Status:       "failed",
		ErrorMessage: "upstream unavailable",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1/sync/campaigns":
			started := failedRun
			started.Status = "running"
			started.ErrorMessage = ""
			writeJSON(writer, http.StatusAccepted, apiResponse{Success: true, Data: started})
		case request.Method == http.MethodGet && request.URL.Path == "/v1/sync/status":
			writeJSON(writer, http.StatusOK, apiResponse{
				Success: true,
				Data:    xhssync.Status{Recent: []store.XHSJGSyncRun{failedRun}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	err := runDailySync(context.Background(), []string{
		"--url", server.URL,
		"--timeout", time.Second.String(),
		"--poll-interval", time.Millisecond.String(),
		"--request-timeout", time.Second.String(),
	}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), failedRun.ErrorMessage) {
		t.Fatalf("error = %v, want failed run message", err)
	}
}
