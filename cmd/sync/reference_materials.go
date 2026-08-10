package main

import (
	"context"
	"net/http"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
)

const maxReferenceMaterialContentRunes = 20000

type referenceMaterialContentRequest struct {
	ReferenceNoteID string `json:"reference_note_id"`
	NoteContent     string `json:"note_content"`
}

func (server *apiServer) maituoReferenceMaterials(writer http.ResponseWriter, request *http.Request) {
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
	result, err := server.maituoAnalytics.MaituoReferenceMaterials(ctx, maituo.ReferenceMaterialsQuery{
		Search: search, Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) maituoReferenceMaterialContent(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		server.getMaituoReferenceMaterialContent(writer, request)
	case http.MethodPut:
		server.putMaituoReferenceMaterialContent(writer, request)
	default:
		writer.Header().Set("Allow", "GET, PUT")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
	}
}

func (server *apiServer) getMaituoReferenceMaterialContent(writer http.ResponseWriter, request *http.Request) {
	noteID, ok := normalizeReferenceNoteID(request.URL.Query().Get("note_id"))
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "note_id 必须是 24 位小红书笔记 ID"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.maituoAnalytics.MaituoReferenceMaterialContent(ctx, noteID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) putMaituoReferenceMaterialContent(writer http.ResponseWriter, request *http.Request) {
	var payload referenceMaterialContentRequest
	if !decodeOptionalJSON(writer, request, &payload) {
		return
	}
	noteID, ok := normalizeReferenceNoteID(payload.ReferenceNoteID)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "reference_note_id 必须是 24 位小红书笔记 ID"})
		return
	}
	noteContent := strings.TrimSpace(payload.NoteContent)
	if noteContent == "" {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "素材内容不能为空"})
		return
	}
	if len([]rune(noteContent)) > maxReferenceMaterialContentRunes {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "素材内容不能超过 20000 个字符"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, found, err := server.maituoAnalytics.SaveMaituoReferenceMaterialContent(ctx, maituo.ReferenceMaterialContentInput{
		ReferenceNoteID: noteID,
		NoteContent:     noteContent,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "参考素材不存在"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func normalizeReferenceNoteID(value string) (string, bool) {
	noteID := strings.ToLower(strings.TrimSpace(value))
	if len(noteID) != 24 {
		return "", false
	}
	for _, character := range noteID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", false
		}
	}
	return noteID, true
}
