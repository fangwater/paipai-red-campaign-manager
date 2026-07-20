package main

import (
	"flag"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/guorai"
)

func TestRegisterFilterFlagsUpdatesReturnedFilter(t *testing.T) {
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	filter := registerFilterFlags(flags)
	if err := flags.Parse([]string{
		"--type", "plan", "--from", "2026-07-01", "--to", "2026-07-08",
	}); err != nil {
		t.Fatal(err)
	}
	if filter.BusinessType != guorai.BusinessTypePlan {
		t.Fatalf("business type = %q", filter.BusinessType)
	}
	if filter.BeginDate != "2026-07-01" || filter.EndDate != "2026-07-08" {
		t.Fatalf("date range = %s through %s", filter.BeginDate, filter.EndDate)
	}
}

func TestGuoraiRollingWindows(t *testing.T) {
	asOf := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	windows := guoraiRollingWindows(asOf, 15, 7)
	if len(windows) != 15 {
		t.Fatalf("window count = %d", len(windows))
	}
	if got := windows[0].End.Format(time.DateOnly); got != "2026-07-06" {
		t.Fatalf("oldest snapshot = %s", got)
	}
	if got := windows[0].Start.Format(time.DateOnly); got != "2026-06-30" {
		t.Fatalf("oldest window start = %s", got)
	}
	if got := windows[14].Start.Format(time.DateOnly); got != "2026-07-14" {
		t.Fatalf("latest window start = %s", got)
	}
	if got := windows[14].End.Format(time.DateOnly); got != "2026-07-20" {
		t.Fatalf("latest snapshot = %s", got)
	}
}
