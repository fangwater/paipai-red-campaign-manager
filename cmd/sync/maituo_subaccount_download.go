package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/store"
)

const maituoSubaccountDownloadPrefix = "/v1/downloads/maituo-subaccount/"

type maituoSubaccountStore interface {
	MaituoSubaccountDirectories(context.Context) ([]maituo.SubaccountDirectory, error)
	MaituoSubaccountReports(context.Context, string) ([]maituo.SubaccountReport, error)
	MaituoSubaccountSnapshot(context.Context, string, time.Time) (maituo.Snapshot, error)
}

func encodeSubaccountID(subaccount string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(subaccount)))
}

func decodeSubaccountID(value string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || !utf8.Valid(decoded) {
		return "", false
	}
	subaccount := strings.TrimSpace(string(decoded))
	return subaccount, subaccount != "" && subaccount != "总体" && encodeSubaccountID(subaccount) == value
}

func (server *apiServer) listMaituoSubaccountDirectories(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	items, err := server.maituoSubaccounts.MaituoSubaccountDirectories(ctx)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	for index := range items {
		items[index].AccountID = encodeSubaccountID(items[index].Subaccount)
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: items})
}

func (server *apiServer) maituoSubaccountDownload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	remainder := strings.Trim(strings.TrimPrefix(request.URL.Path, maituoSubaccountDownloadPrefix), "/")
	segments := strings.Split(remainder, "/")
	if len(segments) < 1 || len(segments) > 2 {
		http.NotFound(writer, request)
		return
	}
	subaccount, ok := decodeSubaccountID(segments[0])
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if len(segments) == 1 {
		server.listMaituoSubaccountReports(writer, request, subaccount)
		return
	}
	server.downloadMaituoSubaccountReport(writer, request, subaccount, segments[1])
}

func (server *apiServer) listMaituoSubaccountReports(writer http.ResponseWriter, request *http.Request, subaccount string) {
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	items, err := server.maituoSubaccounts.MaituoSubaccountReports(ctx, subaccount)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if len(items) == 0 {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "未找到该子账户的历史日报"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]interface{}{"subaccount": subaccount, "reports": items}})
}

func (server *apiServer) downloadMaituoSubaccountReport(writer http.ResponseWriter, request *http.Request, subaccount, filePart string) {
	if path.Ext(filePart) != ".xlsx" {
		http.NotFound(writer, request)
		return
	}
	reportDate, err := time.Parse(time.DateOnly, strings.TrimSuffix(filePart, ".xlsx"))
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	snapshot, err := server.maituoSubaccounts.MaituoSubaccountSnapshot(ctx, subaccount, reportDate)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, store.ErrMaituoSubaccountReportNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	workbook, err := maituo.BuildSubaccountWorkbook(subaccount, snapshot)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, maituo.ErrNoSubaccountData) {
			status = http.StatusNotFound
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	writer.Header().Set("Content-Disposition", `attachment; filename="maituo-subaccount.xlsx"; filename*=UTF-8''`+url.PathEscape(workbook.FileName))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(workbook.Data)
}
