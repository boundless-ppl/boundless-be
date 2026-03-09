package service

import (
	"testing"

	"boundless-be/errs"
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
