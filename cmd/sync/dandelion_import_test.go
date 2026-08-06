package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/dandelion"

	"github.com/xuri/excelize/v2"
)

type dandelionExcelImportStub struct {
	snapshot dandelion.Snapshot
	result   dandelion.ImportResult
}

func (stub *dandelionExcelImportStub) ImportDandelionExcel(_ context.Context, snapshot dandelion.Snapshot) (dandelion.ImportResult, error) {
	stub.snapshot = snapshot
	return stub.result, nil
}

func dandelionUploadBody(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	headers := []interface{}{
		"笔记ID", "笔记标题", "笔记链接", "博主昵称", "笔记发布日期", "下单账号", "SPU名称", "数据更新日期",
	}
	row := []interface{}{
		"0123456789abcdef01234567", "测试笔记", "https://example.com/note", "测试博主",
		"2026-08-01", "杭州智元文化传播有限公司", "辅酶", "2026-08-05",
	}
	if err := workbook.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatal(err)
	}
	data, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	_ = workbook.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "蒲公英.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

func TestDandelionExcelImportEndpoint(t *testing.T) {
	t.Setenv(dandelionUploadArchiveEnv, t.TempDir())
	stub := &dandelionExcelImportStub{result: dandelion.ImportResult{
		RunID: 9, Fetched: 1, Inserted: 1,
	}}
	handler := newAPIHandler(&apiServer{
		dandelionExcelImport: stub,
		timeout:              time.Second,
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	body, contentType := dandelionUploadBody(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/imports/dandelion-excel", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(stub.snapshot.Records) != 1 || stub.snapshot.Records[0].NoteID != "0123456789abcdef01234567" {
		t.Fatalf("snapshot = %+v", stub.snapshot)
	}
	for _, expected := range []string{`"run_id":9`, `"fetched":1`, `"inserted":1`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
}

func TestDandelionExcelImportEndpointRequiresPost(t *testing.T) {
	handler := newAPIHandler(&apiServer{})
	request := httptest.NewRequest(http.MethodGet, "/v1/imports/dandelion-excel", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
