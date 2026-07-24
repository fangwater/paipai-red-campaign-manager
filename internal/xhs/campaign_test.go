package xhs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListCampaigns(t *testing.T) {
	var received CampaignListRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != campaignListPath {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if token := request.Header.Get("Access-Token"); token != "access-token" {
			t.Errorf("Access-Token = %q", token)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code":0,"success":true,"msg":"成功",
			"data":{
				"page":{"page_index":1,"total_count":1},
				"base_campaign_dtos":[{
					"campaign_id":6243312,"campaign_name":"计划名称_test",
					"campaign_filter_state":2,"campaign_update_time":"2026-07-21 10:20:30",
					"campaign_day_budget":10000,"search_bid_ratio":1.25,
					"explore_config":{"campaign_day_budget":20000,"time_period_type":1,
						"start_time":1750000000000,"expire_hour":3,
						"time_period":{"mon":"101111111111111111111111"}}
				}]
			}
		}`))
	}))

	client := newCampaignTestClient(t, upstream)
	status := 6
	data, err := client.ListCampaigns(context.Background(), "access-token", CampaignListRequest{
		AdvertiserID: 123,
		Status:       &status,
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.AdvertiserID != 123 || received.Status == nil || *received.Status != 6 {
		t.Fatalf("request = %+v", received)
	}
	if received.Page == nil || received.Page.PageIndex != 1 || received.Page.PageSize != 20 {
		t.Fatalf("page = %+v", received.Page)
	}
	if data.Page.TotalCount != 1 || len(data.Campaigns) != 1 {
		t.Fatalf("data = %+v", data)
	}
	campaign := data.Campaigns[0]
	if campaign.CampaignID != 6243312 || campaign.CampaignName != "计划名称_test" || campaign.SearchBidRatio != 1.25 {
		t.Fatalf("campaign = %+v", campaign)
	}
	if campaign.ExploreConfig == nil || campaign.ExploreConfig.TimePeriod == nil || campaign.ExploreConfig.TimePeriod.Monday == "" {
		t.Fatalf("explore_config = %+v", campaign.ExploreConfig)
	}
}

func TestCampaignPageRequestJSONCompatibility(t *testing.T) {
	encoded, err := json.Marshal(CampaignPageRequest{PageIndex: 2, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"page_index":2,"page_size":100}` {
		t.Fatalf("encoded page = %s", encoded)
	}
	for _, input := range []string{
		`{"page_index":3,"page_size":50}`,
		`{"pageIndex":3,"pageSize":50}`,
	} {
		var page CampaignPageRequest
		if err := json.Unmarshal([]byte(input), &page); err != nil {
			t.Fatalf("decode %s: %v", input, err)
		}
		if page.PageIndex != 3 || page.PageSize != 50 {
			t.Fatalf("decoded %s as %+v", input, page)
		}
	}
	var mixed CampaignPageRequest
	if err := json.Unmarshal([]byte(`{"page_index":1,"pageIndex":1}`), &mixed); err == nil {
		t.Fatal("mixed page field styles were accepted")
	}
}

func TestListAllCampaignsPaginates(t *testing.T) {
	requestedPages := make([]int, 0, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload CampaignListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if payload.Page == nil || payload.Page.PageSize != 100 {
			t.Errorf("page = %+v", payload.Page)
			return
		}
		requestedPages = append(requestedPages, payload.Page.PageIndex)
		count := 100
		if payload.Page.PageIndex == 3 {
			count = 5
		}
		campaigns := make([]Campaign, count)
		for index := range campaigns {
			campaigns[index].CampaignID = int64((payload.Page.PageIndex-1)*100 + index + 1)
		}
		_ = json.NewEncoder(writer).Encode(campaignListEnvelope{
			Code: 0, Success: true,
			Data: CampaignListData{
				Page:      CampaignPage{PageIndex: payload.Page.PageIndex, TotalCount: 205},
				Campaigns: campaigns,
			},
		})
	}))

	client := newCampaignTestClient(t, upstream)
	result, err := client.ListAllCampaigns(context.Background(), "access-token", CampaignListRequest{
		AdvertiserID: 123,
		Page:         &CampaignPageRequest{PageIndex: 9, PageSize: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 205 || len(result.Campaigns) != 205 {
		t.Fatalf("result count = %d/%d", len(result.Campaigns), result.TotalCount)
	}
	if got := fmt.Sprint(requestedPages); got != "[1 2 3]" {
		t.Fatalf("requested pages = %s", got)
	}
}

func TestListCampaignsRejectsInvalidRequests(t *testing.T) {
	client := newCampaignTestClient(t, httptest.NewServer(http.NotFoundHandler()))
	status := 12
	tests := []CampaignListRequest{
		{},
		{AdvertiserID: 123, CampaignIDs: make([]int64, 21)},
		{AdvertiserID: 123, Status: &status},
		{AdvertiserID: 123, StartTime: "2026-07-01"},
		{AdvertiserID: 123, StartTime: "2026-07-02", ExpireTime: "2026-07-01"},
		{AdvertiserID: 123, Page: &CampaignPageRequest{PageSize: 101}},
	}
	for _, request := range tests {
		_, err := client.ListCampaigns(context.Background(), "access-token", request)
		if err == nil || !strings.Contains(err.Error(), ErrInvalidCampaignRequest.Error()) {
			t.Errorf("ListCampaigns(%+v) error = %v", request, err)
		}
	}
}

func TestListCampaignsReturnsAPIError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":40003,"success":false,"msg":"no permission","request_id":"req-1"}`))
	}))

	client := newCampaignTestClient(t, upstream)
	_, err := client.ListCampaigns(context.Background(), "access-token", CampaignListRequest{AdvertiserID: 123})
	if err == nil || !strings.Contains(err.Error(), "code=40003") || !strings.Contains(err.Error(), "request_id=req-1") {
		t.Fatalf("error = %v", err)
	}
}

func newCampaignTestClient(t *testing.T, upstream *httptest.Server) *Client {
	t.Helper()
	t.Cleanup(upstream.Close)
	client, err := NewClient(11344, "secret", WithBaseURL(upstream.URL), WithHTTPClient(upstream.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return client
}
