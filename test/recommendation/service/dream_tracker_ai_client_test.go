package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"boundless-be/dto"
	"boundless-be/service"
)

func TestHTTPDreamTrackerAIClientCallsReviewEndpointWithJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/dream-tracker/requirements/review" {
			t.Fatalf("expected review endpoint, got %s", r.URL.Path)
		}
		if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
			t.Fatalf("expected application/json content type, got %s", contentType)
		}

		var payload dto.DreamRequirementReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.ReqCatalogID != "catalog-1" {
			t.Fatalf("expected req_catalog_id catalog-1, got %s", payload.ReqCatalogID)
		}
		if payload.DocumentURL != "https://example.com/doc.pdf" {
			t.Fatalf("expected document_url in payload, got %s", payload.DocumentURL)
		}
		if payload.RequiredDocumentType != "PASSPORT" {
			t.Fatalf("expected required_document_type PASSPORT, got %s", payload.RequiredDocumentType)
		}

		response := dto.DreamRequirementReviewResponse{
			Status:     "NEEDS_REVIEW",
			AIStatus:   "COMPLETED",
			AIMessages: []string{"Dokumen kurang jelas."},
			Meta: &dto.DreamRequirementReviewMeta{
				DocumentType:       "PASSPORT",
				VerificationStatus: "NEEDS_REVIEW",
				ConfidenceScore:    0.61,
			},
			CanReupload: true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := service.NewHTTPDreamTrackerAIClient(server.URL)
	result, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{
		DreamReqStatusID:     "req-status-1",
		DreamTrackerID:       "tracker-1",
		ReqCatalogID:         "catalog-1",
		DocumentID:           "doc-1",
		DocumentURL:          "https://example.com/doc.pdf",
		MIMEType:             "application/pdf",
		RequiredDocumentType: "PASSPORT",
		RequirementLabel:     "Passport",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if result.Status != "NEEDS_REVIEW" {
		t.Fatalf("expected status NEEDS_REVIEW, got %s", result.Status)
	}
	if result.Meta == nil || result.Meta.DocumentType != "PASSPORT" {
		t.Fatalf("expected PASSPORT meta, got %#v", result.Meta)
	}
	if !result.CanReupload {
		t.Fatalf("expected can_reupload true")
	}
}

func TestHTTPDreamTrackerAIClientReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := service.NewHTTPDreamTrackerAIClient(server.URL)
	_, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{
		DreamReqStatusID: "req-status-1",
		DreamTrackerID:   "tracker-1",
		ReqCatalogID:     "catalog-1",
		DocumentID:       "doc-1",
		DocumentURL:      "https://example.com/doc.pdf",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected upstream status in error, got %q", err)
	}
}
