package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"boundless-be/errs"
)

type LocalDocumentStorage struct {
	baseDir string
	baseURL string
}

func NewLocalDocumentStorage(baseDir, baseURL string) *LocalDocumentStorage {
	if baseDir == "" {
		baseDir = "uploads"
	}
	return &LocalDocumentStorage{
		baseDir: baseDir,
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

func (s *LocalDocumentStorage) Upload(ctx context.Context, input UploadInput) (StoredObject, error) {
	_ = ctx
	src, objectName, buffer, n, err := openUploadSource(input)
	if err != nil {
		return StoredObject{}, err
	}
	defer src.Close()
	fullPath := filepath.Join(s.baseDir, objectName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return StoredObject{}, fmt.Errorf("create directory: %w", err)
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if n > 0 {
		if _, err := dst.Write(buffer[:n]); err != nil {
			return StoredObject{}, fmt.Errorf("write file header: %w", err)
		}
	}

	limited := io.LimitReader(src, MaxDocumentSizeBytes+1)
	copied, err := io.Copy(dst, limited)
	if err != nil {
		return StoredObject{}, fmt.Errorf("copy file: %w", err)
	}

	size := int64(n) + copied
	if size > MaxDocumentSizeBytes {
		_ = dst.Close()
		_ = os.Remove(fullPath)
		return StoredObject{}, errs.ErrInvalidInput
	}

	return StoredObject{
		StoragePath: fullPath,
		PublicURL:   buildPublicObjectURL(fullPath, s.baseURL, objectName),
		SizeBytes:   size,
		MIMEType:    detectContentType(buffer[:n]),
	}, nil
}

func mustBuildDocumentStorage() DocumentUploader {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_PROVIDER")))
	switch provider {
	case "", "local":
		return NewLocalDocumentStorage(
			os.Getenv("DOCUMENT_STORAGE_DIR"),
			os.Getenv("DOCUMENT_PUBLIC_BASE_URL"),
		)
	case "gcs":
		storage, err := NewGCSDocumentStorage(
			context.Background(),
			os.Getenv("GCS_BUCKET_NAME"),
			os.Getenv("GCS_BUCKET_PUBLIC_BASE_URL"),
		)
		if err != nil {
			panic(err)
		}
		return storage
	default:
		panic("unsupported DOCUMENT_STORAGE_PROVIDER")
	}
}
