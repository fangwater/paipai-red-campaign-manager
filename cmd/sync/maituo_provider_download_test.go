package main

import (
	"bytes"
	"context"
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

type maituoProviderStub struct {
	directories []maituo.ProviderDirectory
	name        string
	reports     []maituo.ProviderReport
	snapshot    maituo.ProviderSnapshot
}

func (stub *maituoProviderStub) MaituoProviderDirectories(context.Context) ([]maituo.ProviderDirectory, error) {
	return stub.directories, nil
}

func (stub *maituoProviderStub) MaituoProviderReports(context.Context, string) (string, []maituo.ProviderReport, error) {
	return stub.name, stub.reports, nil
}

func (stub *maituoProviderStub) MaituoProviderSnapshot(context.Context, string, time.Time) (maituo.ProviderSnapshot, error) {
	return stub.snapshot, nil
}

func providerTestHandler(stub *maituoProviderStub) http.Handler {
	return newAPIHandler(&apiServer{maituoProviders: stub, timeout: time.Second, logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestMaituoProviderDirectoryAndDownload(t *testing.T) {
	date := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	stub := &maituoProviderStub{
		directories: []maituo.ProviderDirectory{{ProviderCode: "manjie", ProviderName: "曼杰", ReportCount: 2, NoteCount: 5}},
		name:        "曼杰",
		reports:     []maituo.ProviderReport{{ReportDate: "2026-08-23", FileName: "daily-曼杰.xlsx", NoteCount: 2}},
		snapshot: maituo.ProviderSnapshot{ProviderCode: "manjie", ProviderName: "曼杰", Snapshot: maituo.Snapshot{
			FileName: "2026-08-23-Maituo-客户日报.xlsx", ReportDate: date,
			Notes: []maituo.NoteDetail{{NoteID: "68a123456789abcdef123456", NoteURL: "https://example.com", Category: "信息流", Placement: "搜索"}},
		}},
	}
	handler := providerTestHandler(stub)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/imports/maituo-provider-directories", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"provider_code":"manjie"`) {
		t.Fatalf("directory status=%d body=%s", list.Code, list.Body.String())
	}

	directory := httptest.NewRecorder()
	handler.ServeHTTP(directory, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-provider/manjie", nil))
	if directory.Code != http.StatusOK || !strings.Contains(directory.Body.String(), `"note_count":2`) {
		t.Fatalf("reports status=%d body=%s", directory.Code, directory.Body.String())
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-provider/manjie/2026-08-23.xlsx", nil))
	if download.Code != http.StatusOK || !strings.Contains(download.Header().Get("Content-Disposition"), "2026-08-23-Maituo") {
		t.Fatalf("download status=%d headers=%v body=%s", download.Code, download.Header(), download.Body.String())
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(download.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	if sheets := workbook.GetSheetList(); len(sheets) != 1 || sheets[0] != maituo.SheetNotes {
		t.Fatalf("download sheets=%v", sheets)
	}
}

func TestMaituoProviderDirectoryRejectsInvalidCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	providerTestHandler(&maituoProviderStub{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/downloads/maituo-provider/not.valid", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
