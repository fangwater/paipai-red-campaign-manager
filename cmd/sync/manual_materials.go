package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"

	"golang.org/x/image/webp"
)

const (
	maxManualMaterialTitleRunes   = 200
	maxManualMaterialBodyRunes    = 20000
	maxManualMaterialComments     = 20
	maxManualMaterialCommentRunes = 500
	maxManualMaterialImages       = 9
	maxManualMaterialImageBytes   = 10 * 1024 * 1024
	maxManualMaterialRequestBytes = 96 * 1024 * 1024
)

type manualMaterialStore interface {
	CreateManualMaterial(context.Context, maituo.ManualMaterialInput) (maituo.ManualMaterial, error)
	UpdateManualMaterial(context.Context, maituo.ManualMaterialInput) (maituo.ManualMaterial, bool, error)
	UpdateManualMaterialTags(context.Context, string, maituo.ManualMaterialTags) (maituo.ManualMaterial, bool, error)
	ManualMaterial(context.Context, string) (maituo.ManualMaterial, bool, error)
	ManualMaterials(context.Context, maituo.ManualMaterialsQuery) (maituo.ManualMaterials, error)
}

type manualMaterialTagsRequest struct {
	NoteType            string `json:"note_type"`
	CoverType           string `json:"cover_type"`
	CommercialIntensity string `json:"commercial_intensity"`
	Audience            string `json:"audience"`
	UserScenario        string `json:"user_scenario"`
}

func (server *apiServer) maituoManualMaterials(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		server.listMaituoManualMaterials(writer, request)
	case http.MethodPost:
		server.createMaituoManualMaterial(writer, request)
	default:
		writer.Header().Set("Allow", "GET, POST")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
	}
}

func (server *apiServer) maituoManualMaterial(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		server.getMaituoManualMaterial(writer, request)
	case http.MethodPut:
		server.updateMaituoManualMaterial(writer, request)
	default:
		writer.Header().Set("Allow", "GET, PUT")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
	}
}

func (server *apiServer) listMaituoManualMaterials(writer http.ResponseWriter, request *http.Request) {
	if server.manualMaterials == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "素材服务未配置"})
		return
	}
	page, ok := positiveQueryInt(request, "page", 1, 1, 100000)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "page 必须是正整数"})
		return
	}
	pageSize, ok := positiveQueryInt(request, "page_size", 20, 1, 100)
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "page_size 必须是 1 到 100 的整数"})
		return
	}
	search := strings.TrimSpace(request.URL.Query().Get("q"))
	if len([]rune(search)) > 200 {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "搜索内容不能超过 200 个字符"})
		return
	}
	untagged := false
	switch strings.ToLower(strings.TrimSpace(request.URL.Query().Get("untagged"))) {
	case "", "0", "false":
	case "1", "true":
		untagged = true
	default:
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "untagged 仅支持 true 或 false"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.manualMaterials.ManualMaterials(ctx, maituo.ManualMaterialsQuery{
		Search: search, Untagged: untagged, Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) getMaituoManualMaterial(writer http.ResponseWriter, request *http.Request) {
	if server.manualMaterials == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "素材服务未配置"})
		return
	}
	materialID, ok := normalizeManualMaterialID(request.URL.Query().Get("material_id"))
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "material_id 必须是 32 位素材编号"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, found, err := server.manualMaterials.ManualMaterial(ctx, materialID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "素材不存在"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) createMaituoManualMaterial(writer http.ResponseWriter, request *http.Request) {
	if server.manualMaterials == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "素材服务未配置"})
		return
	}
	input, status, message := parseManualMaterialForm(request, "")
	if message != "" {
		writeJSON(writer, status, apiResponse{Success: false, Error: message})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, err := server.manualMaterials.CreateManualMaterial(ctx, input)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusCreated, apiResponse{Success: true, Data: result})
}

func (server *apiServer) updateMaituoManualMaterial(writer http.ResponseWriter, request *http.Request) {
	if server.manualMaterials == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "素材服务未配置"})
		return
	}
	materialID, ok := normalizeManualMaterialID(request.URL.Query().Get("material_id"))
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "material_id 必须是 32 位素材编号"})
		return
	}
	input, status, message := parseManualMaterialForm(request, materialID)
	if message != "" {
		writeJSON(writer, status, apiResponse{Success: false, Error: message})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, found, err := server.manualMaterials.UpdateManualMaterial(ctx, input)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "素材不存在"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func (server *apiServer) maituoManualMaterialTags(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		methodNotAllowed(writer, http.MethodPut)
		return
	}
	if server.manualMaterials == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "素材服务未配置"})
		return
	}
	materialID, ok := normalizeManualMaterialID(request.URL.Query().Get("material_id"))
	if !ok {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "material_id 必须是 32 位素材编号"})
		return
	}
	var payload manualMaterialTagsRequest
	if !decodeOptionalJSON(writer, request, &payload) {
		return
	}
	tags := maituo.ManualMaterialTags{
		NoteType:            strings.TrimSpace(payload.NoteType),
		CoverType:           strings.TrimSpace(payload.CoverType),
		CommercialIntensity: strings.TrimSpace(payload.CommercialIntensity),
		Audience:            strings.TrimSpace(payload.Audience),
		UserScenario:        strings.TrimSpace(payload.UserScenario),
	}
	for _, value := range []string{tags.NoteType, tags.CoverType, tags.CommercialIntensity, tags.Audience, tags.UserScenario} {
		if len([]rune(value)) > 100 {
			writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "标签不能超过 100 个字符"})
			return
		}
	}
	if !tags.Complete() {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "请完整填写全部标签"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	result, found, err := server.manualMaterials.UpdateManualMaterialTags(ctx, materialID, tags)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "素材不存在"})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
}

func parseManualMaterialForm(request *http.Request, materialID string) (maituo.ManualMaterialInput, int, string) {
	request.Body = http.MaxBytesReader(nil, request.Body, maxManualMaterialRequestBytes)
	if err := request.ParseMultipartForm(12 << 20); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return maituo.ManualMaterialInput{}, http.StatusRequestEntityTooLarge, "上传内容不能超过 96 MB"
		}
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "上传请求格式错误"
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}

	title := strings.TrimSpace(request.FormValue("title"))
	if title == "" {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "标题不能为空"
	}
	if len([]rune(title)) > maxManualMaterialTitleRunes {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "标题不能超过 200 个字符"
	}
	body := strings.TrimSpace(request.FormValue("body"))
	if body == "" {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "正文不能为空"
	}
	if len([]rune(body)) > maxManualMaterialBodyRunes {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "正文不能超过 20000 个字符"
	}
	comments, err := parseManualMaterialComments(request.FormValue("comments"))
	if err != nil {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, err.Error()
	}
	existingIDs, err := parseExistingImageIDs(request.FormValue("existing_image_ids"))
	if err != nil {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, err.Error()
	}
	uploaded, err := parseUploadedManualImages(request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errManualMaterialImageTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		return maituo.ManualMaterialInput{}, status, err.Error()
	}
	if len(existingIDs)+len(uploaded) == 0 {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "至少上传一张图片"
	}
	if len(existingIDs)+len(uploaded) > maxManualMaterialImages {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, "图片不能超过 9 张"
	}
	noteID, noteURL, err := parseManualMaterialNote(request.FormValue("note_id"), request.FormValue("note_url"))
	if err != nil {
		return maituo.ManualMaterialInput{}, http.StatusBadRequest, err.Error()
	}
	if materialID == "" {
		generated, generateErr := newManualMaterialID()
		if generateErr != nil {
			return maituo.ManualMaterialInput{}, http.StatusInternalServerError, "生成素材编号失败"
		}
		materialID = generated
	}
	return maituo.ManualMaterialInput{
		MaterialID:       materialID,
		NoteID:           noteID,
		NoteURL:          noteURL,
		Title:            title,
		Body:             body,
		Comments:         comments,
		ExistingImageIDs: existingIDs,
		UploadedImages:   uploaded,
	}, 0, ""
}

func parseManualMaterialNote(rawNoteID, rawNoteURL string) (string, string, error) {
	noteID := strings.ToLower(strings.TrimSpace(rawNoteID))
	noteURL := strings.TrimSpace(rawNoteURL)
	if extracted, ok := extractXiaohongshuNoteID(noteURL); ok && noteID == "" {
		noteID = extracted
	}
	if noteID == "" {
		return "", "", errors.New("笔记 ID 不能为空")
	}
	normalized, ok := normalizeReferenceNoteID(noteID)
	if !ok {
		return "", "", errors.New("笔记 ID 必须是 24 位小红书笔记 ID")
	}
	if noteURL == "" {
		return normalized, "https://www.xiaohongshu.com/explore/" + normalized, nil
	}
	if len([]rune(noteURL)) > 500 {
		return "", "", errors.New("笔记链接不能超过 500 个字符")
	}
	extracted, ok := extractXiaohongshuNoteID(noteURL)
	if !ok {
		return "", "", errors.New("笔记链接必须是小红书笔记地址")
	}
	if extracted != normalized {
		return "", "", errors.New("笔记链接与笔记 ID 不一致")
	}
	return normalized, noteURL, nil
}

func extractXiaohongshuNoteID(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	normalized := strings.ToLower(trimmed)
	markers := []string{"xiaohongshu.com/explore/", "xiaohongshu.com/discovery/item/"}
	matched := false
	for _, marker := range markers {
		if index := strings.Index(normalized, marker); index >= 0 {
			normalized = normalized[index+len(marker):]
			matched = true
			break
		}
	}
	if !matched {
		return "", false
	}
	if query := strings.IndexAny(normalized, "?#/"); query >= 0 {
		normalized = normalized[:query]
	}
	return normalizeReferenceNoteID(normalized)
}

func parseManualMaterialComments(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, errors.New("评论必须是字符串数组")
	}
	comments := make([]string, 0, len(values))
	for _, value := range values {
		comment := strings.TrimSpace(value)
		if comment == "" {
			continue
		}
		if len([]rune(comment)) > maxManualMaterialCommentRunes {
			return nil, fmt.Errorf("单条评论不能超过 %d 个字符", maxManualMaterialCommentRunes)
		}
		comments = append(comments, comment)
	}
	if len(comments) > maxManualMaterialComments {
		return nil, fmt.Errorf("评论不能超过 %d 条", maxManualMaterialComments)
	}
	return comments, nil
}

func parseExistingImageIDs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, errors.New("已有图片编号格式错误")
	}
	ids := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		assetID := strings.ToLower(strings.TrimSpace(value))
		if assetID == "" {
			continue
		}
		if !validManuscriptAssetID(assetID) {
			return nil, errors.New("已有图片编号无效")
		}
		if _, exists := seen[assetID]; exists {
			continue
		}
		seen[assetID] = struct{}{}
		ids = append(ids, assetID)
	}
	return ids, nil
}

var errManualMaterialImageTooLarge = errors.New("单张图片不能超过 10 MB")

func parseUploadedManualImages(request *http.Request) ([]maituo.ManualMaterialImageInput, error) {
	if request.MultipartForm == nil {
		return []maituo.ManualMaterialImageInput{}, nil
	}
	headers := request.MultipartForm.File["images"]
	images := make([]maituo.ManualMaterialImageInput, 0, len(headers))
	for _, header := range headers {
		if header.Size > maxManualMaterialImageBytes {
			return nil, errManualMaterialImageTooLarge
		}
		file, err := header.Open()
		if err != nil {
			return nil, errors.New("读取图片失败")
		}
		content, err := io.ReadAll(io.LimitReader(file, maxManualMaterialImageBytes+1))
		file.Close()
		if err != nil {
			return nil, errors.New("读取图片失败")
		}
		if len(content) == 0 {
			return nil, errors.New("图片内容不能为空")
		}
		if len(content) > maxManualMaterialImageBytes {
			return nil, errManualMaterialImageTooLarge
		}
		contentType, width, height, err := decodeManualMaterialImage(content)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(content)
		images = append(images, maituo.ManualMaterialImageInput{
			AssetID:     hex.EncodeToString(digest[:]),
			ContentType: contentType,
			Width:       width,
			Height:      height,
			Content:     content,
		})
	}
	return images, nil
}

func decodeManualMaterialImage(content []byte) (string, int, int, error) {
	contentType := http.DetectContentType(content)
	switch contentType {
	case "image/jpeg", "image/png", "image/gif":
		config, _, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			return "", 0, 0, errors.New("图片无法解析")
		}
		if config.Width <= 0 || config.Height <= 0 {
			return "", 0, 0, errors.New("图片尺寸无效")
		}
		return contentType, config.Width, config.Height, nil
	case "image/webp":
		config, err := webp.DecodeConfig(bytes.NewReader(content))
		if err != nil {
			return "", 0, 0, errors.New("图片无法解析")
		}
		if config.Width <= 0 || config.Height <= 0 {
			return "", 0, 0, errors.New("图片尺寸无效")
		}
		return contentType, config.Width, config.Height, nil
	default:
		return "", 0, 0, errors.New("仅支持 JPEG、PNG、WebP 或 GIF 图片")
	}
}

func newManualMaterialID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func normalizeManualMaterialID(value string) (string, bool) {
	materialID := strings.ToLower(strings.TrimSpace(value))
	if len(materialID) != 32 {
		return "", false
	}
	for _, character := range materialID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", false
		}
	}
	return materialID, true
}
