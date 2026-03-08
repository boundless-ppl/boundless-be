package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/api"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/google/uuid"
)

type fakeUserRepo struct{}

func (f *fakeUserRepo) Create(ctx context.Context, user model.User) (model.User, error) {
	return user, nil
}
func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (model.User, error) {
	return model.User{}, repository.ErrUserNotFound
}
func (f *fakeUserRepo) FindByID(ctx context.Context, userID string) (model.User, error) {
	return model.User{}, repository.ErrUserNotFound
}
func (f *fakeUserRepo) Update(ctx context.Context, user model.User) error {
	return nil
}

type fakeRecRepo struct {
	detail repository.SubmissionDetail
	docs   map[string]model.Document
}

func (f *fakeRecRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	if f.docs == nil {
		f.docs = map[string]model.Document{}
	}
	f.docs[doc.DocumentID] = doc
	return doc, nil
}

func (f *fakeRecRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	doc, ok := f.docs[documentID]
	if !ok || doc.UserID != userID {
		return model.Document{}, errs.ErrDocumentNotFound
	}
	return doc, nil
}

func (f *fakeRecRepo) CreateSubmission(ctx context.Context, params repository.CreateSubmissionParams) (model.RecommendationSubmission, error) {
	f.detail.Submission = params.Submission
	f.detail.Preferences = params.Preferences
	f.detail.Documents = nil
	if params.Submission.TranscriptDocumentID != nil {
		if doc, ok := f.docs[*params.Submission.TranscriptDocumentID]; ok {
			f.detail.Documents = append(f.detail.Documents, doc)
		}
	}
	if params.Submission.CVDocumentID != nil {
		if doc, ok := f.docs[*params.Submission.CVDocumentID]; ok {
			f.detail.Documents = append(f.detail.Documents, doc)
		}
	}
	return params.Submission, nil
}

func (f *fakeRecRepo) UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error {
	f.detail.Submission.Status = status
	return nil
}

func (f *fakeRecRepo) CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error) {
	return model.RecommendationResultSet{}, nil
}

func (f *fakeRecRepo) FindSubmissionDetail(ctx context.Context, submissionID, userID string) (repository.SubmissionDetail, error) {
	if f.detail.Submission.RecSubmissionID != submissionID {
		return repository.SubmissionDetail{}, errs.ErrSubmissionNotFound
	}
	return f.detail, nil
}

func setupHandler(t *testing.T, recRepo repository.RecommendationRepository) http.Handler {
	t.Helper()
	t.Setenv("AUTH_SECRET", "test-secret")
	t.Setenv("AI_SERVICE_URL", "")
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())
	t.Setenv("DOCUMENT_PUBLIC_BASE_URL", "")

	return api.NewHandler(api.Dependencies{
		UserRepo: &fakeUserRepo{},
		RecRepo:  recRepo,
	})
}

func issueToken(t *testing.T) string {
	t.Helper()
	tm := service.NewHMACTokenManager("test-secret")
	tokens, err := tm.IssueTokens("user-1", "student")
	if err != nil {
		t.Fatal(err)
	}
	return tokens.AccessToken
}

func TestUploadDocumentAndCreateSubmissionAPI(t *testing.T) {
	recRepo := &fakeRecRepo{}
	handler := setupHandler(t, recRepo)
	token := issueToken(t)

	uploadBody := &bytes.Buffer{}
	uploadWriter := multipart.NewWriter(uploadBody)
	_ = uploadWriter.WriteField("document_type", "transcript")
	fw, _ := uploadWriter.CreateFormFile("file", "transcript.pdf")
	_, _ = fw.Write([]byte("%PDF-1.7"))
	_ = uploadWriter.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/recommendations/documents", uploadBody)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadReq.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, uploadRec.Code)
	}

	var uploadResp struct {
		Document struct {
			DocumentID string `json:"document_id"`
		} `json:"document"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("invalid upload response: %v", err)
	}

	submitBody := []byte(`{"transcript_document_id":"` + uploadResp.Document.DocumentID + `","preferences":[{"pref_key":"country","pref_value":"Japan"}]}`)
	submitReq := httptest.NewRequest(http.MethodPost, "/recommendations/submissions", bytes.NewBuffer(submitBody))
	submitReq.Header.Set("Authorization", "Bearer "+token)
	submitReq.Header.Set("Content-Type", "application/json")
	submitRec := httptest.NewRecorder()
	handler.ServeHTTP(submitRec, submitReq)

	if submitRec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, submitRec.Code, submitRec.Body.String())
	}
}

func TestGetSubmissionDetailAPI(t *testing.T) {
	now := time.Now().UTC()
	submissionID := uuid.NewString()
	recRepo := &fakeRecRepo{detail: repository.SubmissionDetail{Submission: model.RecommendationSubmission{
		RecSubmissionID: submissionID,
		UserID:          "user-1",
		Status:          model.RecommendationStatusCompleted,
		CreatedAt:       now,
	}}}
	handler := setupHandler(t, recRepo)
	token := issueToken(t)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/"+submissionID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}
