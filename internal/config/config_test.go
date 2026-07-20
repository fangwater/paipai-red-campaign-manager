package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("LARK_APP_ID", "app-id")
	t.Setenv("LARK_APP_SECRET", "secret")
	t.Setenv("LARK_APP_TOKEN", "app-token")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("SYNC_CRON", "0 * * * *")
	t.Setenv("SYNC_ON_START", "false")
	t.Setenv("SYNC_TIMEOUT", "30s")
	t.Setenv("DOCUMENT_REFRESH_INTERVAL", "45m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SyncOnStart {
		t.Fatal("SyncOnStart = true, want false")
	}
	if cfg.SyncTimeout != 30*time.Second {
		t.Fatalf("SyncTimeout = %s, want 30s", cfg.SyncTimeout)
	}
	if cfg.DocumentRefreshInterval != 45*time.Minute {
		t.Fatalf("DocumentRefreshInterval = %s, want 45m", cfg.DocumentRefreshInterval)
	}
	if cfg.SyncCron != "0 * * * *" {
		t.Fatalf("SyncCron = %q, want %q", cfg.SyncCron, "0 * * * *")
	}
}

func TestLoadRequiresCredentials(t *testing.T) {
	t.Setenv("LARK_APP_ID", "")
	t.Setenv("LARK_APP_SECRET", "")
	t.Setenv("LARK_APP_TOKEN", "")
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing environment variables error")
	}
}
