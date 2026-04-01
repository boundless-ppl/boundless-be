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

	"boundless-be/controller"
	"boundless-be/dto"
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

func TestRegisterSuccessController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, repo)
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
}

func TestMeUnauthorizedWhenUserIDMissingController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAuthService{}
	c := controller.NewAuthController(svc, &fakeUserRepository{})
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
	c := controller.NewAuthController(svc, &fakeUserRepository{findByIDErr: repository.ErrUserNotFound})
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
