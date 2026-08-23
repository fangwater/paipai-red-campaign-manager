package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/store"
)

const maituoProviderDownloadPrefix = "/v1/downloads/maituo-provider/"

var providerCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type maituoProviderStore interface {
	MaituoProviderDirectories(context.Context) ([]maituo.ProviderDirectory, error)
	MaituoProviderReports(context.Context, string) (string, []maituo.ProviderReport, error)
	MaituoProviderSnapshot(context.Context, string, time.Time) (maituo.ProviderSnapshot, error)
}

func (server *apiServer) listMaituoProviderDirectories(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	items, err := server.maituoProviders.MaituoProviderDirectories(ctx)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: items})
}

func (server *apiServer) maituoProviderDownload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	remainder := strings.Trim(strings.TrimPrefix(request.URL.Path, maituoProviderDownloadPrefix), "/")
	segments := strings.Split(remainder, "/")
	if len(segments) < 1 || len(segments) > 2 || !providerCodePattern.MatchString(segments[0]) {
		http.NotFound(writer, request)
		return
	}
	if len(segments) == 1 {
		server.listMaituoProviderReports(writer, request, segments[0])
		return
	}
	server.downloadMaituoProviderReport(writer, request, segments[0], segments[1])
}

func (server *apiServer) listMaituoProviderReports(writer http.ResponseWriter, request *http.Request, providerCode string) {
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	providerName, items, err := server.maituoProviders.MaituoProviderReports(ctx, providerCode)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, store.ErrMaituoProviderReportNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]interface{}{
		"provider_code": providerCode, "provider_name": providerName, "reports": items,
	}})
}

func (server *apiServer) downloadMaituoProviderReport(writer http.ResponseWriter, request *http.Request, providerCode, filePart string) {
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
	providerSnapshot, err := server.maituoProviders.MaituoProviderSnapshot(ctx, providerCode, reportDate)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, store.ErrMaituoProviderReportNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	workbook, err := maituo.BuildProviderWorkbook(providerSnapshot.ProviderName, providerSnapshot.Snapshot)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, maituo.ErrNoProviderData) {
			status = http.StatusNotFound
		}
		writeJSON(writer, status, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	writer.Header().Set("Content-Disposition", `attachment; filename="maituo-provider.xlsx"; filename*=UTF-8''`+url.PathEscape(workbook.FileName))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(workbook.Data)
}
