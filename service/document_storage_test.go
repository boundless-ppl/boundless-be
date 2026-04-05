package service

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"boundless-be/errs"
	"boundless-be/model"
)

func TestNewGCSDocumentStorageRequiresBucketName(t *testing.T) {
	storage, err := NewGCSDocumentStorage(t.Context(), "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if storage != nil {
		t.Fatal("expected nil storage")
	}
}

func TestGCSDocumentStorageUploadRejectsNilHeader(t *testing.T) {
	storage := &GCSDocumentStorage{}
	_, err := storage.Upload(t.Context(), UploadInput{})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestMustBuildDocumentStorageReturnsLocalProvider(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_PROVIDER", "local")
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	storage := mustBuildDocumentStorage()
	if _, ok := storage.(*LocalDocumentStorage); !ok {
		t.Fatalf("expected *LocalDocumentStorage, got %T", storage)
	}
}

func TestMustBuildDocumentStoragePanicsOnUnsupportedProvider(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_PROVIDER", "unsupported")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = mustBuildDocumentStorage()
}

func TestLocalDocumentStorageUploadSuccess(t *testing.T) {
	storage := NewLocalDocumentStorage(t.TempDir(), "https://cdn.example.com")
	header := newFileHeader(t, "cv.pdf", []byte("%PDF-1.7 test file"))

	object, err := storage.Upload(t.Context(), UploadInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeCV,
		Header:       header,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if object.SizeBytes == 0 {
		t.Fatal("expected non-zero size")
	}
	if filepath.Ext(object.StoragePath) != ".pdf" {
		t.Fatalf("unexpected storage path: %s", object.StoragePath)
	}
	if object.PublicURL == object.StoragePath {
		t.Fatal("expected public url to use configured base url")
	}
	if _, err := os.Stat(object.StoragePath); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}
}

func TestLocalDocumentStorageUploadRejectsInvalidExtension(t *testing.T) {
	storage := NewLocalDocumentStorage(t.TempDir(), "")
	header := newFileHeader(t, "cv.exe", []byte("not allowed"))

	_, err := storage.Upload(t.Context(), UploadInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeCV,
		Header:       header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func newFileHeader(t *testing.T, filename string, contents []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(contents)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(contents) + 1024)); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	_ = file.Close()
	return header
}
