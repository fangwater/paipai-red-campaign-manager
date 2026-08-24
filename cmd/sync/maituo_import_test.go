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
	saved       []maituo.SavedImport
	daily       maituo.DailyNoteReport
	dailyDate   time.Time
	dailyCalled int
}

func (stub *maituoImportStub) ImportMaituoCustomerDaily(context.Context, maituo.Snapshot) (maituo.ImportResult, error) {
	return maituo.ImportResult{}, nil
}

func (stub *maituoImportStub) SavedMaituoImports(context.Context) ([]maituo.SavedImport, error) {
	return stub.saved, nil
}

func (stub *maituoImportStub) MaituoDailyNotes(_ context.Context, reportDate time.Time) (maituo.DailyNoteReport, error) {
	stub.dailyCalled++
	stub.dailyDate = reportDate
	return stub.daily, nil
}

func TestMaituoImportEndpointListsSavedReports(t *testing.T) {
	handler := newAPIHandler(&apiServer{
		maituoImport: &maituoImportStub{saved: []maituo.SavedImport{{
			RunID: 7, FileName: "2026-07-23-MaiTuo.xlsx", FileSHA256: "abc",
			ReportDate: "2026-07-23", Fetched: 275, MergedRows: 227, CompletedAt: time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC),
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
	for _, expected := range []string{`"report_date":"2026-07-23"`, `"file_sha256":"abc"`, `"fetched":275`, `"merged_rows":227`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
}

func TestMaituoImportEndpointReturnsMergedDailyNotes(t *testing.T) {
	searchCost := 30.0
	stub := &maituoImportStub{daily: maituo.DailyNoteReport{
		ReportDate: "2026-07-23",
		Total:      1,
		Items: []maituo.NoteDetail{{
			NoteID: "note-1", NoteURL: "https://example.com/note-1", Category: "测评",
			Subaccount: "账户A", CampaignName: "计划A", Placement: "搜索", Spend: 150, SearchUsers: 5, SearchCost: &searchCost, CPC: 2.1429, CTRPct: 11.6667,
		}},
	}}
	handler := newAPIHandler(&apiServer{maituoImport: stub, timeout: time.Second})
	request := httptest.NewRequest(http.MethodGet, "/v1/imports/maituo-customer-daily?report_date=2026-07-23", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.dailyCalled != 1 || stub.dailyDate.Format(time.DateOnly) != "2026-07-23" {
		t.Fatalf("daily call count=%d date=%s", stub.dailyCalled, stub.dailyDate)
	}
	for _, expected := range []string{`"report_date":"2026-07-23"`, `"total":1`, `"note_id":"note-1"`, `"placement":"搜索"`, `"search_cost":30`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
	for _, expected := range []string{`"subaccount":"账户A"`, `"campaign_name":"计划A"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
}

func TestMaituoImportEndpointRejectsInvalidReportDate(t *testing.T) {
	stub := &maituoImportStub{}
	handler := newAPIHandler(&apiServer{maituoImport: stub, timeout: time.Second})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/imports/maituo-customer-daily?report_date=2026-7-23", nil))
	if recorder.Code != http.StatusBadRequest || stub.dailyCalled != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.dailyCalled, recorder.Body.String())
	}
}
