package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientEmbedUsesCompatibleEndpointAndResponseIndexes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/compatible-mode/v1/embeddings" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		var payload embeddingRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "test-model" || payload.Dimensions != 3 || len(payload.Input) != 2 {
			t.Errorf("payload = %+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data":[
				{"index":1,"embedding":[0,1,0]},
				{"index":0,"embedding":[1,0,0]}
			],
			"usage":{"total_tokens":17}
		}`))
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL+"/compatible-mode/v1", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	vectors, usage, err := client.Embed(context.Background(), []string{"first", "second"}, "test-model", 3)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if vectors[0][0] != 1 || vectors[1][1] != 1 || usage.TotalTokens != 17 {
		t.Fatalf("vectors = %v, usage = %+v", vectors, usage)
	}
}

func TestClientEmbedRejectsDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[{"index":0,"embedding":[1,0]}]}`))
	}))
	defer server.Close()
	client, err := NewClient("test-key", server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, _, err = client.Embed(context.Background(), []string{"first"}, "test-model", 3)
	if err == nil || !strings.Contains(err.Error(), "dimension 2") {
		t.Fatalf("Embed() error = %v", err)
	}
}

func TestClientEmbedReturnsStructuredAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"code":"invalid_request","message":"bad model"}}`))
	}))
	defer server.Close()
	client, err := NewClient("test-key", server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, _, err = client.Embed(context.Background(), []string{"first"}, "test-model", 3)
	if err == nil || !strings.Contains(err.Error(), "invalid_request") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("Embed() error = %v", err)
	}
}
