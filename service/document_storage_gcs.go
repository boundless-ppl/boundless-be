package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"boundless-be/errs"

	"cloud.google.com/go/storage"
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
	src, objectName, buffer, n, err := openUploadSource(input)
	if err != nil {
		return StoredObject{}, err
	}
	defer src.Close()
	writer := s.client.Bucket(s.bucketName).Object(objectName).NewWriter(ctx)

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

	return StoredObject{
		StoragePath: objectName,
		PublicURL:   buildPublicObjectURL(fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucketName, objectName), s.publicBaseURL, objectName),
		SizeBytes:   size,
		MIMEType:    contentType,
	}, nil
}
