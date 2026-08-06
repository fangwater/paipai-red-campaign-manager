package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/dandelion"
	"paipai-red-campaign-manager/internal/store"
)

type dandelionExcelImportStore interface {
	ImportDandelionExcel(context.Context, dandelion.Snapshot) (dandelion.ImportResult, error)
}

func (server *apiServer) importDandelionExcel(writer http.ResponseWriter, request *http.Request) {
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
	data, err := io.ReadAll(io.LimitReader(file, maxExcelFileSize+1))
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "读取 Excel 文件失败"})
		return
	}
	if int64(len(data)) > maxExcelFileSize {
		writeJSON(writer, http.StatusRequestEntityTooLarge, apiResponse{Success: false, Error: "Excel 文件不能超过 50 MB"})
		return
	}
	uploadID, archivePath, err := archiveDandelionUpload(data, time.Now())
	if err != nil {
		server.logger.Error("archive Dandelion Excel upload", "file", fileName, "error", err)
		writeJSON(writer, http.StatusInternalServerError, apiResponse{Success: false, Error: "保存上传文件失败"})
		return
	}
	server.logger.Info("Dandelion Excel upload archived", "file", fileName, "upload_id", uploadID, "archive", archivePath, "bytes", len(data))
	snapshot, err := dandelion.Parse(bytes.NewReader(data), fileName)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, dandelion.ErrInvalidWorkbook) {
			status = http.StatusBadRequest
		}
		server.logger.Warn("Dandelion Excel workbook rejected", "file", fileName, "upload_id", uploadID, "archive", archivePath, "error", err)
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error() + "；文件已保存，编号：" + uploadID})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := server.dandelionExcelImport.ImportDandelionExcel(ctx, snapshot)
	server.logger.Info("Dandelion Excel import finished",
		"file", fileName, "upload_id", uploadID, "archive", archivePath, "sha256", snapshot.FileSHA256, "sheet", snapshot.SheetName,
		"result", result, "duration", time.Since(startedAt), "error", err)
	if err == nil {
		writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
		return
	}
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, store.ErrDandelionExcelImportLocked):
		status = http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, apiResponse{Success: false, Data: result, Error: err.Error()})
}
