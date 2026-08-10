package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoReferenceMaterials(t *testing.T) {
	stub := &maituoAnalyticsStub{materialsResult: maituo.ReferenceMaterials{Total: 268}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/reference-materials?q=智元&page=2&page_size=40", nil)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterials(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.materialsCalls != 1 || stub.materialsQuery.Search != "智元" ||
		stub.materialsQuery.Page != 2 || stub.materialsQuery.PageSize != 40 {
		t.Fatalf("calls = %d, query = %+v", stub.materialsCalls, stub.materialsQuery)
	}
}

func TestMaituoReferenceMaterialsRejectsInvalidPageSize(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/reference-materials?page_size=101", nil)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterials(response, request)
	if response.Code != http.StatusBadRequest || stub.materialsCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.materialsCalls)
	}
}

func TestMaituoReferenceMaterialContent(t *testing.T) {
	stub := &maituoAnalyticsStub{referenceContentResult: maituo.ReferenceMaterialContent{
		ReferenceNoteID: "abcdefabcdefabcdefabcdef",
		Found:           true,
		NoteContent:     "参考素材正文",
	}}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/reference-material-content?note_id=ABCDEFABCDEFABCDEFABCDEF", nil)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterialContent(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.referenceContentCalls != 1 || stub.referenceContentNoteID != "abcdefabcdefabcdefabcdef" {
		t.Fatalf("calls = %d, note_id = %q", stub.referenceContentCalls, stub.referenceContentNoteID)
	}
}

func TestPutMaituoReferenceMaterialContent(t *testing.T) {
	stub := &maituoAnalyticsStub{
		referenceContentFound: true,
		referenceContentResult: maituo.ReferenceMaterialContent{
			ReferenceNoteID: "6208dd8e000000002103e259",
			Found:           true,
			NoteContent:     "素材正文",
		},
	}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/analytics/maituo/reference-material-content",
		strings.NewReader(`{"reference_note_id":"6208DD8E000000002103E259","note_content":"  素材正文  "}`),
	)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterialContent(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.referenceContentSaveCalls != 1 ||
		stub.referenceContentInput.ReferenceNoteID != "6208dd8e000000002103e259" ||
		stub.referenceContentInput.NoteContent != "素材正文" {
		t.Fatalf("calls = %d, input = %+v", stub.referenceContentSaveCalls, stub.referenceContentInput)
	}
}

func TestPutMaituoReferenceMaterialContentRejectsEmptyContent(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/analytics/maituo/reference-material-content",
		strings.NewReader(`{"reference_note_id":"6208dd8e000000002103e259","note_content":"  "}`),
	)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterialContent(response, request)
	if response.Code != http.StatusBadRequest || stub.referenceContentSaveCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.referenceContentSaveCalls)
	}
}

func TestPutMaituoReferenceMaterialContentReturnsNotFound(t *testing.T) {
	stub := &maituoAnalyticsStub{}
	server := &apiServer{maituoAnalytics: stub, timeout: time.Second}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/analytics/maituo/reference-material-content",
		strings.NewReader(`{"reference_note_id":"6208dd8e000000002103e259","note_content":"素材正文"}`),
	)
	response := httptest.NewRecorder()

	server.maituoReferenceMaterialContent(response, request)
	if response.Code != http.StatusNotFound || stub.referenceContentSaveCalls != 1 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.referenceContentSaveCalls)
	}
}
