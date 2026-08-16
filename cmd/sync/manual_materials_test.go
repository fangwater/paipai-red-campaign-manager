package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

const transparentPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

type manualMaterialStub struct {
	createInput maituo.ManualMaterialInput
	updateInput maituo.ManualMaterialInput
	listQuery   maituo.ManualMaterialsQuery
	getID       string
	created     maituo.ManualMaterial
	updated     maituo.ManualMaterial
	listed      maituo.ManualMaterials
	item        maituo.ManualMaterial
	found       bool
	createCalls int
	updateCalls int
	listCalls   int
	getCalls    int
}

func (stub *manualMaterialStub) CreateManualMaterial(_ context.Context, input maituo.ManualMaterialInput) (maituo.ManualMaterial, error) {
	stub.createCalls++
	stub.createInput = input
	result := stub.created
	if result.MaterialID == "" {
		result = maituo.ManualMaterial{
			MaterialID: input.MaterialID, Title: input.Title, Body: input.Body,
			Comments: input.Comments, ImageCount: len(input.UploadedImages) + len(input.ExistingImageIDs),
			CommentCount: len(input.Comments),
		}
	}
	return result, nil
}

func (stub *manualMaterialStub) UpdateManualMaterial(_ context.Context, input maituo.ManualMaterialInput) (maituo.ManualMaterial, bool, error) {
	stub.updateCalls++
	stub.updateInput = input
	return stub.updated, stub.found, nil
}

func (stub *manualMaterialStub) UpdateManualMaterialTags(_ context.Context, materialID string, tags maituo.ManualMaterialTags) (maituo.ManualMaterial, bool, error) {
	stub.updateCalls++
	stub.getID = materialID
	result := stub.updated
	result.MaterialID = materialID
	result.Tags = tags
	result.Tagged = tags.Complete()
	return result, stub.found, nil
}

func (stub *manualMaterialStub) ManualMaterial(_ context.Context, materialID string) (maituo.ManualMaterial, bool, error) {
	stub.getCalls++
	stub.getID = materialID
	return stub.item, stub.found, nil
}

func (stub *manualMaterialStub) ManualMaterials(_ context.Context, query maituo.ManualMaterialsQuery) (maituo.ManualMaterials, error) {
	stub.listCalls++
	stub.listQuery = query
	return stub.listed, nil
}

func decodeManualMaterialPNG() []byte {
	data, err := base64.StdEncoding.DecodeString(transparentPNG)
	if err != nil {
		panic(err)
	}
	return data
}

func newManualMaterialForm(fields map[string]string, files [][]byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			panic(err)
		}
	}
	for index, file := range files {
		part, err := writer.CreateFormFile("images", "image-"+strings.Repeat("0", index+1)+".png")
		if err != nil {
			panic(err)
		}
		if _, err := part.Write(file); err != nil {
			panic(err)
		}
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return body, writer.FormDataContentType()
}

func TestCreateManualMaterial(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title":    "  辅酶Q10早起实测  ",
		"body":     "  连续两周记录睡眠和精力变化。  ",
		"note_id":  "  6208DD8E000000002103E259  ",
		"note_url": "https://www.xiaohongshu.com/explore/6208DD8E000000002103E259",
		"comments": `["这条笔记帮到我了","想问剂量怎么选"]`,
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.createCalls != 1 {
		t.Fatalf("create calls = %d", stub.createCalls)
	}
	input := stub.createInput
	if input.Title != "辅酶Q10早起实测" || input.Body != "连续两周记录睡眠和精力变化。" ||
		input.NoteID != "6208dd8e000000002103e259" ||
		input.NoteURL != "https://www.xiaohongshu.com/explore/6208DD8E000000002103E259" ||
		len(input.Comments) != 2 || input.Comments[0] != "这条笔记帮到我了" ||
		len(input.UploadedImages) != 1 || len(input.MaterialID) != 32 {
		t.Fatalf("input = %+v", input)
	}
	if input.UploadedImages[0].ContentType != "image/png" ||
		input.UploadedImages[0].Width != 1 || input.UploadedImages[0].Height != 1 {
		t.Fatalf("image = %+v", input.UploadedImages[0])
	}
}

func TestCreateManualMaterialRejectsEmptyTitle(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title":   "   ",
		"body":    "正文",
		"note_id": "6208dd8e000000002103e259",
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusBadRequest || stub.createCalls != 0 {
		t.Fatalf("status = %d, calls = %d, body = %s", response.Code, stub.createCalls, response.Body.String())
	}
}

func TestCreateManualMaterialRejectsMissingImages(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title":   "标题",
		"body":    "正文",
		"note_id": "6208dd8e000000002103e259",
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusBadRequest || stub.createCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.createCalls)
	}
}

func TestCreateManualMaterialRejectsMissingNoteID(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title": "标题",
		"body":  "正文",
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusBadRequest || stub.createCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.createCalls)
	}
}

func TestCreateManualMaterialInfersNoteIDFromURL(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title":    "标题",
		"body":     "正文",
		"note_url": "https://www.xiaohongshu.com/explore/6208dd8e000000002103e259?xsec_token=abc",
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusCreated || stub.createInput.NoteID != "6208dd8e000000002103e259" {
		t.Fatalf("status = %d, input = %+v", response.Code, stub.createInput)
	}
}

func TestListManualMaterials(t *testing.T) {
	stub := &manualMaterialStub{listed: maituo.ManualMaterials{Total: 3, Page: 2}}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/manual-materials?q=辅酶&untagged=true&page=2&page_size=10", nil)
	response := httptest.NewRecorder()

	server.maituoManualMaterials(response, request)
	if response.Code != http.StatusOK || stub.listCalls != 1 ||
		stub.listQuery.Search != "辅酶" || !stub.listQuery.Untagged ||
		stub.listQuery.Page != 2 || stub.listQuery.PageSize != 10 {
		t.Fatalf("status = %d, query = %+v", response.Code, stub.listQuery)
	}
}

func TestGetManualMaterial(t *testing.T) {
	stub := &manualMaterialStub{
		found: true,
		item:  maituo.ManualMaterial{MaterialID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Title: "标题"},
	}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	request := httptest.NewRequest(http.MethodGet, "/v1/analytics/maituo/manual-material?material_id=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil)
	response := httptest.NewRecorder()

	server.maituoManualMaterial(response, request)
	if response.Code != http.StatusOK || stub.getCalls != 1 || stub.getID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("status = %d, id = %q", response.Code, stub.getID)
	}
}

func TestUpdateManualMaterialKeepsExistingImages(t *testing.T) {
	stub := &manualMaterialStub{
		found: true,
		updated: maituo.ManualMaterial{
			MaterialID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Title: "更新后的标题",
		},
	}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	existingID := strings.Repeat("ab", 32)
	body, contentType := newManualMaterialForm(map[string]string{
		"title":              "更新后的标题",
		"body":               "更新后的正文",
		"note_id":            "6208dd8e000000002103e259",
		"existing_image_ids": `["` + existingID + `"]`,
		"comments":           `["继续跟进"]`,
	}, nil)
	request := httptest.NewRequest(http.MethodPut, "/v1/analytics/maituo/manual-material?material_id=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterial(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stub.updateCalls != 1 || stub.updateInput.MaterialID != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
		stub.updateInput.NoteID != "6208dd8e000000002103e259" ||
		len(stub.updateInput.ExistingImageIDs) != 1 || stub.updateInput.ExistingImageIDs[0] != existingID {
		t.Fatalf("input = %+v", stub.updateInput)
	}
}

func TestUpdateManualMaterialReturnsNotFound(t *testing.T) {
	stub := &manualMaterialStub{}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	body, contentType := newManualMaterialForm(map[string]string{
		"title":   "标题",
		"body":    "正文",
		"note_id": "6208dd8e000000002103e259",
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPut, "/v1/analytics/maituo/manual-material?material_id=cccccccccccccccccccccccccccccccc", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	server.maituoManualMaterial(response, request)
	if response.Code != http.StatusNotFound || stub.updateCalls != 1 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.updateCalls)
	}
}

func TestCreateManualMaterialJSONResponse(t *testing.T) {
	stub := &manualMaterialStub{created: maituo.ManualMaterial{
		MaterialID: "dddddddddddddddddddddddddddddddd", Title: "标题", Body: "正文",
		Comments: []string{"评论"}, ImageCount: 1, CommentCount: 1,
	}}
	handler := newAPIHandler(&apiServer{manualMaterials: stub, timeout: time.Second})
	body, contentType := newManualMaterialForm(map[string]string{
		"title":    "标题",
		"body":     "正文",
		"note_id":  "6208dd8e000000002103e259",
		"comments": `["评论"]`,
	}, [][]byte{decodeManualMaterialPNG()})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/maituo/manual-materials", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload apiResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestUpdateManualMaterialTags(t *testing.T) {
	stub := &manualMaterialStub{found: true}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/analytics/maituo/manual-material-tags?material_id=dddddddddddddddddddddddddddddddd",
		strings.NewReader(`{"note_type":"科普","cover_type":"大字报","commercial_intensity":"软广","audience":"职场人","user_scenario":"精力疲惫"}`),
	)
	response := httptest.NewRecorder()

	server.maituoManualMaterialTags(response, request)
	if response.Code != http.StatusOK || stub.updateCalls != 1 || stub.getID != "dddddddddddddddddddddddddddddddd" {
		t.Fatalf("status = %d, id = %q, calls = %d", response.Code, stub.getID, stub.updateCalls)
	}
}

func TestUpdateManualMaterialTagsRejectsIncomplete(t *testing.T) {
	stub := &manualMaterialStub{found: true}
	server := &apiServer{manualMaterials: stub, timeout: time.Second}
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/analytics/maituo/manual-material-tags?material_id=dddddddddddddddddddddddddddddddd",
		strings.NewReader(`{"note_type":"科普"}`),
	)
	response := httptest.NewRecorder()

	server.maituoManualMaterialTags(response, request)
	if response.Code != http.StatusBadRequest || stub.updateCalls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, stub.updateCalls)
	}
}
