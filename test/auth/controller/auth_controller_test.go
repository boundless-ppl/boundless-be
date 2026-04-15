package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

type fakeAuthService struct {
	registerErr error
	loginTokens service.AuthTokens
	loginErr    error
	logoutErr   error
}

func (f *fakeAuthService) Register(ctx context.Context, fullName, role, email, password string) error {
	return f.registerErr
}

func (f *fakeAuthService) Login(ctx context.Context, email, password string) (service.AuthTokens, error) {
	return f.loginTokens, f.loginErr
}

func (f *fakeAuthService) Logout(token string) error {
	return f.logoutErr
}

type fakeUserRepository struct {
	findByIDUser model.User
	findByIDErr  error
}

type fakePremiumRepository struct {
	currentSubscription model.UserSubscription
	currentErr          error
	currentPayment      model.Payment
	currentPaymentErr   error
}

func (f *fakeUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	return model.User{}, nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	return model.User{}, repository.ErrUserNotFound
}

func (f *fakeUserRepository) FindByID(ctx context.Context, userID string) (model.User, error) {
	if f.findByIDErr != nil {
		return model.User{}, f.findByIDErr
	}
	if f.findByIDUser.UserID == "" {
		return model.User{}, repository.ErrUserNotFound
	}
	return f.findByIDUser, nil
}

func (f *fakeUserRepository) Update(ctx context.Context, user model.User) error {
	return nil
}

func (f *fakePremiumRepository) FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error) {
	if f.currentErr != nil {
		return model.UserSubscription{}, f.currentErr
	}
	if f.currentSubscription.UserSubscriptionID == "" {
		return model.UserSubscription{}, errs.ErrPremiumSubscriptionNotFound
	}
	return f.currentSubscription, nil
}

func (f *fakePremiumRepository) FindLatestPendingPaymentByUser(ctx context.Context, userID string, reference time.Time) (model.Payment, error) {
	if f.currentPaymentErr != nil {
		return model.Payment{}, f.currentPaymentErr
	}
	if f.currentPayment.PaymentID == "" {
		return model.Payment{}, errs.ErrPaymentNotFound
	}
	return f.currentPayment, nil
}

func TestRegisterSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/register", c.Register)

	body, _ := json.Marshal(dto.RegisterRequest{NamaLengkap: "Alice Doe", Email: USER_EMAIL_ALICE, Password: "Secret123!"})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestRegisterInvalidBodyController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/register", c.Register)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRegisterDuplicateEmailController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{registerErr: repository.ErrEmailExists}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/register", c.Register)

	body, _ := json.Marshal(dto.RegisterRequest{NamaLengkap: "Alice Doe", Email: USER_EMAIL_ALICE, Password: "Secret123!"})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestLoginSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{loginTokens: service.AuthTokens{AccessToken: "a", RefreshToken: "r"}}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/login", c.Login)

	body, _ := json.Marshal(dto.LoginRequest{Email: USER_EMAIL_ALICE, Password: "Secret123!"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got dto.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.AccessToken == "" || got.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
}

func TestLoginInvalidCredentialsController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{loginErr: service.ErrInvalidCredentials}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/login", c.Login)

	body, _ := json.Marshal(dto.LoginRequest{Email: USER_EMAIL_ALICE, Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "email") {
		t.Fatal("expected sanitized error message")
	}
}

func TestLoginInvalidBodyController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/login", c.Login)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLogoutSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/logout", func(ctx *gin.Context) {
		ctx.Set(middleware.TokenContextKey, "token")
		ctx.Next()
	}, c.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestLogoutUnauthorizedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{logoutErr: errors.New("fail")}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/logout", c.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

const ROUTE_AUTH_ME = "/auth/me"
const THE_SCHOLAR = "The Scholar"
const USER_EMAIL_ALICE = "alice@example.com"

func TestMeSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-1",
		NamaLengkap: "Alice Doe",
		Email:       USER_EMAIL_ALICE,
		Role:        "admin",
	}}
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentSubscription: model.UserSubscription{
		UserSubscriptionID:  "us-1",
		UserID:              "u-1",
		SubscriptionID:      "sub-1",
		SourcePaymentID:     "pay-1",
		PackageNameSnapshot: THE_SCHOLAR,
		StartDate:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		EndDate:             time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC),
		CreatedAt:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	}})
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-1")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got dto.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got.UserID != "u-1" {
		t.Fatalf("expected user_id u-1, got %s", got.UserID)
	}
	if got.Email != USER_EMAIL_ALICE {
		t.Fatalf("expected email %s, got %s", USER_EMAIL_ALICE, got.Email)
	}
	if got.Role != "admin" {
		t.Fatalf("expected role admin, got %s", got.Role)
	}
	if !got.IsPremium {
		t.Fatal("expected premium user")
	}
	if got.PremiumStartAt == nil || got.PremiumEndAt == nil {
		t.Fatalf("expected premium dates, got %+v", got)
	}
}

func TestMeUnauthorizedWhenUserIDMissingController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMeUnauthorizedWhenUserNotFoundController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{findByIDErr: repository.ErrUserNotFound}, nil)
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-missing")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMeNonPremiumController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-2",
		NamaLengkap: "Bob Doe",
		Email:       "bob@example.com",
		Role:        "user",
	}}
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentErr: errs.ErrPremiumSubscriptionNotFound})
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-2")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got dto.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.IsPremium {
		t.Fatal("expected non premium user")
	}
	if got.PremiumStartAt != nil || got.PremiumEndAt != nil {
		t.Fatalf("expected nil premium dates, got %+v", got)
	}
}

func TestMeShowsPendingPaymentAwaitingAdminVerificationController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-4",
		NamaLengkap: "Alice",
		Email:       USER_EMAIL_ALICE,
		Role:        "user",
	}}
	expiredAt := time.Date(2026, time.April, 16, 10, 0, 0, 0, time.UTC)
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentPayment: model.Payment{
		PaymentID:           "pay-1",
		TransactionID:       "TX-1",
		UserID:              "u-4",
		PackageNameSnapshot: THE_SCHOLAR,
		Status:              model.PaymentStatusPending,
		ExpiredAt:           &expiredAt,
		ProofDocumentID:     ptrString("doc-1"),
	}})
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-4")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got dto.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !got.HasPendingPayment {
		t.Fatal("expected pending payment flag")
	}
	if got.TransactionID == nil || *got.TransactionID != "TX-1" {
		t.Fatalf("expected transaction id TX-1, got %+v", got.TransactionID)
	}
}

func TestMeIgnoresExpiredPendingPaymentController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-5",
		NamaLengkap: "Alice",
		Email:       USER_EMAIL_ALICE,
		Role:        "user",
	}}
	expiredAt := time.Date(2026, time.April, 14, 10, 0, 0, 0, time.UTC)
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentPayment: model.Payment{
		PaymentID:           "pay-expired",
		TransactionID:       "TX-EXPIRED",
		UserID:              "u-5",
		PackageNameSnapshot: THE_SCHOLAR,
		Status:              model.PaymentStatusPending,
		ExpiredAt:           &expiredAt,
		ProofDocumentID:     ptrString("doc-2"),
	}})
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-5")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got dto.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.HasPendingPayment {
		t.Fatalf("expected expired pending payment to be ignored, got %+v", got)
	}
}

func TestMeIgnoresPendingPaymentWithoutProofController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-6",
		NamaLengkap: "Alice",
		Email:       USER_EMAIL_ALICE,
		Role:        "user",
	}}
	expiredAt := time.Date(2026, time.April, 16, 10, 0, 0, 0, time.UTC)
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentPayment: model.Payment{
		PaymentID:           "pay-no-proof",
		TransactionID:       "TX-NO-PROOF",
		UserID:              "u-6",
		PackageNameSnapshot: THE_SCHOLAR,
		Status:              model.PaymentStatusPending,
		ExpiredAt:           &expiredAt,
	}})
	router := gin.New()
	router.GET(ROUTE_AUTH_ME, func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-6")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, ROUTE_AUTH_ME, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got dto.MeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.HasPendingPayment {
		t.Fatalf("expected pending payment without proof to be ignored, got %+v", got)
	}
}

func ptrString(value string) *string {
	return &value
}
