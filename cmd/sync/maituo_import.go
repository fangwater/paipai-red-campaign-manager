package main

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/store"
)

const (
	maxExcelFileSize    = int64(50 << 20)
	maxExcelRequestSize = maxExcelFileSize + (1 << 20)
)

type maituoImportStore interface {
	ImportMaituoCustomerDaily(context.Context, maituo.Snapshot) (maituo.ImportResult, error)
	SavedMaituoImports(context.Context) ([]maituo.SavedImport, error)
	MaituoDailyNotes(context.Context, time.Time) (maituo.DailyNoteReport, error)
}

func (server *apiServer) importMaituoCustomerDaily(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		if strings.TrimSpace(request.URL.Query().Get("report_date")) != "" {
			server.getMaituoDailyNotes(writer, request)
			return
		}
		server.listSavedMaituoImports(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxExcelRequestSize)
	if err := request.ParseMultipartForm(8 << 20); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			writeJSON(writer, http.StatusRequestEntityTooLarge, apiResponse{Success: false, Error: "Excel 文件不能超过 50 MB"})
			return
		}
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "上传请求格式错误"})
		return
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "必须上传 Excel 文件"})
		return
	}
	defer file.Close()
	fileName := filepath.Base(header.Filename)
	if !strings.EqualFold(filepath.Ext(fileName), ".xlsx") {
		writeJSON(writer, http.StatusUnsupportedMediaType, apiResponse{Success: false, Error: "仅支持 .xlsx 文件"})
		return
	}
	if header.Size > maxExcelFileSize {
		writeJSON(writer, http.StatusRequestEntityTooLarge, apiResponse{Success: false, Error: "Excel 文件不能超过 50 MB"})
		return
	}
	snapshot, err := maituo.Parse(file, fileName)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, maituo.ErrInvalidWorkbook) {
			status = http.StatusBadRequest
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := server.maituoImport.ImportMaituoCustomerDaily(ctx, snapshot)
	server.logger.Info("Maituo customer daily import finished", "file", fileName, "sha256", snapshot.FileSHA256, "result", result, "duration", time.Since(startedAt), "error", err)
	if err == nil {
		writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
		return
	}
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, store.ErrMaituoImportLocked):
		status = http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, apiResponse{Success: false, Data: result, Error: err.Error()})
}

func (server *apiServer) listSavedMaituoImports(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	items, err := server.maituoImport.SavedMaituoImports(ctx)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: items})
}

func (server *apiServer) getMaituoDailyNotes(writer http.ResponseWriter, request *http.Request) {
	reportDate, err := time.Parse(time.DateOnly, strings.TrimSpace(request.URL.Query().Get("report_date")))
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "report_date 必须是 YYYY-MM-DD 格式"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoImport.MaituoDailyNotes(ctx, reportDate)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}
