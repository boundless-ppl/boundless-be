package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/middleware"

	"github.com/gin-gonic/gin"
)

func TestLoginRateLimitMiddlewareBlocksAfterRepeatedFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := middleware.NewLoginAttemptLimiter(3, time.Minute, 2*time.Minute)
	router := gin.New()
	router.POST("/auth/login", middleware.NewLoginRateLimitMiddleware(limiter), func(ctx *gin.Context) {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected status %d, got %d", i+1, http.StatusUnauthorized, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestLoginRateLimitMiddlewareResetsAfterSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := middleware.NewLoginAttemptLimiter(2, time.Minute, 2*time.Minute)
	shouldFail := true
	router := gin.New()
	router.POST("/auth/login", middleware.NewLoginRateLimitMiddleware(limiter), func(ctx *gin.Context) {
		if shouldFail {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"access_token": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	shouldFail = false
	req = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	shouldFail = true
	req = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
