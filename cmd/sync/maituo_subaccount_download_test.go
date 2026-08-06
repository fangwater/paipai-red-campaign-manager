package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/xuri/excelize/v2"
)

type maituoSubaccountStub struct {
	directories []maituo.SubaccountDirectory
	reports     map[string][]maituo.SubaccountReport
	snapshots   map[string]maituo.Snapshot
}

func (stub *maituoSubaccountStub) MaituoSubaccountDirectories(context.Context) ([]maituo.SubaccountDirectory, error) {
	return stub.directories, nil
}

func (stub *maituoSubaccountStub) MaituoSubaccountReports(_ context.Context, subaccount string) ([]maituo.SubaccountReport, error) {
	return stub.reports[subaccount], nil
}

func (stub *maituoSubaccountStub) MaituoSubaccountSnapshot(_ context.Context, subaccount string, reportDate time.Time) (maituo.Snapshot, error) {
	return stub.snapshots[subaccount+"/"+reportDate.Format(time.DateOnly)], nil
}

func subaccountTestHandler(stub *maituoSubaccountStub) http.Handler {
	return newAPIHandler(&apiServer{maituoSubaccounts: stub, timeout: time.Second, logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestMaituoSubaccountDirectoryAndSingleDateDownload(t *testing.T) {
	date := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	stub := &maituoSubaccountStub{
		directories: []maituo.SubaccountDirectory{{Subaccount: "账户A", ReportCount: 2, EarliestReportDate: "2026-08-04", LatestReportDate: "2026-08-05"}},
		reports:     map[string][]maituo.SubaccountReport{"账户A": {{ReportDate: "2026-08-05", FileName: "daily.xlsx"}}},
		snapshots: map[string]maituo.Snapshot{"账户A/2026-08-05": {
			FileName: "2026-08-05-Maituo-客户日报.xlsx", ReportDate: date,
			Notes:       []maituo.NoteDetail{{NoteID: "a", Subaccount: "账户A"}, {NoteID: "b", Subaccount: "账户B"}},
			Subaccounts: []maituo.SubaccountOverview{{SPU: "辅酶", Subaccount: "账户A"}, {SPU: "辅酶", Subaccount: "账户B"}},
		}},
	}
	handler := subaccountTestHandler(stub)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/imports/maituo-subaccount-directories", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"account_id":"6LSm5oi3QQ"`) {
		t.Fatalf("directory status=%d body=%s", list.Code, list.Body.String())
	}

	directory := httptest.NewRecorder()
	handler.ServeHTTP(directory, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-subaccount/6LSm5oi3QQ", nil))
	if directory.Code != http.StatusOK || !strings.Contains(directory.Body.String(), `"report_date":"2026-08-05"`) {
		t.Fatalf("reports status=%d body=%s", directory.Code, directory.Body.String())
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-subaccount/6LSm5oi3QQ/2026-08-05.xlsx", nil))
	if download.Code != http.StatusOK || !strings.Contains(download.Header().Get("Content-Disposition"), "2026-08-05-Maituo") {
		t.Fatalf("download status=%d headers=%v body=%s", download.Code, download.Header(), download.Body.String())
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(download.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	notes, _ := workbook.GetRows(maituo.SheetNotes)
	if len(notes) != 2 || notes[1][3] != "账户A" {
		t.Fatalf("download leaked rows: %v", notes)
	}
}

func TestMaituoSubaccountDirectoryRejectsInvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	subaccountTestHandler(&maituoSubaccountStub{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-subaccount/not-valid", nil))
	if recorder.Code != http.StatusNotFound {
		var payload map[string]interface{}
		_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
		t.Fatalf("status=%d body=%v", recorder.Code, payload)
	}
}
