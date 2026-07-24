package store

import (
	"testing"
	"time"
)

func TestParseXHSCampaignTimes(t *testing.T) {
	timestamp, err := parseXHSCampaignTime("2026-07-21 13:14:15")
	if err != nil {
		t.Fatal(err)
	}
	if timestamp == nil || timestamp.Format(time.RFC3339) != "2026-07-21T13:14:15+08:00" {
		t.Fatalf("timestamp = %v", timestamp)
	}
	date, err := parseXHSCampaignDate("2919-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if date == nil || date.Format(time.DateOnly) != "2919-01-01" {
		t.Fatalf("date = %v", date)
	}
	if empty, err := parseXHSCampaignTime("  "); err != nil || empty != nil {
		t.Fatalf("empty timestamp = %v, error = %v", empty, err)
	}
}
