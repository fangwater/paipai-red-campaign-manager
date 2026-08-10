package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

type manuscriptAssetStub struct {
	asset maituo.ManuscriptAsset
	found bool
	calls int
}

func (stub *manuscriptAssetStub) ManuscriptAsset(context.Context, string) (maituo.ManuscriptAsset, bool, error) {
	stub.calls++
	return stub.asset, stub.found, nil
}

func TestManuscriptAssetEndpointServesImmutableImage(t *testing.T) {
	assetID := strings.Repeat("a", 64)
	content := []byte("image-content")
	stub := &manuscriptAssetStub{found: true, asset: maituo.ManuscriptAsset{
		AssetID: assetID, ContentType: "image/png", Content: content,
		CreatedAt: time.Date(2026, 8, 9, 9, 0, 0, 0, time.UTC),
	}}
	handler := newAPIHandler(&apiServer{manuscriptAssets: stub, timeout: time.Second})
	request := httptest.NewRequest(http.MethodGet, manuscriptAssetPrefix+assetID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), content) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.Bytes())
	}
	if got := response.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header().Get("ETag"); got != `"`+assetID+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "private, max-age=86400, immutable" {
		t.Fatalf("cache control = %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("store calls = %d", stub.calls)
	}
}

func TestManuscriptAssetEndpointRejectsInvalidIDBeforeQuery(t *testing.T) {
	stub := &manuscriptAssetStub{}
	handler := newAPIHandler(&apiServer{manuscriptAssets: stub, timeout: time.Second})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, manuscriptAssetPrefix+"not-a-hash", nil))
	if response.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, stub.calls, response.Body.String())
	}
}

func TestManuscriptAssetEndpointReturnsNotFound(t *testing.T) {
	assetID := strings.Repeat("b", 64)
	stub := &manuscriptAssetStub{}
	handler := newAPIHandler(&apiServer{manuscriptAssets: stub, timeout: time.Second})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, manuscriptAssetPrefix+assetID, nil))
	if response.Code != http.StatusNotFound || stub.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, stub.calls, response.Body.String())
	}
}
