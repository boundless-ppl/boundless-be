package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/middleware"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type fakeTokenService struct {
	claims service.TokenClaims
	err    error
}

func (f *fakeTokenService) ValidateAccessToken(token string) (service.TokenClaims, error) {
	return f.claims, f.err
}

func TestRequireAuthSuccessMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := middleware.NewAuthMiddleware(&fakeTokenService{
		claims: service.TokenClaims{
			UserID:    "u1",
			Role:      "admin",
			ExpiresAt: time.Now().Add(time.Minute),
		},
	})

	router := gin.New()
	router.GET("/secure", m.RequireAuth(), func(ctx *gin.Context) {
		if _, ok := ctx.Get(middleware.UserIDContextKey); !ok {
			t.Fatal("expected user_id in context")
		}
		if _, ok := ctx.Get(middleware.RoleContextKey); !ok {
			t.Fatal("expected role in context")
		}
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRequireAuthMissingBearerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := middleware.NewAuthMiddleware(&fakeTokenService{})

	router := gin.New()
	router.GET("/secure", m.RequireAuth(), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Basic abc")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireAuthEmptyBearerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := middleware.NewAuthMiddleware(&fakeTokenService{})

	router := gin.New()
	router.GET("/secure", m.RequireAuth(), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer    ")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := middleware.NewAuthMiddleware(&fakeTokenService{})

	router := gin.New()
	router.GET("/admin", func(ctx *gin.Context) {
		ctx.Set(middleware.RoleContextKey, "admin")
		ctx.Next()
	}, m.RequireRole("admin"), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRequireRoleForbiddenMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := middleware.NewAuthMiddleware(&fakeTokenService{})

	router := gin.New()
	router.GET("/admin", func(ctx *gin.Context) {
		ctx.Set(middleware.RoleContextKey, "user")
		ctx.Next()
	}, m.RequireRole("admin"), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}
