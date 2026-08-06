package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const dandelionUploadArchiveEnv = "DANDELION_UPLOAD_ARCHIVE_DIR"

func archiveDandelionUpload(data []byte, uploadedAt time.Time) (string, string, error) {
	directory := strings.TrimSpace(os.Getenv(dandelionUploadArchiveEnv))
	if directory == "" {
		directory = filepath.Join("data", "dandelion-uploads")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("create Dandelion upload archive: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("secure Dandelion upload archive: %w", err)
	}

	sum := sha256.Sum256(data)
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	uploadID := uploadedAt.In(location).Format("20060102-150405.000000000") + "_" + hex.EncodeToString(sum[:6])
	path := filepath.Join(directory, uploadID+".xlsx")
	temporary, err := os.CreateTemp(directory, ".dandelion-upload-*")
	if err != nil {
		return "", "", fmt.Errorf("create temporary Dandelion upload: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("secure temporary Dandelion upload: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("write Dandelion upload: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("sync Dandelion upload: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", "", fmt.Errorf("close Dandelion upload: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", "", fmt.Errorf("archive Dandelion upload: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", "", fmt.Errorf("secure Dandelion upload: %w", err)
	}
	return uploadID, path, nil
}
