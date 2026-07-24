package xhs

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "session.json")
	token := Token{
		AccessToken:           "access",
		RefreshToken:          "refresh",
		AccessTokenExpiresAt:  time.Now().Add(time.Hour),
		RefreshTokenExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	if err := SaveSession(path, token); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %o, want 600", info.Mode().Perm())
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(directory) error = %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o, want 700", directoryInfo.Mode().Perm())
	}

	session, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if session.Version != SessionVersion || session.Token.AccessToken != "access" || session.Token.RefreshToken != "refresh" {
		t.Fatalf("session = %+v", session)
	}
}

func TestSaveSessionDoesNotChangeExistingDirectoryMode(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "session.json")
	token := Token{AccessToken: "access", RefreshToken: "refresh"}
	if err := SaveSession(path, token); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("existing directory mode = %o, want 755", info.Mode().Perm())
	}
}

func TestLoadSessionRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"token":{"access_token":"a","refresh_token":"r"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(path); err == nil {
		t.Fatal("LoadSession() accepted an unknown version")
	}
}
func TestRefreshSessionRotatesPersistedTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(path, Token{AccessToken: "access-old", RefreshToken: "refresh-old"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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
				"platform_type": 1
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := client.RefreshSession(context.Background(), path)
	if err != nil {
		t.Fatalf("RefreshSession() error = %v", err)
	}
	session, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken != "refresh-new" || session.Token.AccessToken != "access-new" || session.Token.RefreshToken != "refresh-new" {
		t.Fatalf("session tokens were not rotated: %+v", session.Token)
	}
}

func TestRefreshSessionKeepsOldSessionOnAPIError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(path, Token{AccessToken: "access-old", RefreshToken: "refresh-old"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":40002,"success":false,"msg":"refresh token expired"}`))
	}))
	defer server.Close()

	client, err := NewClient(11344, "secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.RefreshSession(context.Background(), path); err == nil {
		t.Fatal("RefreshSession() error = nil")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("session file changed after a failed refresh")
	}
}
