package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"boundless-be/api"
	"boundless-be/dto"
	"boundless-be/model"
	"boundless-be/repository"
)

func TestMain(m *testing.M) {
	os.Setenv("AUTH_SECRET", "test-secret")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")
	os.Exit(m.Run())
}

type testUserRepo struct {
	byEmail map[string]model.User
	byID    map[string]model.User
}

func newTestUserRepo() *testUserRepo {
	return &testUserRepo{
		byEmail: map[string]model.User{},
		byID:    map[string]model.User{},
	}
}

func (r *testUserRepo) Create(ctx context.Context, user model.User) (model.User, error) {
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := r.byEmail[email]; exists {
		return model.User{}, repository.ErrEmailExists
	}
	user.Email = email
	r.byEmail[email] = user
	r.byID[user.UserID] = user
	return user, nil
}

func (r *testUserRepo) FindByEmail(ctx context.Context, email string) (model.User, error) {
	user, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *testUserRepo) FindByID(ctx context.Context, userID string) (model.User, error) {
	user, ok := r.byID[userID]
	if !ok {
		return model.User{}, repository.ErrUserNotFound
	}
	return user, nil
}

func (r *testUserRepo) Update(ctx context.Context, user model.User) error {
	if _, ok := r.byID[user.UserID]; !ok {
		return repository.ErrUserNotFound
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	user.Email = email
	r.byID[user.UserID] = user
	r.byEmail[email] = user
	return nil
}

func TestRootRouteSuccessApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{
		UserRepo: newTestUserRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "hi\n" {
		t.Fatalf("expected body hi, got %q", string(body))
	}
}

func TestUnknownRouteReturnsNotFoundApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{
		UserRepo: newTestUserRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestFaviconRouteReturnsNoContentApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{
		UserRepo: newTestUserRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestLoginRedirectsToRootUnderThreeSecondsApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{
		UserRepo: newTestUserRepo(),
	})
	registerUser(t, handler, "admin@example.com", "admin")
	body, _ := json.Marshal(dto.LoginRequest{Email: "admin@example.com", Password: "Secret123!"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

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
	if elapsed > 3*time.Second {
		t.Fatalf("expected response under 3 seconds, got %v", elapsed)
	}
}

func TestLogoutRevokesTokenApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{
		UserRepo: newTestUserRepo(),
	})
	registerUser(t, handler, "admin@example.com", "admin")
	tokens := loginUser(t, handler, "admin@example.com")

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, logoutRec.Code)
	}

	reusedReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	reusedReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, reusedReq)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

// ── UpdateProfile ────────────────────────────────────────────────────────────

func TestUpdateProfileSuccessApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.UpdateProfileRequest{NamaLengkap: "Alice Updated"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d — body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestUpdateProfileReflectedInMeApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.UpdateProfileRequest{NamaLengkap: "Alice Updated"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)

	var got dto.MeResponse
	if err := json.Unmarshal(meRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.NamaLengkap != "Alice Updated" {
		t.Fatalf("expected updated name, got %s", got.NamaLengkap)
	}
}

func TestUpdateProfileUnauthorizedApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})

	body, _ := json.Marshal(dto.UpdateProfileRequest{NamaLengkap: "Alice Updated"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestUpdateProfileInvalidBodyApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ── ChangePassword ───────────────────────────────────────────────────────────

func TestChangePasswordSuccessApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "Secret123!", NewPassword: "NewSecret456@"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d — body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestChangePasswordNewPasswordWorksApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "Secret123!", NewPassword: "NewSecret456@"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	loginBody, _ := json.Marshal(dto.LoginRequest{Email: "alice@example.com", Password: "NewSecret456@"})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected new password to work, got status %d", loginRec.Code)
	}
}

func TestChangePasswordOldPasswordRejectedApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "Secret123!", NewPassword: "NewSecret456@"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	loginBody, _ := json.Marshal(dto.LoginRequest{Email: "alice@example.com", Password: "Secret123!"})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to be rejected, got status %d", loginRec.Code)
	}
}

func TestChangePasswordUnauthorizedApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "Secret123!", NewPassword: "NewSecret456@"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestChangePasswordInvalidBodyApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestChangePasswordWrongCurrentPasswordApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "WrongPass1!", NewPassword: "NewSecret456@"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestChangePasswordWeakNewPasswordApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo()})
	registerUser(t, handler, "alice@example.com", "user")
	tokens := loginUser(t, handler, "alice@example.com")

	body, _ := json.Marshal(dto.ChangePasswordRequest{CurrentPassword: "Secret123!", NewPassword: "weak"})
	req := httptest.NewRequest(http.MethodPut, "/auth/me/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func registerUser(t *testing.T, handler http.Handler, email, role string) {
	t.Helper()
	body, _ := json.Marshal(dto.RegisterRequest{
		NamaLengkap: "Test User",
		Email:       email,
		Password:    "Secret123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func loginUser(t *testing.T, handler http.Handler, email string) dto.AuthResponse {
	t.Helper()
	body, _ := json.Marshal(dto.LoginRequest{
		Email:    email,
		Password: "Secret123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var tokens dto.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	return tokens
}
