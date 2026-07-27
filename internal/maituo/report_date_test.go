package maituo

import (
	"testing"
	"time"
)

func TestResolveReportDateFromFileName(t *testing.T) {
	date, err := resolveReportDate("2026-07-23-MaiTuo-客户日报.xlsx", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := date.Format("2006-01-02"); got != "2026-07-23" {
		t.Fatalf("report date = %s", got)
	}
}

func TestResolveReportDateFallsBackToLatestTrend(t *testing.T) {
	date, err := resolveReportDate("Maituo-客户日报.xlsx", []SearchTrend{
		{Date: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := date.Format("2006-01-02"); got != "2026-07-24" {
		t.Fatalf("report date = %s", got)
	}
}
