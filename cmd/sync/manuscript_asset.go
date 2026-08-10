package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
)

const manuscriptAssetPrefix = "/v1/manuscript-assets/"

func (server *apiServer) manuscriptAsset(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
		return
	}
	assetID := strings.TrimPrefix(request.URL.Path, manuscriptAssetPrefix)
	if !validManuscriptAssetID(assetID) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid manuscript asset id"})
		return
	}
	if server.manuscriptAssets == nil {
		writeJSON(writer, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "manuscript asset store is unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	asset, found, err := server.manuscriptAssets.ManuscriptAsset(ctx, assetID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, apiResponse{Success: false, Error: "manuscript asset was not found"})
		return
	}

	writer.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	writer.Header().Set("Content-Type", asset.ContentType)
	writer.Header().Set("Content-Disposition", "inline")
	writer.Header().Set("ETag", `"`+asset.AssetID+`"`)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(writer, request, asset.AssetID, asset.CreatedAt, bytes.NewReader(asset.Content))
}

func validManuscriptAssetID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
