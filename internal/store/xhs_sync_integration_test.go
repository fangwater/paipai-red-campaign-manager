package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestXHSJGIncrementalSinceUnknownAdvertiserIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	postgres, err := NewPostgres(ctx, databaseURL, "xhs-jg-cursor-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	advertiserID := time.Now().UnixNano()
	defer func() {
		_, _ = postgres.pool.Exec(context.Background(), "DELETE FROM xhs_jg_advertisers WHERE advertiser_id=$1", advertiserID)
	}()

	lowerBound := time.Now().Add(-48*time.Hour - 2*time.Second)
	since, err := postgres.XHSJGIncrementalSince(ctx, advertiserID, "campaigns")
	if err != nil {
		t.Fatal(err)
	}
	upperBound := time.Now().Add(-48*time.Hour + 2*time.Second)
	if since.Before(lowerBound) || since.After(upperBound) {
		t.Fatalf("unknown advertiser cursor = %v, want two-day fallback between %v and %v", since, lowerBound, upperBound)
	}

	fullSyncedAt := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Microsecond)
	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO xhs_jg_advertisers (
			advertiser_id, advertiser_name, first_seen_at, last_seen_at, last_full_synced_at
		) VALUES ($1, 'cursor integration', NOW(), NOW(), $2)
	`, advertiserID, fullSyncedAt); err != nil {
		t.Fatal(err)
	}
	since, err = postgres.XHSJGIncrementalSince(ctx, advertiserID, "campaigns")
	if err != nil {
		t.Fatal(err)
	}
	if !since.Equal(fullSyncedAt) {
		t.Fatalf("stored advertiser cursor = %v, want %v", since, fullSyncedAt)
	}
}
