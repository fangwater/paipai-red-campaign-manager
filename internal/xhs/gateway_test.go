package xhs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayForcesCampaignPausedAndUsesAllowlistedPath(t *testing.T) {
	var received map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/open/jg/campaign/create" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Access-Token") != "access-token" {
			t.Fatalf("access token header = %q", request.Header.Get("Access-Token"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true,"code":0,"request_id":"req-1","data":{"campaign_id":42}}`))
	}))
	defer upstream.Close()
	client, err := NewClient(1, "secret", WithBaseURL(upstream.URL), WithHTTPClient(upstream.Client()))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CallGateway(context.Background(), "access-token", OperationCampaignCreate, json.RawMessage(`{"advertiser_id":1234,"campaign_name":"test","enable":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if received["enable"] != float64(0) || received["advertiser_id"] != float64(1234) {
		t.Fatalf("campaign payload = %+v", received)
	}
	if result.RequestID != "req-1" || string(result.Data) != `{"campaign_id":42}` {
		t.Fatalf("gateway result = %+v", result)
	}
}

func TestGatewayRejectsUnknownOperationAndInvalidAdvertiser(t *testing.T) {
	client, err := NewClient(1, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CallGateway(context.Background(), "access", GatewayOperation("arbitrary.proxy"), json.RawMessage(`{"advertiser_id":1}`)); err == nil {
		t.Fatal("gateway accepted an unknown operation")
	}
	if _, err := client.CallGateway(context.Background(), "access", OperationTargetOptions, json.RawMessage(`{"advertiser_id":0}`)); err == nil {
		t.Fatal("gateway accepted an invalid advertiser")
	}
}
