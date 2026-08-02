package main

import (
	"context"
	"net/http"
	"strings"

	"paipai-red-campaign-manager/internal/model"
)

type contentAnalysisStore interface {
	ContentAnalysis(context.Context, model.ContentAnalysisQuery) (model.ContentAnalysis, error)
}

func (server *apiServer) contentAnalysisHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
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
	agency := strings.TrimSpace(request.URL.Query().Get("agency"))
	if agency == "" {
		agency = "全部"
	}
	if agency != "全部" && agency != "曼杰" && agency != "有一有二" && agency != "智元" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "agency 仅支持全部、曼杰、有一有二或智元"})
		return
	}
	dimension := strings.TrimSpace(request.URL.Query().Get("dimension"))
	if dimension == "" {
		dimension = "audience"
	}
	if dimension != "audience" && dimension != "scenario" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "dimension 仅支持 audience 或 scenario"})
		return
	}
	if server.contentAnalysis == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "内容分析服务未配置"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.contentAnalysis.ContentAnalysis(ctx, model.ContentAnalysisQuery{
		SPU: spu, Agency: agency, Dimension: dimension,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}
