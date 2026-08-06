package main

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDandelionExcelImportArchivesRejectedWorkbook(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(dandelionUploadArchiveEnv, directory)
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile("file", "无法解析.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not-an-xlsx-workbook")); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	handler := newAPIHandler(&apiServer{
		timeout: time.Second,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/imports/dandelion-excel", body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "文件已保存，编号：") {
		t.Fatalf("response = %s", recorder.Body.String())
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("archive entries = %d", len(entries))
	}
	data, err := os.ReadFile(directory + "/" + entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "not-an-xlsx-workbook" {
		t.Fatalf("archive data = %q", data)
	}
}
