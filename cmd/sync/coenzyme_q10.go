package main

import (
	"context"
	"net/http"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"
	"paipai-red-campaign-manager/internal/store"
)

type coenzymeQ10SyncService interface {
	Run(context.Context) (coenzyme.SyncResult, error)
}

type coenzymeQ10StatusStore interface {
	CoenzymeQ10SyncStatus(context.Context, int) (store.CoenzymeQ10SyncStatus, error)
}

func (server *apiServer) syncCoenzymeQ10(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if !decodeOptionalEmptyRequest(writer, request) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := server.coenzymeQ10Sync.Run(ctx)
	server.logger.Info("Lark coenzyme Q10 daily sync finished",
		"result", result,
		"duration", time.Since(startedAt),
		"error", err,
	)
	writeSyncResult(writer, result, err)
}

func (server *apiServer) coenzymeQ10StatusHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	status, err := server.coenzymeQ10Status.CoenzymeQ10SyncStatus(ctx, 10)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: status})
}
