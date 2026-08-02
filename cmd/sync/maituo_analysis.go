package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
)

type maituoAnalyticsStore interface {
	MaituoNoteCampaignAnalysis(context.Context, maituo.NoteCampaignAnalysisQuery) (maituo.NoteCampaignAnalysis, error)
	MaituoNoteContent(context.Context, string) (maituo.NoteContent, error)
	MaituoTrafficComparison(context.Context, maituo.TrafficComparisonQuery) (maituo.TrafficComparison, error)
	MaituoTrafficDeliveryComparison(context.Context, maituo.TrafficDeliveryComparisonQuery) (maituo.TrafficDeliveryComparison, error)
	MaituoAccountPlanDiagnosis(context.Context, string) (maituo.AccountPlanDiagnosis, error)
}

func (server *apiServer) maituoAccountPlanDiagnosis(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	spu := strings.TrimSpace(request.URL.Query().Get("spu"))
	if spu == "" {
		spu = "辅酶"
	}
	if len([]rune(spu)) > 50 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "spu 不能超过 50 个字符"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoAccountPlanDiagnosis(ctx, spu)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) maituoNoteCampaignAnalysis(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	window := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("window")))
	if window == "" {
		window = "7d"
	}
	if window != "3d" && window != "7d" && window != "all" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "window 仅支持 3d、7d 或 all"})
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
	planID := strings.TrimSpace(request.URL.Query().Get("plan_id"))
	if len([]rune(planID)) > 200 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "plan_id 不能超过 200 个字符"})
		return
	}
	sort := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("sort")))
	if sort == "" {
		sort = "cumulative_spend"
	}
	if sort != "daily_spend" && sort != "cumulative_spend" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "sort 仅支持 daily_spend 或 cumulative_spend"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoNoteCampaignAnalysis(ctx, maituo.NoteCampaignAnalysisQuery{
		Window: window, Search: search, PlanID: planID, Sort: sort, Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) maituoNoteContent(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	noteID := strings.TrimSpace(request.URL.Query().Get("note_id"))
	if noteID == "" || len([]rune(noteID)) > 200 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "note_id 必须填写且不能超过 200 个字符"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoNoteContent(ctx, noteID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func positiveQueryInt(request *http.Request, name string, fallback, minimum, maximum int) (int, bool) {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, false
	}
	return parsed, true
}

func (server *apiServer) maituoTrafficComparison(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	window := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("window")))
	if window == "" {
		window = "7d"
	}
	if window != "3d" && window != "7d" && window != "all" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "window 仅支持 3d、7d 或 all"})
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

	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoTrafficComparison(ctx, maituo.TrafficComparisonQuery{
		Window: window, Search: search, Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) maituoTrafficDeliveryComparison(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	noteID := strings.TrimSpace(request.URL.Query().Get("note_id"))
	if noteID == "" || len([]rune(noteID)) > 200 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "note_id 必须填写且不能超过 200 个字符"})
		return
	}
	placement := strings.TrimSpace(request.URL.Query().Get("placement"))
	if placement != "信息流" && placement != "搜索" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "placement 仅支持信息流或搜索"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoTrafficDeliveryComparison(ctx, maituo.TrafficDeliveryComparisonQuery{
		NoteID: noteID, Placement: placement,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}
