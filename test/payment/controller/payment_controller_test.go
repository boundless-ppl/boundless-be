package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/controller"
	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type fakePaymentService struct {
	createPaymentOut service.CreatePaymentOutput
	createPaymentErr error
	getMyPaymentOut  service.PaymentDetailOutput
	getMyPaymentErr  error
	listAdminOut     service.AdminListPaymentsOutput
	listAdminErr     error
	updateOut        service.AdminUpdatePaymentStatusOutput
	updateErr        error
	uploadOut        model.Document
	uploadErr        error
}

func (f *fakePaymentService) ListPackages(ctx context.Context) ([]model.Subscription, error) {
	return nil, nil
}
func (f *fakePaymentService) CreatePayment(ctx context.Context, userID, subscriptionID string) (service.CreatePaymentOutput, error) {
	return f.createPaymentOut, f.createPaymentErr
}
func (f *fakePaymentService) GetMyPayment(ctx context.Context, userID, paymentID string) (service.PaymentDetailOutput, error) {
	return f.getMyPaymentOut, f.getMyPaymentErr
}
func (f *fakePaymentService) ListAdminPayments(ctx context.Context, query, status string, page, pageSize int) (service.AdminListPaymentsOutput, error) {
	return f.listAdminOut, f.listAdminErr
}
func (f *fakePaymentService) UpdatePaymentStatus(ctx context.Context, adminUserID string, paymentID string, status string, startDate *string, adminNote *string, proofDocumentID *string) (service.AdminUpdatePaymentStatusOutput, error) {
	return f.updateOut, f.updateErr
}
func (f *fakePaymentService) UploadProofForPayment(ctx context.Context, userID, paymentID string, file *multipart.FileHeader) (model.Document, error) {
	return f.uploadOut, f.uploadErr
}

func TestCreatePaymentUnauthorizedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments", controller.NewPaymentController(&fakePaymentService{}).CreatePayment)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreatePaymentSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.April, 6, 9, 0, 0, 0, time.UTC)
	expiredAt := now.Add(24 * time.Hour)
	svc := &fakePaymentService{createPaymentOut: service.CreatePaymentOutput{Payment: model.Payment{
		PaymentID:           "pay-1",
		TransactionID:       "TX-1",
		Status:              model.PaymentStatusPending,
		PackageNameSnapshot: "The Scholar",
		CreatedAt:           now,
		ExpiredAt:           &expiredAt,
	}}}

	r := gin.New()
	r.POST("/payments", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(svc).CreatePayment)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}
	var out dto.CreatePaymentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.PaymentID != "pay-1" {
		t.Fatalf("expected payment id pay-1, got %s", out.PaymentID)
	}
}

func TestGetMyPaymentNotFoundController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakePaymentService{getMyPaymentErr: errs.ErrPaymentNotFound}
	r := gin.New()
	r.GET("/payments/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(svc).GetMyPayment)

	req := httptest.NewRequest(http.MethodGet, "/payments/11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUploadProofSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakePaymentService{uploadOut: model.Document{DocumentID: "doc-1", UploadedAt: time.Now().UTC()}}
	r := gin.New()
	r.POST("/payments/:id/proof", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(svc).UploadPaymentProof)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "proof.pdf")
	_, _ = fw.Write([]byte("%PDF"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/payments/11111111-1111-1111-1111-111111111111/proof", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestListAdminPaymentsSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	proofID := "doc-1"
	proofURL := "http://localhost/doc-1"
	svc := &fakePaymentService{listAdminOut: service.AdminListPaymentsOutput{Items: []repository.AdminPaymentItem{{
		PaymentID:        "pay-1",
		TransactionID:    "tx-1",
		UserID:           "user-1",
		UserName:         "Alice",
		PackageName:      "The Scholar",
		Amount:           299000,
		Status:           model.PaymentStatusPending,
		TransactionAt:    time.Now().UTC(),
		ProofDocumentID:  &proofID,
		ProofDocumentURL: &proofURL,
	}}}}

	r := gin.New()
	r.GET("/admin/payments", controller.NewPaymentController(svc).ListAdminPayments)

	req := httptest.NewRequest(http.MethodGet, "/admin/payments?page=abc&page_size=0", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var out dto.AdminPaymentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].PaymentID != "pay-1" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestListAdminPaymentsInvalidStatusController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/payments", controller.NewPaymentController(&fakePaymentService{listAdminErr: errs.ErrInvalidPaymentStatus}).ListAdminPayments)

	req := httptest.NewRequest(http.MethodGet, "/admin/payments?status=invalid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdatePaymentStatusSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	active := now
	expires := now.AddDate(0, 1, 0)
	svc := &fakePaymentService{updateOut: service.AdminUpdatePaymentStatusOutput{
		Payment: model.Payment{
			PaymentID:     "pay-1",
			TransactionID: "tx-1",
			Status:        model.PaymentStatusSuccess,
			PaidAt:        &now,
		},
		PremiumActiveAt:  &active,
		PremiumExpiredAt: &expires,
	}}

	r := gin.New()
	r.PATCH("/admin/payments/:id/status", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "admin-1")
		c.Next()
	}, controller.NewPaymentController(svc).UpdatePaymentStatus)

	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestUpdatePaymentStatusConflictController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/admin/payments/:id/status", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "admin-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{updateErr: errs.ErrPaymentNotPending}).UpdatePaymentStatus)

	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestUploadProofInternalServerErrorController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakePaymentService{uploadErr: errors.New("storage down")}
	r := gin.New()
	r.POST("/payments/:id/proof", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(svc).UploadPaymentProof)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "proof.pdf")
	_, _ = fw.Write([]byte("%PDF"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/payments/11111111-1111-1111-1111-111111111111/proof", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestCreatePaymentInvalidBodyController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{}).CreatePayment)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreatePaymentServiceErrorController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{createPaymentErr: errs.ErrSubscriptionNotFound}).CreatePayment)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCreatePaymentUnauthorizedFromServiceController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{createPaymentErr: errs.ErrUnauthorized}).CreatePayment)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetMyPaymentSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	svc := &fakePaymentService{getMyPaymentOut: service.PaymentDetailOutput{Payment: model.Payment{
		PaymentID:              "pay-1",
		TransactionID:          "tx-1",
		Status:                 model.PaymentStatusSuccess,
		PackageNameSnapshot:    "The Scholar",
		DurationMonthsSnapshot: 3,
		PriceAmountSnapshot:    100,
		BenefitsSnapshot:       []string{"a"},
		QrisImageURL:           "-",
		CreatedAt:              now,
		ExpiredAt:              &now,
		PaidAt:                 &now,
	}, PremiumActiveAt: &now, PremiumExpiredAt: &now}}

	r := gin.New()
	r.GET("/payments/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(svc).GetMyPayment)

	req := httptest.NewRequest(http.MethodGet, "/payments/11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetMyPaymentInvalidIDController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/payments/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{}).GetMyPayment)

	req := httptest.NewRequest(http.MethodGet, "/payments/not-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdatePaymentStatusBadRequestController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/admin/payments/:id/status", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "admin-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{updateErr: errs.ErrInvalidPaymentStatus}).UpdatePaymentStatus)

	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdatePaymentStatusUnauthorizedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/admin/payments/:id/status", controller.NewPaymentController(&fakePaymentService{}).UpdatePaymentStatus)

	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestUpdatePaymentStatusInvalidIDController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/admin/payments/:id/status", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "admin-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{}).UpdatePaymentStatus)

	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/not-uuid/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUploadProofPaymentNotFoundController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments/:id/proof", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{uploadErr: errs.ErrPaymentNotFound}).UploadPaymentProof)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "proof.pdf")
	_, _ = fw.Write([]byte("%PDF"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/payments/11111111-1111-1111-1111-111111111111/proof", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUploadProofUnauthorizedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments/:id/proof", controller.NewPaymentController(&fakePaymentService{}).UploadPaymentProof)

	req := httptest.NewRequest(http.MethodPost, "/payments/11111111-1111-1111-1111-111111111111/proof", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestUploadProofInvalidIDController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments/:id/proof", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{}).UploadPaymentProof)

	req := httptest.NewRequest(http.MethodPost, "/payments/not-uuid/proof", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUploadProofMissingFileController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/payments/:id/proof", func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, "user-1")
		c.Next()
	}, controller.NewPaymentController(&fakePaymentService{}).UploadPaymentProof)

	req := httptest.NewRequest(http.MethodPost, "/payments/11111111-1111-1111-1111-111111111111/proof", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
