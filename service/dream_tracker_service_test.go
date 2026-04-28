package service

import (
	"path/filepath"
	"strings"
	"testing"

	"boundless-be/model"
)

func TestResolveReviewDocumentURLPrefersLocalPathWhenAIServiceLocalhost(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "http://127.0.0.1:8000")
	t.Setenv("DOCUMENT_STORAGE_DIR", "/tmp/boundless-storage")

	doc := model.Document{
		StoragePath: "user-1/PASSPORT/abc.pdf",
		PublicURL:   "https://cdn.example.com/doc.pdf",
	}

	url := resolveReviewDocumentURL(doc)
	expected := filepath.Join("/tmp/boundless-storage", "user-1/PASSPORT/abc.pdf")
	if !strings.HasSuffix(url, expected) {
		t.Fatalf("expected local path ending with %s, got %s", expected, url)
	}
}

func TestResolveReviewDocumentURLUsesPublicURLWhenAINotLocal(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "https://ai.example.com")

	doc := model.Document{
		StoragePath: "user-1/PASSPORT/abc.pdf",
		PublicURL:   "https://cdn.example.com/doc.pdf",
	}

	url := resolveReviewDocumentURL(doc)
	if url != "https://cdn.example.com/doc.pdf" {
		t.Fatalf("expected public URL, got %s", url)
	}
}

func TestResolveReviewDocumentURLFallsBackToLocalPathWhenPublicMissing(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "https://ai.example.com")
	t.Setenv("DOCUMENT_STORAGE_DIR", "/tmp/boundless-storage")

	doc := model.Document{StoragePath: "user-1/PASSPORT/abc.pdf"}
	url := resolveReviewDocumentURL(doc)
	expected := filepath.Join("/tmp/boundless-storage", "user-1/PASSPORT/abc.pdf")
	if !strings.HasSuffix(url, expected) {
		t.Fatalf("expected local fallback ending with %s, got %s", expected, url)
	}
}
