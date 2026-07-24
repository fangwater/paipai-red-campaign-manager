package xhs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExchangeToken(t *testing.T) {
	var received tokenRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/api/open/oauth2/access_token" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code": 0,
			"success": true,
			"msg": "成功",
			"data": {
				"access_token": "access",
				"access_token_expires_in": 86399,
				"refresh_token": "refresh",
				"refresh_token_expires_in": 2591999,
				"user_id": "user-1",
				"app_id": 11344,
				"approval_role_type": 4,
				"role_type": 1,
				"platform_type": 1,
				"approval_advertisers": [
					{"advertiser_id": 1234, "advertiser_name": "测试广告主"}
				],
				"scope": "report"
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	token, err := client.ExchangeToken(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("ExchangeToken() error = %v", err)
	}
	if received.AppID != 11344 || received.Secret != "secret" || received.AuthCode != "auth-code" {
		t.Fatalf("request = %+v", received)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" {
		t.Fatalf("tokens were not decoded")
	}
	if token.AccessTokenExpiresAt != now.Add(86399*time.Second) {
		t.Fatalf("AccessTokenExpiresAt = %s", token.AccessTokenExpiresAt)
	}
	if len(token.ApprovalAdvertisers) != 1 || token.ApprovalAdvertisers[0].ID != 1234 {
		t.Fatalf("ApprovalAdvertisers = %+v", token.ApprovalAdvertisers)
	}
}

func TestExchangeTokenReturnsAPIErrorWithoutLeakingSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code": 40001, "success": false, "msg": "auth code expired"}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "do-not-leak", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.ExchangeToken(context.Background(), "expired")
	if err == nil || !strings.Contains(err.Error(), "code=40001") {
		t.Fatalf("ExchangeToken() error = %v", err)
	}
	if strings.Contains(err.Error(), "do-not-leak") {
		t.Fatal("error leaked application secret")
	}
}

func TestExchangeTokenValidatesInputs(t *testing.T) {
	if _, err := NewClient(0, "secret"); err == nil {
		t.Fatal("NewClient() accepted zero app ID")
	}
	if _, err := NewClient(11344, ""); err == nil {
		t.Fatal("NewClient() accepted empty secret")
	}
	client, err := NewClient(11344, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ExchangeToken(context.Background(), " "); err == nil {
		t.Fatal("ExchangeToken() accepted empty auth code")
	}
}
func TestRefreshToken(t *testing.T) {
	var received refreshTokenRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/api/open/oauth2/refresh_token" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code": 0,
			"success": true,
			"msg": "成功",
			"data": {
				"access_token": "access-new",
				"access_token_expires_in": 86399,
				"refresh_token": "refresh-new",
				"refresh_token_expires_in": 2591999,
				"user_id": "user-1",
				"app_id": 11344,
				"approval_role_type": 4,
				"role_type": 1,
				"platform_type": 1,
				"approval_advertisers": [
					{"advertiser_id": 1234, "advertiser_name": "测试广告主"}
				],
				"scope": "report"
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	token, err := client.RefreshToken(context.Background(), "refresh-old")
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}
	if received.AppID != 11344 || received.Secret != "secret" || received.RefreshToken != "refresh-old" {
		t.Fatalf("request = %+v", received)
	}
	if token.AccessToken != "access-new" || token.RefreshToken != "refresh-new" {
		t.Fatalf("tokens were not rotated: %+v", token)
	}
	if token.RefreshTokenExpiresAt != now.Add(2591999*time.Second) {
		t.Fatalf("RefreshTokenExpiresAt = %s", token.RefreshTokenExpiresAt)
	}
}

func TestRefreshTokenRejectsEmptyToken(t *testing.T) {
	client, err := NewClient(11344, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.RefreshToken(context.Background(), " "); err == nil {
		t.Fatal("RefreshToken() accepted an empty refresh token")
	}
}
