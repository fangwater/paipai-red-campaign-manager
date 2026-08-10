package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type maituoAnalyticsStub struct {
	query                     maituo.NoteCampaignAnalysisQuery
	result                    maituo.NoteCampaignAnalysis
	contentNoteID             string
	contentResult             maituo.NoteContent
	materialsQuery            maituo.ReferenceMaterialsQuery
	materialsResult           maituo.ReferenceMaterials
	referenceContentNoteID    string
	referenceContentInput     maituo.ReferenceMaterialContentInput
	referenceContentResult    maituo.ReferenceMaterialContent
	referenceContentFound     bool
	comparisonQuery           maituo.TrafficComparisonQuery
	comparisonResult          maituo.TrafficComparison
	deliveryQuery             maituo.TrafficDeliveryComparisonQuery
	deliveryResult            maituo.TrafficDeliveryComparison
	diagnosisSPU              string
	diagnosisResult           maituo.AccountPlanDiagnosis
	calls                     int
	contentCalls              int
	materialsCalls            int
	referenceContentCalls     int
	referenceContentSaveCalls int
	comparisonCalls           int
	deliveryCalls             int
	diagnosisCalls            int
}

func (stub *maituoAnalyticsStub) MaituoNoteContent(_ context.Context, noteID string) (maituo.NoteContent, error) {
	stub.contentCalls++
	stub.contentNoteID = noteID
	return stub.contentResult, nil
}

func (stub *maituoAnalyticsStub) MaituoReferenceMaterials(_ context.Context, query maituo.ReferenceMaterialsQuery) (maituo.ReferenceMaterials, error) {
	stub.materialsCalls++
	stub.materialsQuery = query
	return stub.materialsResult, nil
}

func (stub *maituoAnalyticsStub) MaituoReferenceMaterialContent(_ context.Context, noteID string) (maituo.ReferenceMaterialContent, error) {
	stub.referenceContentCalls++
	stub.referenceContentNoteID = noteID
	return stub.referenceContentResult, nil
}

func (stub *maituoAnalyticsStub) SaveMaituoReferenceMaterialContent(_ context.Context, input maituo.ReferenceMaterialContentInput) (maituo.ReferenceMaterialContent, bool, error) {
	stub.referenceContentSaveCalls++
	stub.referenceContentInput = input
	return stub.referenceContentResult, stub.referenceContentFound, nil
}

func (stub *maituoAnalyticsStub) MaituoAccountPlanDiagnosis(_ context.Context, spu string) (maituo.AccountPlanDiagnosis, error) {
	stub.diagnosisCalls++
	stub.diagnosisSPU = spu
	return stub.diagnosisResult, nil
}

func (stub *maituoAnalyticsStub) MaituoNoteCampaignAnalysis(_ context.Context, query maituo.NoteCampaignAnalysisQuery) (maituo.NoteCampaignAnalysis, error) {
	stub.calls++
	stub.query = query
	return stub.result, nil
}

func (stub *maituoAnalyticsStub) MaituoTrafficComparison(_ context.Context, query maituo.TrafficComparisonQuery) (maituo.TrafficComparison, error) {
	stub.comparisonCalls++
	stub.comparisonQuery = query
	return stub.comparisonResult, nil
}

func (stub *maituoAnalyticsStub) MaituoTrafficDeliveryComparison(_ context.Context, query maituo.TrafficDeliveryComparisonQuery) (maituo.TrafficDeliveryComparison, error) {
	stub.deliveryCalls++
	stub.deliveryQuery = query
	return stub.deliveryResult, nil
}

func TestMaituoAccountPlanDiagnosis(t *testing.T) {
	stub := &maituoAnalyticsStub{diagnosisResult: maituo.AccountPlanDiagnosis{ReportDate: "2026-07-27", SPU: "辅酶"}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/account-plan-diagnosis", nil)
	response := httptest.NewRecorder()

	server.maituoAccountPlanDiagnosis(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.diagnosisCalls != 1 || stub.diagnosisSPU != "辅酶" {
		t.Fatalf("calls = %d, spu = %q", stub.diagnosisCalls, stub.diagnosisSPU)
	}
}

func TestMaituoNoteCampaignAnalysis(t *testing.T) {
	stub := &maituoAnalyticsStub{result: maituo.NoteCampaignAnalysis{Window: "3d", Total: 1}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?window=3d&q=campaign&plan_id=211253819&sort=daily_spend&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.query.Window != "3d" || stub.query.Search != "campaign" || stub.query.PlanID != "211253819" || stub.query.Sort != "daily_spend" || stub.query.Page != 2 || stub.query.PageSize != 40 {
		t.Fatalf("query = %+v", stub.query)
	}
	var payload apiResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestMaituoNoteContent(t *testing.T) {
	stub := &maituoAnalyticsStub{contentResult: maituo.NoteContent{
		NoteID: "6a33d5aa000000001c024f8a", Found: true, NoteContent: "稿件正文",
	}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-content?note_id=6a33d5aa000000001c024f8a", nil)
	response := httptest.NewRecorder()

	server.maituoNoteContent(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.contentCalls != 1 || stub.contentNoteID != "6a33d5aa000000001c024f8a" {
		t.Fatalf("calls = %d, note_id = %q", stub.contentCalls, stub.contentNoteID)
	}
}

func TestMaituoNoteContentRequiresNoteID(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-content", nil)
	response := httptest.NewRecorder()

	server.maituoNoteContent(response, request)
	if response.Code != http.StatusBadRequest || stub.contentCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.contentCalls)
	}
}

func TestMaituoNoteCampaignAnalysisRejectsInvalidSort(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?sort=search_cost", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.calls)
	}
}

func TestMaituoNoteCampaignAnalysisRejectsInvalidWindow(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/note-campaigns?window=30d", nil)
	response := httptest.NewRecorder()

	server.maituoNoteCampaignAnalysis(response, request)
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.calls)
	}
}

func TestMaituoTrafficComparison(t *testing.T) {
	stub := &maituoAnalyticsStub{comparisonResult: maituo.TrafficComparison{Window: "3d", Total: 2}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/traffic-comparisons?window=3d&q=note&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.maituoTrafficComparison(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.comparisonCalls != 1 || stub.comparisonQuery.Window != "3d" || stub.comparisonQuery.Search != "note" || stub.comparisonQuery.Page != 2 || stub.comparisonQuery.PageSize != 40 {
		t.Fatalf("calls = %d, query = %+v", stub.comparisonCalls, stub.comparisonQuery)
	}
}

func TestMaituoTrafficComparisonRejectsInvalidWindow(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/traffic-comparisons?window=30d", nil)
	response := httptest.NewRecorder()

	server.maituoTrafficComparison(response, request)
	if response.Code != http.StatusBadRequest || stub.comparisonCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.comparisonCalls)
	}
}

func TestMaituoTrafficDeliveryComparison(t *testing.T) {
	stub := &maituoAnalyticsStub{deliveryResult: maituo.TrafficDeliveryComparison{NoteID: "note-1", Placement: "搜索"}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/traffic-comparison-delivery?note_id=note-1&placement=%E6%90%9C%E7%B4%A2", nil)
	response := httptest.NewRecorder()

	server.maituoTrafficDeliveryComparison(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.deliveryCalls != 1 || stub.deliveryQuery.NoteID != "note-1" || stub.deliveryQuery.Placement != "搜索" {
		t.Fatalf("calls = %d, query = %+v", stub.deliveryCalls, stub.deliveryQuery)
	}
}

func TestMaituoTrafficDeliveryComparisonRejectsInvalidPlacement(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/traffic-comparison-delivery?note_id=note-1&placement=all", nil)
	response := httptest.NewRecorder()

	server.maituoTrafficDeliveryComparison(response, request)
	if response.Code != http.StatusBadRequest || stub.deliveryCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.deliveryCalls)
	}
}
