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

func TestRegisterSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{}, nil)
	router := gin.New()
	router.POST("/auth/register", c.Register)

	body, _ := json.Marshal(dto.RegisterRequest{NamaLengkap: "Alice Doe", Email: "alice@example.com", Password: "Secret123!"})
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

	body, _ := json.Marshal(dto.RegisterRequest{NamaLengkap: "Alice Doe", Email: "alice@example.com", Password: "Secret123!"})
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

	body, _ := json.Marshal(dto.LoginRequest{Email: "alice@example.com", Password: "Secret123!"})
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

	body, _ := json.Marshal(dto.LoginRequest{Email: "alice@example.com", Password: "wrong"})
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

func TestMeSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	repo := &fakeUserRepository{findByIDUser: model.User{
		UserID:      "u-1",
		NamaLengkap: "Alice Doe",
		Email:       "alice@example.com",
		Role:        "admin",
	}}
	c := controller.NewAuthController(svc, repo, &fakePremiumRepository{currentSubscription: model.UserSubscription{
		UserSubscriptionID:  "us-1",
		UserID:              "u-1",
		SubscriptionID:      "sub-1",
		SourcePaymentID:     "pay-1",
		PackageNameSnapshot: "The Scholar",
		StartDate:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		EndDate:             time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC),
		CreatedAt:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	}})
	router := gin.New()
	router.GET("/auth/me", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-1")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
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
	if got.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", got.Email)
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
	router.GET("/auth/me", c.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
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
	router.GET("/auth/me", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-missing")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
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
	router.GET("/auth/me", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "u-2")
		ctx.Next()
	}, c.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
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
