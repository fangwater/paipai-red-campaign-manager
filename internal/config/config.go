package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultLarkSyncListen          = "127.0.0.1:18081"
	defaultTimeout                 = 10 * time.Minute
	defaultDocumentRefreshInterval = time.Hour
	defaultBailianEmbeddingModel   = "qwen3.7-text-embedding"
	defaultBailianDimensions       = 1024
	defaultCoenzymeQ10WikiToken    = "WZlEwfr9dicJM1kwSVXcxhnUnDb"
	defaultCoenzymeQ10SheetID      = "a961f7"
	defaultCoenzymeQ10SheetName    = "辅酶q10日数据"
)

type Config struct {
	LarkAppID               string
	LarkAppSecret           string
	LarkAppToken            string
	LarkDandelionAppToken   string
	LarkDandelionTableID    string
	DatabaseURL             string
	LarkSyncListen          string
	SyncTimeout             time.Duration
	DocumentRefreshInterval time.Duration
	BailianAPIKey           string
	BailianWorkspaceID      string
	BailianBaseURL          string
	BailianEmbeddingModel   string
	BailianDimensions       int
	CoenzymeQ10WikiToken    string
	CoenzymeQ10SheetID      string
	CoenzymeQ10SheetName    string
}

func Load() (Config, error) {
	cfg := Config{
		LarkAppID:               os.Getenv("LARK_APP_ID"),
		LarkAppSecret:           os.Getenv("LARK_APP_SECRET"),
		LarkAppToken:            os.Getenv("LARK_APP_TOKEN"),
		LarkDandelionAppToken:   os.Getenv("LARK_DANDELION_APP_TOKEN"),
		LarkDandelionTableID:    os.Getenv("LARK_DANDELION_TABLE_ID"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		LarkSyncListen:          envOrDefault("LARK_SYNC_LISTEN", defaultLarkSyncListen),
		SyncTimeout:             defaultTimeout,
		DocumentRefreshInterval: defaultDocumentRefreshInterval,
		BailianAPIKey:           firstNonEmpty(os.Getenv("BAILIAN_API_KEY"), os.Getenv("DASHSCOPE_API_KEY")),
		BailianWorkspaceID:      os.Getenv("BAILIAN_WORKSPACE_ID"),
		BailianBaseURL:          os.Getenv("BAILIAN_BASE_URL"),
		BailianEmbeddingModel:   envOrDefault("BAILIAN_EMBEDDING_MODEL", defaultBailianEmbeddingModel),
		BailianDimensions:       defaultBailianDimensions,
		CoenzymeQ10WikiToken:    envOrDefault("LARK_COENZYME_Q10_WIKI_TOKEN", defaultCoenzymeQ10WikiToken),
		CoenzymeQ10SheetID:      envOrDefault("LARK_COENZYME_Q10_SHEET_ID", defaultCoenzymeQ10SheetID),
		CoenzymeQ10SheetName:    envOrDefault("LARK_COENZYME_Q10_SHEET_NAME", defaultCoenzymeQ10SheetName),
	}

	var err error
	cfg.SyncTimeout, err = positiveDuration("SYNC_TIMEOUT", cfg.SyncTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.DocumentRefreshInterval, err = positiveDuration("DOCUMENT_REFRESH_INTERVAL", cfg.DocumentRefreshInterval)
	if err != nil {
		return Config{}, err
	}
	cfg.BailianDimensions, err = positiveInt("BAILIAN_EMBEDDING_DIMENSIONS", cfg.BailianDimensions)
	if err != nil {
		return Config{}, err
	}

	missing := make([]string, 0, 6)
	for name, value := range map[string]string{
		"LARK_APP_ID":              cfg.LarkAppID,
		"LARK_APP_SECRET":          cfg.LarkAppSecret,
		"LARK_APP_TOKEN":           cfg.LarkAppToken,
		"LARK_DANDELION_APP_TOKEN": cfg.LarkDandelionAppToken,
		"LARK_DANDELION_TABLE_ID":  cfg.LarkDandelionTableID,
		"DATABASE_URL":             cfg.DatabaseURL,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

func positiveDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if value <= 0 {
		return 0, errors.New(name + " must be positive")
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func positiveInt(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New(name + " must be a positive integer")
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
