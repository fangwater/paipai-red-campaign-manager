package maituo

import (
	"os"
	"testing"
)

func TestUploadedSample(t *testing.T) {
	path := os.Getenv("MAITUO_SAMPLE_PATH")
	if path == "" {
		t.Skip("MAITUO_SAMPLE_PATH is not set")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	snapshot, err := Parse(file, "2026-07-23-MaiTuo-客户日报.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.KPIs) != 8 || len(snapshot.Notes) != 243 || len(snapshot.SPUs) != 2 || len(snapshot.Subaccounts) != 8 || len(snapshot.Trends) != 14 {
		t.Fatalf("unexpected row counts: kpis=%d notes=%d spus=%d subaccounts=%d trends=%d", len(snapshot.KPIs), len(snapshot.Notes), len(snapshot.SPUs), len(snapshot.Subaccounts), len(snapshot.Trends))
	}
	if snapshot.FileSHA256 != "34f7619f2db09d8f8233c99c87a5aee8c0e06208f2c7adb850f79d241d06082b" {
		t.Fatalf("unexpected SHA-256: %s", snapshot.FileSHA256)
	}
}
