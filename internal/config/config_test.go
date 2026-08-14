package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("LARK_APP_ID", "app-id")
	t.Setenv("LARK_APP_SECRET", "secret")
	t.Setenv("LARK_APP_TOKEN", "app-token")
	t.Setenv("LARK_DANDELION_APP_TOKEN", "dandelion-app-token")
	t.Setenv("LARK_DANDELION_TABLE_ID", "tbl-dandelion")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("LARK_SYNC_LISTEN", "127.0.0.1:19081")
	t.Setenv("SYNC_TIMEOUT", "30s")
	t.Setenv("DOCUMENT_REFRESH_INTERVAL", "45m")
	t.Setenv("BAILIAN_API_KEY", "bailian-key")
	t.Setenv("BAILIAN_WORKSPACE_ID", "workspace-id")
	t.Setenv("BAILIAN_EMBEDDING_MODEL", "qwen3.7-text-embedding")
	t.Setenv("BAILIAN_EMBEDDING_DIMENSIONS", "768")
	t.Setenv("XHS_JG_INTERNAL_API_KEY", "internal-key")
	t.Setenv("DELIVERY_API_CREDENTIALS_JSON", `[{"key":"delivery-key","actor":"operator-1","role":"operator","advertiser_ids":[1234]}]`)
	t.Setenv("DELIVERY_MEDIA_WRITES_ENABLED", "true")
	t.Setenv("DELIVERY_LLM_MODEL", "test-llm")
	t.Setenv("DELIVERY_RANKER_URL", "https://ranker.example.test/predict")
	t.Setenv("DELIVERY_RANKER_MODEL", "test-ranker")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LarkDandelionAppToken != "dandelion-app-token" || cfg.LarkDandelionTableID != "tbl-dandelion" {
		t.Fatalf("dandelion config = %q / %q", cfg.LarkDandelionAppToken, cfg.LarkDandelionTableID)
	}
	if cfg.LarkSyncListen != "127.0.0.1:19081" {
		t.Fatalf("LarkSyncListen = %q", cfg.LarkSyncListen)
	}
	if cfg.SyncTimeout != 30*time.Second {
		t.Fatalf("SyncTimeout = %s, want 30s", cfg.SyncTimeout)
	}
	if cfg.DocumentRefreshInterval != 45*time.Minute {
		t.Fatalf("DocumentRefreshInterval = %s, want 45m", cfg.DocumentRefreshInterval)
	}
	if cfg.BailianAPIKey != "bailian-key" || cfg.BailianWorkspaceID != "workspace-id" || cfg.BailianDimensions != 768 {
		t.Fatalf("Bailian config = key:%t workspace:%q dimensions:%d", cfg.BailianAPIKey != "", cfg.BailianWorkspaceID, cfg.BailianDimensions)
	}
	if cfg.CoenzymeQ10WikiToken != defaultCoenzymeQ10WikiToken || cfg.CoenzymeQ10SheetID != defaultCoenzymeQ10SheetID || cfg.CoenzymeQ10SheetName != defaultCoenzymeQ10SheetName {
		t.Fatalf("coenzyme Q10 defaults = %q / %q / %q", cfg.CoenzymeQ10WikiToken, cfg.CoenzymeQ10SheetID, cfg.CoenzymeQ10SheetName)
	}
	if !cfg.DeliveryMediaWrites || cfg.DeliveryCredentialsJSON == "" || cfg.XHSJGInternalAPIKey != "internal-key" || cfg.DeliveryLLMModel != "test-llm" || cfg.DeliveryRankerModel != "test-ranker" {
		t.Fatalf("delivery config = %+v", cfg)
	}
}

func TestLoadRequiresCredentials(t *testing.T) {
	t.Setenv("LARK_APP_ID", "")
	t.Setenv("LARK_APP_SECRET", "")
	t.Setenv("LARK_APP_TOKEN", "")
	t.Setenv("LARK_DANDELION_APP_TOKEN", "")
	t.Setenv("LARK_DANDELION_TABLE_ID", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("XHS_JG_INTERNAL_API_KEY", "")
	t.Setenv("DELIVERY_API_CREDENTIALS_JSON", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing environment variables error")
	}
}

func TestLoadDoesNotRequireDeliveryAPICredentials(t *testing.T) {
	t.Setenv("LARK_APP_ID", "app-id")
	t.Setenv("LARK_APP_SECRET", "secret")
	t.Setenv("LARK_APP_TOKEN", "app-token")
	t.Setenv("LARK_DANDELION_APP_TOKEN", "dandelion-app-token")
	t.Setenv("LARK_DANDELION_TABLE_ID", "tbl-dandelion")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("XHS_JG_INTERNAL_API_KEY", "internal-key")
	t.Setenv("DELIVERY_API_CREDENTIALS_JSON", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DeliveryCredentialsJSON != "" {
		t.Fatalf("DeliveryCredentialsJSON = %q", cfg.DeliveryCredentialsJSON)
	}
}
