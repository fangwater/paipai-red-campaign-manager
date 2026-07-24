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

func TestDaemonClientListsAllActiveUnits(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload xhs.UnitListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		requestedPages = append(requestedPages, payload.Page)
		units := []xhs.Unit{{UnitID: int64(payload.Page), CampaignID: 10, UnitFilterState: 10}}
		if payload.Page == 1 {
			units = append(units, xhs.Unit{UnitID: 99, CampaignID: 10, UnitFilterState: 1})
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": true,
			"data":    xhs.UnitListData{TotalCount: 3, Units: units},
		})
	}))
	t.Cleanup(server.Close)
	client, err := newDaemonClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	units, err := client.listAllActiveUnits(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || fmt.Sprint(requestedPages) != "[1 2]" {
		t.Fatalf("units/pages = %+v/%v", units, requestedPages)
	}
}

func TestDaemonClientListsAllActiveCreativities(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload xhs.CreativityListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		requestedPages = append(requestedPages, payload.Page.PageIndex)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": true,
			"data": xhs.CreativityListData{
				Page:         xhs.CampaignPage{PageIndex: payload.Page.PageIndex, TotalCount: 2},
				Creativities: []xhs.Creativity{{CreativityID: int64(payload.Page.PageIndex), CampaignID: 10, UnitID: 20}},
			},
		})
	}))
	t.Cleanup(server.Close)
	client, err := newDaemonClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	creativities, err := client.listAllActiveCreativities(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if len(creativities) != 2 || fmt.Sprint(requestedPages) != "[1 2]" {
		t.Fatalf("creativities/pages = %+v/%v", creativities, requestedPages)
	}
}
