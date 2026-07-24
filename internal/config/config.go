package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultLarkSyncListen          = "127.0.0.1:18081"
	defaultTimeout                 = 10 * time.Minute
	defaultDocumentRefreshInterval = time.Hour
)

type Config struct {
	LarkAppID               string
	LarkAppSecret           string
	LarkAppToken            string
	DatabaseURL             string
	LarkSyncListen          string
	SyncTimeout             time.Duration
	DocumentRefreshInterval time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		LarkAppID:               os.Getenv("LARK_APP_ID"),
		LarkAppSecret:           os.Getenv("LARK_APP_SECRET"),
		LarkAppToken:            os.Getenv("LARK_APP_TOKEN"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		LarkSyncListen:          envOrDefault("LARK_SYNC_LISTEN", defaultLarkSyncListen),
		SyncTimeout:             defaultTimeout,
		DocumentRefreshInterval: defaultDocumentRefreshInterval,
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

	missing := make([]string, 0, 4)
	for name, value := range map[string]string{
		"LARK_APP_ID":     cfg.LarkAppID,
		"LARK_APP_SECRET": cfg.LarkAppSecret,
		"LARK_APP_TOKEN":  cfg.LarkAppToken,
		"DATABASE_URL":    cfg.DatabaseURL,
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
