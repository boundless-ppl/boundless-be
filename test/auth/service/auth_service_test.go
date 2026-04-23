package service_test

import (
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto/hmac"
	"crypto/sha256"
)

func TestMain(m *testing.M) {
	os.Setenv("AUTH_SECRET", "test-secret")
	os.Exit(m.Run())
}

func TestRegisterSuccessService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRegisterInvalidEmailService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice", "Secret123!")
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
	}
}

func TestRegisterWeakPasswordService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "weak")
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
	}
}

func TestRegisterDuplicateEmailService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if !errors.Is(err, repository.ErrEmailExists) {
		t.Fatalf("expected %v, got %v", repository.ErrEmailExists, err)
	}
}

func TestRegisterUnexpectedRepoErrorService(t *testing.T) {
	userRepo := newTestUserRepoService()
	userRepo.createErr = errors.New("db down")
	authService := service.NewAuthService(userRepo)

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
	}
}

func TestPasswordStoredAsHashService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)
	rawPassword := "Secret123!"

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", rawPassword)
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

	_ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
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

	_ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	_, err := authService.Login(context.Background(), "alice@example.com", "Wrong123!")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidCredentials, err)
	}
}

func TestLoginResetCounterUpdateErrorStillIssuesTokenService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	user, err := userRepo.FindByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	user.FailedLoginCount = 1
	user.FirstFailedAt = time.Now().Add(-time.Minute)
	userRepo.byEmail[user.Email] = user
	userRepo.byID[user.UserID] = user

	userRepo.updateErr = errors.New("update failed")
	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
}

func TestTokenLifetimeService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
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

	if err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, err := authService.ValidateToken(tokens.AccessToken); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
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

func TestValidateTokenInvalidService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if _, err := authService.ValidateToken("invalid-token"); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestValidateAccessTokenRejectRefreshService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if _, err := authService.ValidateAccessToken(tokens.RefreshToken); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestRefreshAccessSuccessService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if err := authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	tokens, err := authService.Login(context.Background(), "alice@example.com", "Secret123!")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	access, err := authService.RefreshAccess(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if access == "" {
		t.Fatal("expected non-empty refreshed access token")
	}
	if _, err := authService.ValidateAccessToken(access); err != nil {
		t.Fatalf("expected refreshed access token to be valid, got %v", err)
	}
}

func TestRefreshAccessRejectsInvalidTokenService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if _, err := authService.RefreshAccess("bad.token"); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestValidateAccessTokenExpiredService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)
	secret := os.Getenv("AUTH_SECRET")
	expired := signedToken(secret, "tid-expired", "access", "u1", "admin", time.Now().Add(-time.Minute).Unix())

	if _, err := authService.ValidateAccessToken(expired); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestLogoutInvalidTokenService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if err := authService.Logout("bad.token.format"); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestValidateTokenMalformedPayloadService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)
	secret := os.Getenv("AUTH_SECRET")

	// payload has invalid exp field to hit parse-int error path.
	payload := base64.RawURLEncoding.EncodeToString([]byte("id|access|u1|admin|not-number"))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := payload + "." + sig

	if _, err := authService.ValidateToken(token); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestValidateTokenInvalidPartCountService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	if _, err := authService.ValidateToken("one-part-only"); !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected %v, got %v", service.ErrInvalidToken, err)
	}
}

func TestLogoutCleansExpiredRevocationsService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)
	secret := os.Getenv("AUTH_SECRET")

	expired := signedToken(secret, "expired-id", "access", "u1", "admin", time.Now().Add(-time.Minute).Unix())
	valid := signedToken(secret, "valid-id", "access", "u1", "admin", time.Now().Add(time.Minute).Unix())

	if err := authService.Logout(expired); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := authService.Logout(valid); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestNewAuthServiceMissingSecretService(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		_ = os.Unsetenv("AUTH_SECRET")
		_ = service.NewAuthService(newTestUserRepoService())
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestNewAuthServiceMissingSecretService")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.Success() {
		t.Fatalf("expected non-zero exit, got %v", err)
	}
}

func TestLoginAttemptLimitAndLockService(t *testing.T) {
	userRepo := newTestUserRepoService()
	authService := service.NewAuthService(userRepo)

	_ = authService.Register(context.Background(), "Alice Doe", "admin", "alice@example.com", "Secret123!")
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
		t.Fatalf("decode error: %v", err)
	}
	payloadItems := strings.Split(string(payloadRaw), "|")
	if len(payloadItems) != 5 {
		t.Fatalf("invalid payload: %s", string(payloadRaw))
	}
	expUnix, err := strconv.ParseInt(payloadItems[4], 10, 64)
	if err != nil {
		t.Fatalf("invalid exp: %v", err)
	}
	return time.Unix(expUnix, 0)
}

type testUserRepoService struct {
	byEmail   map[string]model.User
	byID      map[string]model.User
	updateErr error
	createErr error
}

func newTestUserRepoService() *testUserRepoService {
	return &testUserRepoService{
		byEmail: map[string]model.User{},
		byID:    map[string]model.User{},
	}
}

func (r *testUserRepoService) Create(ctx context.Context, user model.User) (model.User, error) {
	if r.createErr != nil {
		return model.User{}, r.createErr
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := r.byEmail[email]; exists {
		return model.User{}, repository.ErrEmailExists
	}
	user.Email = email
	r.byEmail[email] = user
	r.byID[user.UserID] = user
	return user, nil
}

func (r *testUserRepoService) FindByEmail(ctx context.Context, email string) (model.User, error) {
	user, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *testUserRepoService) FindByID(ctx context.Context, userID string) (model.User, error) {
	user, ok := r.byID[userID]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *testUserRepoService) Update(ctx context.Context, user model.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.byID[user.UserID]; !ok {
		return repository.ErrUserNotFound
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	user.Email = email
	r.byID[user.UserID] = user
	r.byEmail[email] = user
	return nil
}

func signedToken(secret, tokenID, tokenType, userID, role string, expUnix int64) string {
	payload := strings.Join([]string{tokenID, tokenType, userID, role, strconv.FormatInt(expUnix, 10)}, "|")
	payloadEncoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadEncoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadEncoded + "." + sig
}
