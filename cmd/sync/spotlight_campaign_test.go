package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type spotlightCampaignStub struct {
	query        maituo.SpotlightCampaignQuery
	advertiserID int64
	campaignID   int64
	found        bool
	listCalls    int
	detailCalls  int
}

func (stub *spotlightCampaignStub) SpotlightCampaigns(_ context.Context, query maituo.SpotlightCampaignQuery) (maituo.SpotlightCampaignList, error) {
	stub.listCalls++
	stub.query = query
	return maituo.SpotlightCampaignList{Page: query.Page, PageSize: query.PageSize, Items: []maituo.SpotlightCampaignSummary{}}, nil
}

func (stub *spotlightCampaignStub) SpotlightCampaignDetail(_ context.Context, advertiserID, campaignID int64) (maituo.SpotlightCampaignDetail, bool, error) {
	stub.detailCalls++
	stub.advertiserID = advertiserID
	stub.campaignID = campaignID
	return maituo.SpotlightCampaignDetail{Campaign: maituo.SpotlightCampaignSummary{AdvertiserID: advertiserID, CampaignID: campaignID}}, stub.found, nil
}

func TestSpotlightCampaigns(t *testing.T) {
	stub := &spotlightCampaignStub{}
	server := &apiServer{spotlightStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/spotlight/campaigns?q=218463780&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.spotlightCampaigns(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.listCalls != 1 || stub.query.Search != "218463780" || stub.query.Page != 2 || stub.query.PageSize != 40 {
		t.Fatalf("calls = %d, query = %+v", stub.listCalls, stub.query)
	}
}

func TestSpotlightCampaignDetail(t *testing.T) {
	stub := &spotlightCampaignStub{found: true}
	server := &apiServer{spotlightStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/spotlight/campaign-detail?advertiser_id=11517228&campaign_id=218463780", nil)
	response := httptest.NewRecorder()

	server.spotlightCampaignDetail(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.detailCalls != 1 || stub.advertiserID != 11517228 || stub.campaignID != 218463780 {
		t.Fatalf("calls = %d, advertiser = %d, campaign = %d", stub.detailCalls, stub.advertiserID, stub.campaignID)
	}
}

func TestSpotlightCampaignDetailRejectsInvalidIDs(t *testing.T) {
	stub := &spotlightCampaignStub{}
	server := &apiServer{spotlightStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/spotlight/campaign-detail?advertiser_id=0&campaign_id=abc", nil)
	response := httptest.NewRecorder()

	server.spotlightCampaignDetail(response, request)
	if response.Code != http.StatusBadRequest || stub.detailCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.detailCalls)
	}
}

func TestSpotlightCampaignDetailReturnsNotFound(t *testing.T) {
	stub := &spotlightCampaignStub{}
	server := &apiServer{spotlightStore: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/spotlight/campaign-detail?advertiser_id=1&campaign_id=2", nil)
	response := httptest.NewRecorder()

	server.spotlightCampaignDetail(response, request)
	if response.Code != http.StatusNotFound || stub.detailCalls != 1 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.detailCalls)
	}
}
