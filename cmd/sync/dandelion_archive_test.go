package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveDandelionUpload(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(dandelionUploadArchiveEnv, directory)
	uploadedAt := time.Date(2026, 8, 6, 15, 50, 12, 123, time.FixedZone("Asia/Shanghai", 8*60*60))
	uploadID, path, err := archiveDandelionUpload([]byte("workbook"), uploadedAt)
	if err != nil {
		t.Fatal(err)
	}
	if uploadID != "20260806-155012.000000123_244f4e42fb0e" {
		t.Fatalf("upload ID = %q", uploadID)
	}
	if filepath.Dir(path) != directory {
		t.Fatalf("archive path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "workbook" {
		t.Fatalf("archive data = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("archive mode = %o", info.Mode().Perm())
	}
}
