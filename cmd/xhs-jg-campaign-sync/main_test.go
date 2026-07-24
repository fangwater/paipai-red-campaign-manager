package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"paipai-red-campaign-manager/internal/xhs"
)

func TestDaemonClientListsAllActiveCampaigns(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/v1/campaigns/list" {
			t.Errorf("path = %s", request.URL.Path)
			http.NotFound(writer, request)
			return
		}
		var payload xhs.CampaignListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if payload.AdvertiserID != 123 || payload.Status == nil || *payload.Status != 6 {
			t.Errorf("payload = %+v", payload)
		}
		requestedPages = append(requestedPages, payload.Page.PageIndex)
		count := 100
		if payload.Page.PageIndex == 2 {
			count = 1
		}
		campaigns := make([]xhs.Campaign, count)
		for index := range campaigns {
			campaigns[index].CampaignID = int64((payload.Page.PageIndex-1)*100 + index + 1)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": true,
			"data": xhs.CampaignListData{
				Page:      xhs.CampaignPage{PageIndex: payload.Page.PageIndex, TotalCount: 101},
				Campaigns: campaigns,
			},
		})
	}))
	t.Cleanup(server.Close)

	client, err := newDaemonClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	campaigns, err := client.listAllActiveCampaigns(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(campaigns) != 101 || campaigns[100].CampaignID != 101 {
		t.Fatalf("campaigns = %d, last = %+v", len(campaigns), campaigns[100])
	}
	if got := fmt.Sprint(requestedPages); got != "[1 2]" {
		t.Fatalf("requested pages = %s", got)
	}
}

func TestSelectAdvertisers(t *testing.T) {
	status := xhs.ManagerStatus{ApprovalAdvertisers: []xhs.Advertiser{
		{ID: 30, Name: "third"}, {ID: 10, Name: "first"}, {ID: 30, Name: "duplicate"},
	}}
	all, err := selectAdvertisers(status, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != 10 || all[1].ID != 30 {
		t.Fatalf("all advertisers = %+v", all)
	}
	one, err := selectAdvertisers(status, 30)
	if err != nil || len(one) != 1 || one[0].Name != "third" {
		t.Fatalf("selected advertiser = %+v, error = %v", one, err)
	}
	if _, err := selectAdvertisers(status, 20); err == nil {
		t.Fatal("selectAdvertisers accepted an unauthorized advertiser")
	}
}
