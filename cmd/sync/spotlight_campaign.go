package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
)

type spotlightCampaignStore interface {
	SpotlightCampaigns(context.Context, maituo.SpotlightCampaignQuery) (maituo.SpotlightCampaignList, error)
	SpotlightCampaignDetail(context.Context, int64, int64) (maituo.SpotlightCampaignDetail, bool, error)
}

func (server *apiServer) spotlightCampaigns(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
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
	result, err := server.spotlightStore.SpotlightCampaigns(ctx, maituo.SpotlightCampaignQuery{Search: search, Page: page, PageSize: pageSize})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) spotlightCampaignDetail(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	advertiserID, advertiserOK := positiveInt64Query(request, "advertiser_id")
	campaignID, campaignOK := positiveInt64Query(request, "campaign_id")
	if !advertiserOK || !campaignOK {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "advertiser_id 和 campaign_id 必须是正整数"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, found, err := server.spotlightStore.SpotlightCampaignDetail(ctx, advertiserID, campaignID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "未找到该聚光计划"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func positiveInt64Query(request *http.Request, name string) (int64, bool) {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil && parsed > 0
}
