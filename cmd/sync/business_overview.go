package main

import (
	"context"
	"net/http"
	"strings"

	"paipai-red-campaign-manager/internal/model"
)

type businessOverviewStore interface {
	BusinessOverview(context.Context, int, string) (model.BusinessOverview, error)
}

func (server *apiServer) businessOverviewHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	days, ok := positiveQueryInt(request, "days", 7, 7, 30)
	if !ok || (days != 7 && days != 14 && days != 30) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "days 仅支持 7、14 或 30"})
		return
	}
	spu := strings.TrimSpace(request.URL.Query().Get("spu"))
	if spu == "" {
		spu = "辅酶"
	}
	if spu != "辅酶" && spu != "磷虾油" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "spu 仅支持辅酶或磷虾油"})
		return
	}
	if server.businessOverview == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "数据总览服务未配置"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.businessOverview.BusinessOverview(ctx, days, spu)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}
