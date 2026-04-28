package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"boundless-be/errs"

	"github.com/google/uuid"
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
	if input.Header == nil {
		return StoredObject{}, errs.ErrInvalidInput
	}

	src, err := input.Header.Open()
	if err != nil {
		return StoredObject{}, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(input.Header.Filename))
	if _, ok := allowedDocumentExtensions[ext]; !ok {
		return StoredObject{}, errs.ErrInvalidInput
	}

	objectName := fmt.Sprintf("%s/%s/%s%s", input.UserID, input.DocumentType, uuid.NewString(), ext)
	fullPath := filepath.Join(s.baseDir, objectName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return StoredObject{}, fmt.Errorf("create directory: %w", err)
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		return StoredObject{}, fmt.Errorf("read file header: %w", err)
	}

	if n > 0 {
		if _, err := dst.Write(buffer[:n]); err != nil {
			return StoredObject{}, fmt.Errorf("write file header: %w", err)
		}
	}

	limited := io.LimitReader(src, MaxDocumentSizeBytes+1)
	bufPtr := copyBufPool.Get().(*[]byte)
	copied, err := io.CopyBuffer(dst, limited, *bufPtr)
	copyBufPool.Put(bufPtr)
	if err != nil {
		return StoredObject{}, fmt.Errorf("copy file: %w", err)
	}

	size := int64(n) + copied
	if size > MaxDocumentSizeBytes {
		_ = dst.Close()
		_ = os.Remove(fullPath)
		return StoredObject{}, errs.ErrInvalidInput
	}

	publicURL := fullPath
	if s.baseURL != "" {
		publicURL = s.baseURL + "/" + filepath.ToSlash(objectName)
	}

	return StoredObject{
		StoragePath: fullPath,
		PublicURL:   publicURL,
		SizeBytes:   size,
		MIMEType:    detectContentType(buffer[:n]),
	}, nil
}

func mustBuildDocumentStorage() DocumentStorage {
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
