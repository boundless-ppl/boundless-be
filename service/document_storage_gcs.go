package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"boundless-be/errs"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type GCSDocumentStorage struct {
	client        *storage.Client
	bucketName    string
	publicBaseURL string
}

func NewGCSDocumentStorage(ctx context.Context, bucketName, publicBaseURL string) (*GCSDocumentStorage, error) {
	if strings.TrimSpace(bucketName) == "" {
		return nil, fmt.Errorf("GCS_BUCKET_NAME is required")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create gcs client: %w", err)
	}

	return &GCSDocumentStorage{
		client:        client,
		bucketName:    bucketName,
		publicBaseURL: strings.TrimSuffix(publicBaseURL, "/"),
	}, nil
}

func (s *GCSDocumentStorage) Upload(ctx context.Context, input UploadInput) (StoredObject, error) {
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
	writer := s.client.Bucket(s.bucketName).Object(objectName).NewWriter(ctx)

	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		_ = writer.Close()
		return StoredObject{}, fmt.Errorf("read file header: %w", err)
	}

	contentType := detectContentType(buffer[:n])
	writer.ContentType = contentType

	if n > 0 {
		if _, err := writer.Write(buffer[:n]); err != nil {
			_ = writer.Close()
			return StoredObject{}, fmt.Errorf("write gcs header chunk: %w", err)
		}
	}

	limited := io.LimitReader(src, MaxDocumentSizeBytes+1)
	copied, err := io.Copy(writer, limited)
	if err != nil {
		_ = writer.Close()
		return StoredObject{}, fmt.Errorf("stream file to gcs: %w", err)
	}

	size := int64(n) + copied
	if size > MaxDocumentSizeBytes {
		_ = writer.Close()
		_ = s.client.Bucket(s.bucketName).Object(objectName).Delete(ctx)
		return StoredObject{}, errs.ErrInvalidInput
	}

	if err := writer.Close(); err != nil {
		return StoredObject{}, fmt.Errorf("finalize gcs upload: %w", err)
	}

	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucketName, objectName)
	if s.publicBaseURL != "" {
		publicURL = s.publicBaseURL + "/" + objectName
	}

	return StoredObject{
		StoragePath: objectName,
		PublicURL:   publicURL,
		SizeBytes:   size,
		MIMEType:    contentType,
	}, nil
}
