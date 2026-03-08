package service_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakeRecommendationRepo struct {
	createdDocument       model.Document
	createDocumentErr     error
	findDocumentByIDUser  map[string]model.Document
	findDocumentByIDError error

	createSubmissionParams repository.CreateSubmissionParams
	createSubmissionErr    error

	detail repository.SubmissionDetail
}

func (f *fakeRecommendationRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	f.createdDocument = doc
	if f.createDocumentErr != nil {
		return model.Document{}, f.createDocumentErr
	}
	return doc, nil
}

func (f *fakeRecommendationRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	if f.findDocumentByIDError != nil {
		return model.Document{}, f.findDocumentByIDError
	}
	doc, ok := f.findDocumentByIDUser[documentID]
	if !ok || doc.UserID != userID {
		return model.Document{}, errs.ErrDocumentNotFound
	}
	return doc, nil
}

func (f *fakeRecommendationRepo) CreateSubmission(ctx context.Context, params repository.CreateSubmissionParams) (model.RecommendationSubmission, error) {
	f.createSubmissionParams = params
	if f.createSubmissionErr != nil {
		return model.RecommendationSubmission{}, f.createSubmissionErr
	}
	return params.Submission, nil
}

func (f *fakeRecommendationRepo) UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error {
	return nil
}

func (f *fakeRecommendationRepo) CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error) {
	return model.RecommendationResultSet{}, nil
}

func (f *fakeRecommendationRepo) FindSubmissionDetail(ctx context.Context, submissionID, userID string) (repository.SubmissionDetail, error) {
	return f.detail, nil
}

func makeFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	files := req.MultipartForm.File[fieldName]
	if len(files) == 0 {
		t.Fatal("missing multipart file header")
	}
	return files[0]
}

func TestCreateSubmissionServiceRejectsWhenNoDocument(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)

	_, err := svc.CreateSubmission(context.Background(), service.CreateSubmissionInput{UserID: "user-1"})
	if err != errs.ErrNoDocumentProvided {
		t.Fatalf("expected %v, got %v", errs.ErrNoDocumentProvided, err)
	}
}

func TestCreateSubmissionServiceRejectsUnsupportedFileType(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "transcript", "transcript.txt", []byte("not allowed"))

	_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeTranscript,
		File:         header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestCreateSubmissionServiceRejectsFileLargerThan5MB(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "cv", "cv.pdf", []byte("small"))
	header.Size = service.MaxDocumentSizeBytes + 1

	_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeCV,
		File:         header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestUploadDocumentServiceSuccess(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "")
	t.Setenv("DOCUMENT_PUBLIC_BASE_URL", "")
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "transcript", "transcript.pdf", []byte("%PDF-1.7 content"))

	out, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeTranscript,
		File:         header,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Document.DocumentType != model.DocumentTypeTranscript {
		t.Fatalf("expected transcript type, got %s", out.Document.DocumentType)
	}
	if out.Document.SizeBytes <= 0 {
		t.Fatal("expected positive stored file size")
	}
	if _, err := os.Stat(out.Document.StoragePath); err != nil {
		t.Fatalf("expected uploaded file on disk, got %v", err)
	}
	if filepath.Ext(out.Document.OriginalFilename) != ".pdf" {
		t.Fatalf("expected pdf filename, got %s", out.Document.OriginalFilename)
	}
}

func TestCreateSubmissionServiceSuccessDraftMode(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "")

	transcriptID := "doc-transcript-1"
	repo := &fakeRecommendationRepo{
		findDocumentByIDUser: map[string]model.Document{
			transcriptID: {
				DocumentID:       transcriptID,
				UserID:           "user-1",
				OriginalFilename: "transcript.pdf",
				PublicURL:        "http://local/transcript.pdf",
				DocumentType:     model.DocumentTypeTranscript,
			},
		},
	}
	svc := service.NewRecommendationService(repo)

	out, err := svc.CreateSubmission(context.Background(), service.CreateSubmissionInput{
		UserID:               "user-1",
		TranscriptDocumentID: &transcriptID,
		Preferences: []dto.PreferenceInput{
			{Key: "country", Value: "Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != model.RecommendationStatusDraft {
		t.Fatalf("expected draft status, got %s", out.Status)
	}
}

func TestUploadDocumentServiceAllowsSupportedExtensions(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	tests := []struct {
		name     string
		filename string
	}{
		{name: "pdf", filename: "doc.pdf"},
		{name: "png", filename: "doc.png"},
		{name: "jpg", filename: "doc.jpg"},
		{name: "jpeg", filename: "doc.jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRecommendationRepo{}
			svc := service.NewRecommendationService(repo)
			header := makeFileHeader(t, "file", tt.filename, []byte("dummy content"))

			_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
				UserID:       "user-1",
				DocumentType: model.DocumentTypeTranscript,
				File:         header,
			})
			if err != nil {
				t.Fatalf("expected nil error for %s, got %v", tt.filename, err)
			}
		})
	}
}
