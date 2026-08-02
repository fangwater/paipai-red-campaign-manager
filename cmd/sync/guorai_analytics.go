package main

import (
	"context"
	"net/http"
	"strings"

	"paipai-red-campaign-manager/internal/model"
)

type guoraiAnalyticsStore interface {
	GuoraiLatest(context.Context, model.GuoraiLatestQuery) (model.GuoraiLatestResult, error)
}

func (server *apiServer) guoraiLatest(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	entityType := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("type")))
	if entityType == "" {
		entityType = "note"
	}
	if entityType != "note" && entityType != "plan" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "type 仅支持 note 或 plan"})
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
	page, ok := positiveQueryInt(request, "page", 1, 1, 100000)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "page 必须是正整数"})
		return
	}
	pageSize, ok := positiveQueryInt(request, "page_size", 25, 1, 100)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "page_size 必须是 1 到 100 的整数"})
		return
	}
	search := strings.TrimSpace(request.URL.Query().Get("q"))
	if len([]rune(search)) > 200 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "搜索内容不能超过 200 个字符"})
		return
	}
	sortOption := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("sort")))
	if sortOption == "" {
		sortOption = "roi"
	}
	switch sortOption {
	case "publish_time", "payment", "cost", "roi":
	default:
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "sort 仅支持 publish_time、payment、cost 或 roi"})
		return
	}
	if server.guoraiAnalytics == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "薯量数据服务未配置"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.guoraiAnalytics.GuoraiLatest(ctx, model.GuoraiLatestQuery{
		EntityType: entityType,
		SPU:        spu,
		Search:     search,
		Sort:       sortOption,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}
