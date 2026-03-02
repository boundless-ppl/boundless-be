package service_test

import (
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Setenv("AUTH_SECRET", "test-secret")
	os.Exit(m.Run())
}

func TestRegisterSuccessService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	tokens, err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
}

func TestRegisterInvalidEmailService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_, err := authService.Register(context.Background(), "Alice Doe", "admin", "alice", "Secret123!")
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
	}
}

func TestRegisterWeakPasswordService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_, err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "weak")
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
	}
}

func TestPasswordStoredAsHashService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)
	rawPassword := "Secret123!"

	_, err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", rawPassword)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	user, err := userRepo.FindByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if user.PasswordHash == rawPassword || user.PasswordHash == "" {
		t.Fatal("expected password hash to be encrypted")
	}
}

func TestLoginSuccessUnderThreeSecondsService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_, _ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	start := time.Now()
	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("expected login under 3 seconds, got %v", elapsed)
	}
}

func TestLoginWrongPasswordService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_, _ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	_, err := authService.Login(context.Background(), "alice@example.com", "Wrong123!")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidCredentials, err)
	}
}

func TestTokenLifetimeService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	tokens, err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	accessExp := tokenExpiryFromRaw(t, tokens.AccessToken)
	refreshExp := tokenExpiryFromRaw(t, tokens.RefreshToken)
	accessDuration := time.Until(accessExp)
	refreshDuration := time.Until(refreshExp)

	if accessDuration < 14*time.Minute || accessDuration > 16*time.Minute {
		t.Fatalf("expected access token around 15 minutes, got %v", accessDuration)
	}
	if refreshDuration < 23*time.Hour || refreshDuration > 25*time.Hour {
		t.Fatalf("expected refresh token around 24 hours, got %v", refreshDuration)
	}
}

func TestValidateTokenAndLogoutService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	tokens, _ := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if _, err := authService.ValidateAccessToken(tokens.AccessToken); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := authService.Logout(tokens.AccessToken); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, err := authService.ValidateAccessToken(tokens.AccessToken); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestLoginAttemptLimitAndLockService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_, _ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	for i := 0; i < 4; i++ {
		_, err := authService.Login(context.Background(), "alice@example.com", "Wrong123!")
		if !errors.Is(err, service.ErrInvalidCredentials) {
			t.Fatalf("expected invalid credentials on attempt %d, got %v", i+1, err)
		}
	}

	_, err := authService.Login(context.Background(), "alice@example.com", "Wrong123!")
	if !errors.Is(err, service.ErrAccountLocked) {
		t.Fatalf("expected %v, got %v", service.ErrAccountLocked, err)
	}
}

func tokenExpiryFromRaw(t *testing.T, token string) time.Time {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("invalid token format: %s", token)
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("dec