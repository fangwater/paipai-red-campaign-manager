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

func TestListAllUnitsUsesTopLevelPaginationAndPreservesRawPayload(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != unitListPath {
			t.Errorf("path = %s", request.URL.Path)
		}
		var payload UnitListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requestedPages = append(requestedPages, payload.Page)
		count := 100
		if payload.Page == 2 {
			count = 1
		}
		units := make([]map[string]any, count)
		for index := range units {
			id := int64((payload.Page-1)*100 + index + 1)
			units[index] = map[string]any{
				"id": id, "campaign_id": int64(100), "name": fmt.Sprintf("unit-%d", id),
				"unit_filter_state": 10, "target_config": map[string]any{"target_gender": "all"},
			}
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0, "success": true,
			"data": map[string]any{"total_count": 101, "unit_infos": units},
		})
	}))

	client := newCampaignTestClient(t, upstream)
	result, err := client.ListAllUnits(context.Background(), "access-token", UnitListRequest{AdvertiserID: 123})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 101 || fmt.Sprint(requestedPages) != "[1 2]" {
		t.Fatalf("units/pages = %d/%v", len(result.Units), requestedPages)
	}
	raw, err := json.Marshal(result.Units[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"target_config"`) {
		t.Fatalf("raw payload lost unknown fields: %s", raw)
	}
}

func TestListAllCreativitiesUsesNestedPaginationAndPreservesRawPayload(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != creativitySearchPath {
			t.Errorf("path = %s", request.URL.Path)
		}
		var payload CreativityListRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requestedPages = append(requestedPages, payload.Page.PageIndex)
		count := 100
		if payload.Page.PageIndex == 2 {
			count = 1
		}
		creativities := make([]map[string]any, count)
		for index := range creativities {
			id := int64((payload.Page.PageIndex-1)*100 + index + 1)
			creativities[index] = map[string]any{
				"advertiser_id": int64(123), "campaign_id": int64(100), "unit_id": int64(200),
				"creativity_id": id, "creativity_name": fmt.Sprintf("creativity-%d", id),
				"creativity_filter_state": 8, "audit_comment": map[string]string{"reason": "ok"},
			}
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0, "success": true,
			"data": map[string]any{
				"page":            map[string]any{"page_index": payload.Page.PageIndex, "total_count": 101},
				"creativity_dtos": creativities,
			},
		})
	}))

	client := newCampaignTestClient(t, upstream)
	status := 2
	result, err := client.ListAllCreativities(context.Background(), "access-token", CreativityListRequest{
		AdvertiserID: 123, Status: &status,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Creativities) != 101 || fmt.Sprint(requestedPages) != "[1 2]" {
		t.Fatalf("creativities/pages = %d/%v", len(result.Creativities), requestedPages)
	}
	raw, err := json.Marshal(result.Creativities[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"audit_comment"`) {
		t.Fatalf("raw payload lost unknown fields: %s", raw)
	}
}

func TestDeliveryRequestsRejectInvalidValues(t *testing.T) {
	client := newCampaignTestClient(t, httptest.NewServer(http.NotFoundHandler()))
	invalidUnitStatus := 9
	if _, err := client.ListUnits(context.Background(), "access-token", UnitListRequest{AdvertiserID: 123, Status: &invalidUnitStatus}); !strings.Contains(fmt.Sprint(err), ErrInvalidUnitRequest.Error()) {
		t.Fatalf("unit error = %v", err)
	}
	invalidCreativityStatus := 6
	if _, err := client.ListCreativities(context.Background(), "access-token", CreativityListRequest{AdvertiserID: 123, Status: &invalidCreativityStatus}); !strings.Contains(fmt.Sprint(err), ErrInvalidCreativityRequest.Error()) {
		t.Fatalf("creativity error = %v", err)
	}
}
