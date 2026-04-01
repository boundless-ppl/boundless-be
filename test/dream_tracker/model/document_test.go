package model_test

import (
	"testing"

	"boundless-be/model"
)

func TestDocumentAdditionalFieldsModel(t *testing.T) {
	doc := model.Document{
		DocumentID:    "doc-1",
		UserID:        "user-1",
		Nama:          "CV Grace",
		DokumenURL:    "https://example.com/cv.pdf",
		DokumenSizeKB: 256,
	}

	if doc.Nama != "CV Grace" {
		t.Fatalf("unexpected nama: %q", doc.Nama)
	}
	if doc.DokumenURL != "https://example.com/cv.pdf" {
		t.Fatalf("unexpected dokumen_url: %q", doc.DokumenURL)
	}
	if doc.DokumenSizeKB != 256 {
		t.Fatalf("unexpected dokumen size: %d", doc.DokumenSizeKB)
	}
}
