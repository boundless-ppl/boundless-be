package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"boundless-be/errs"

	"github.com/google/uuid"
)

func openUploadSource(input UploadInput) (multipart.File, string, []byte, int, error) {
	if input.Header == nil {
		return nil, "", nil, 0, errs.ErrInvalidInput
	}

	src, err := input.Header.Open()
	if err != nil {
		return nil, "", nil, 0, fmt.Errorf("open file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(input.Header.Filename))
	if _, ok := allowedDocumentExtensions[ext]; !ok {
		_ = src.Close()
		return nil, "", nil, 0, errs.ErrInvalidInput
	}

	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		_ = src.Close()
		return nil, "", nil, 0, fmt.Errorf("read file header: %w", err)
	}

	objectName := fmt.Sprintf("%s/%s/%s%s", input.UserID, input.DocumentType, uuid.NewString(), ext)
	return src, objectName, buffer, n, nil
}

func buildPublicObjectURL(defaultURL, baseURL, objectName string) string {
	if strings.TrimSpace(baseURL) == "" {
		return defaultURL
	}
	return strings.TrimSuffix(baseURL, "/") + "/" + filepath.ToSlash(objectName)
}
