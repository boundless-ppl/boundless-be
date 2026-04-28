package service

import (
	"bytes"
	"io/fs"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"boundless-be/errs"
	"boundless-be/model"
)

func buildMultipartHeader(t *testing.T, fieldName, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(content)) + (1 << 20)); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}

	files := req.MultipartForm.File[fieldName]
	if len(files) == 0 {
		t.Fatal("missing multipart file header")
	}
	return files[0]
}

func TestLocalDocumentStorageUploadSuccess(t *testing.T) {
	baseDir := t.TempDir()
	storage := NewLocalDocumentStorage(baseDir, "https://cdn.example.com/")
	header := buildMultipartHeader(t, "file", "proof.PDF", []byte("%PDF-1.7 test-content"))

	out, err := storage.Upload(t.Context(), UploadInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypePaymentProof,
		Header:       header,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.StoragePath == "" || out.PublicURL == "" {
		t.Fatalf("expected non-empty path/url, got %#v", out)
	}
	if out.MIMEType != "application/pdf" {
		t.Fatalf("expected application/pdf mime, got %s", out.MIMEType)
	}
	if !strings.HasPrefix(out.StoragePath, baseDir) {
		t.Fatalf("expected storage path under base dir, got %s", out.StoragePath)
	}
	if !strings.HasPrefix(out.PublicURL, "https://cdn.example.com/user-1/payment_proof/") {
		t.Fatalf("expected public URL to use base URL, got %s", out.PublicURL)
	}
	if _, err := os.Stat(out.StoragePath); err != nil {
		t.Fatalf("expected stored file to exist, got %v", err)
	}
}

func TestLocalDocumentStorageUploadRejectsUnsupportedExtension(t *testing.T) {
	storage := NewLocalDocumentStorage(t.TempDir(), "")
	header := buildMultipartHeader(t, "file", "notes.txt", []byte("plain-text"))

	_, err := storage.Upload(t.Context(), UploadInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypePaymentProof,
		Header:       header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestLocalDocumentStorageUploadOversizeRemovesPartialFile(t *testing.T) {
	baseDir := t.TempDir()
	storage := NewLocalDocumentStorage(baseDir, "")
	oversize := append([]byte("%PDF-1.7 "), bytes.Repeat([]byte("a"), int(MaxDocumentSizeBytes)+32)...)
	header := buildMultipartHeader(t, "file", "big.pdf", oversize)

	_, err := storage.Upload(t.Context(), UploadInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypePaymentProof,
		Header:       header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}

	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			t.Fatalf("walk dir: %v", walkErr)
		}
		if !d.IsDir() {
			t.Fatalf("expected no uploaded file left after oversize rejection, found %s", path)
		}
		return nil
	})
}
