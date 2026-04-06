package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type fakePaymentService struct {
	createPaymentOut service.CreatePaymentOutput
	createPaymentErr error
	getMyPaymentErr  error
	uploadOut        model.Document
}

func (f *fakePaymentService) ListPackages(ctx context.Context) ([]model.Subscription, error) {
	return nil, nil
}
func (f *fakePaymentService) CreatePayment(ctx context.Context, userID, subscriptionID string) (service.CreatePaymentOutput, error) {
	return f.createPaymentOut, f.createPaymentErr
}
func (f *fakePaymentService) GetMyPayment(ctx context.Context, userID, paymentID string) (service.PaymentDetailOutput, error) {
	return service.PaymentDetailOutput{}, f.getMyPaymentErr
}
func (f *fakePaymentService) ListAdminPayments(ctx context.Context, query, status string, page, pageSize int) (service.AdminListPaymentsOutput, error) {
	return service.AdminListPaymentsOutput{}, nil
}
func (f *fakePaymentService) UpdatePaymentStatus(ctx context.Context, adminUserID string, paymentID string, status string, startDate *string, adminNote *string, proofDocumentID *string) (service.AdminUpdatePaymentStatusOutput, error) {
	return service.AdminUpdatePaymentStatusOutput{}, nil
}
func (f *fakePaymentService) UploadProofForPayment(ctx context.Context, userID, paymentID string, file *multipart.FileHeader) (model.Document, error) {
	return f.uploadOut, nil
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
