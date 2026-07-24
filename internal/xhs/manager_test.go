package xhs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenManagerStartsWithoutSessionAndAuthorizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code":0,
			"success":true,
			"msg":"成功",
			"data":{
				"access_token":"access",
				"access_token_expires_in":3600,
				"refresh_token":"refresh",
				"refresh_token_expires_in":2592000,
				"user_id":"user-1",
				"app_id":11344,
				"platform_type":1
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.json")
	manager, err := NewTokenManager(client, path)
	if err != nil {
		t.Fatal(err)
	}
	if manager.Status().Authorized {
		t.Fatal("manager unexpectedly started authorized")
	}
	if _, err := manager.Authorize(context.Background(), "auth-code"); err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	status := manager.Status()
	if !status.Authorized || !status.AccessTokenValid || status.UserID != "user-1" {
		t.Fatalf("status = %+v", status)
	}
	session, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if session.Token.RefreshToken != "refresh" {
		t.Fatalf("saved refresh token = %q", session.Token.RefreshToken)
	}
}

func TestTokenManagerRunRefreshesAndRetries(t *testing.T) {
	var requests atomic.Int32
	refreshed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload refreshTokenRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload.RefreshToken != "refresh-old" {
			t.Errorf("refresh token = %q", payload.RefreshToken)
		}
		attempt := requests.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			_, _ = writer.Write([]byte(`{"code":50001,"success":false,"msg":"temporary error"}`))
			return
		}
		_, _ = writer.Write([]byte(`{
			"code":0,
			"success":true,
			"msg":"成功",
			"data":{
				"access_token":"access-new",
				"access_token_expires_in":3600,
				"refresh_token":"refresh-new",
				"refresh_token_expires_in":2592000,
				"user_id":"user-1",
				"app_id":11344,
				"platform_type":1
			}
		}`))
		select {
		case refreshed <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(path, Token{
		AccessToken:           "access-old",
		RefreshToken:          "refresh-old",
		AccessTokenExpiresAt:  time.Now().Add(40 * time.Millisecond),
		RefreshTokenExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewTokenManager(client, path, WithRefreshBefore(30*time.Millisecond), WithRefreshRetry(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Run(ctx)

	select {
	case <-refreshed:
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not retry and refresh")
	}
	deadline := time.Now().Add(time.Second)
	for {
		session, loadErr := LoadSession(path)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if session.Token.AccessToken == "access-new" && session.Token.RefreshToken == "refresh-new" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("session was not rotated: %+v", session.Token)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if requests.Load() != 2 {
		t.Fatalf("refresh requests = %d, want 2", requests.Load())
	}
}
