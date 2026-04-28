package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"boundless-be/api"
	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"golang.org/x/crypto/bcrypt"
)

type testUserRepo struct {
	byEmail map[string]model.User
	byID    map[string]model.User
}

func newTestUserRepo() *testUserRepo {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("Secret123!"), bcrypt.DefaultCost)
	admin := model.User{UserID: "admin-1", NamaLengkap: "Admin", Role: "admin", Email: "admin@example.com", PasswordHash: string(hashed), CreatedAt: time.Now().UTC()}
	return &testUserRepo{
		byEmail: map[string]model.User{"admin@example.com": admin},
		byID:    map[string]model.User{"admin-1": admin},
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
	r.byID[user.UserID] = user
	r.byEmail[strings.ToLower(strings.TrimSpace(user.Email))] = user
	return nil
}

type testPaymentRepo struct {
	subscription model.Subscription
	payments     map[string]model.Payment
}

func newTestPaymentRepo() *testPaymentRepo {
	return &testPaymentRepo{
		subscription: model.Subscription{SubscriptionID: "11111111-1111-1111-1111-111111111111", PackageKey: "scholar", Name: "The Scholar", DurationMonths: 3, PriceAmount: 100, Benefits: []string{"a"}, IsActive: true},
		payments:     map[string]model.Payment{},
	}
}

func (r *testPaymentRepo) ListActiveSubscriptions(ctx context.Context) ([]model.Subscription, error) {
	return []model.Subscription{r.subscription}, nil
}
func (r *testPaymentRepo) FindActiveSubscriptionByID(ctx context.Context, subscriptionID string) (model.Subscription, error) {
	if subscriptionID != r.subscription.SubscriptionID {
		return model.Subscription{}, errs.ErrSubscriptionNotFound
	}
	return r.subscription, nil
}
func (r *testPaymentRepo) CreatePayment(ctx context.Context, payment model.Payment) (model.Payment, error) {
	r.payments[payment.PaymentID] = payment
	return payment, nil
}
func (r *testPaymentRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	return doc, nil
}
func (r *testPaymentRepo) FindPaymentByID(ctx context.Context, paymentID string) (model.Payment, error) {
	payment, ok := r.payments[paymentID]
	if !ok {
		return model.Payment{}, errs.ErrPaymentNotFound
	}
	return payment, nil
}
func (r *testPaymentRepo) FindPaymentByIDAndUser(ctx context.Context, paymentID, userID string) (model.Payment, error) {
	payment, ok := r.payments[paymentID]
	if !ok || payment.UserID != userID {
		return model.Payment{}, errs.ErrPaymentNotFound
	}
	return payment, nil
}
func (r *testPaymentRepo) FindUserSubscriptionByPaymentID(ctx context.Context, paymentID, userID string) (model.UserSubscription, error) {
	return model.UserSubscription{}, sql.ErrNoRows
}
func (r *testPaymentRepo) FindPremiumCoverageEndAt(ctx context.Context, userID string, reference time.Time) (*time.Time, error) {
	return nil, nil
}
func (r *testPaymentRepo) FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error) {
	return model.UserSubscription{}, errs.ErrPremiumSubscriptionNotFound
}
func (r *testPaymentRepo) ListAdminPayments(ctx context.Context, params repository.PaymentListParams) ([]repository.AdminPaymentItem, error) {
	return []repository.AdminPaymentItem{}, nil
}
func (r *testPaymentRepo) ListPendingPaymentNotifications(ctx context.Context, limit int) ([]repository.PendingPaymentNotification, error) {
	return nil, nil
}
func (r *testPaymentRepo) AttachPaymentProofDocument(ctx context.Context, paymentID, userID, documentID string) error {
	return nil
}
func (r *testPaymentRepo) MarkPaymentNotificationSent(ctx context.Context, paymentID string, notifiedAt time.Time) error {
	return nil
}
func (r *testPaymentRepo) MarkPaymentSuccess(ctx context.Context, params repository.MarkPaymentSuccessParams) (model.Payment, model.UserSubscription, error) {
	return model.Payment{}, model.UserSubscription{}, nil
}
func (r *testPaymentRepo) MarkPaymentFailed(ctx context.Context, params repository.MarkPaymentFailedParams) (model.Payment, error) {
	return model.Payment{}, nil
}

func TestMain(m *testing.M) {
	os.Setenv("AUTH_SECRET", "test-secret")
	os.Setenv("CORS_ALLOWED_ORIGINS", "*")
	os.Setenv("DOCUMENT_STORAGE_PROVIDER", "local")
	os.Setenv("DOCUMENT_STORAGE_DIR", os.TempDir())
	os.Exit(m.Run())
}

func TestPaymentRoutesListPackagesApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})
	req := httptest.NewRequest(http.MethodGet, "/subscriptions/packages", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestPaymentCreateUnauthorizedApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestPaymentCreateAndGetApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})

	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"nama_lengkap":"User","email":"user@example.com","password":"Secret123!"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	handler.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, regRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"user@example.com","password":"Secret123!"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, loginRec.Code)
	}

	var tokens dto.AuthResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("unmarshal tokens: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"subscription_id":"11111111-1111-1111-1111-111111111111"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, createRec.Code)
	}

	var created dto.CreatePaymentResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal payment response: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+created.PaymentID, nil)
	getReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, getRec.Code)
	}
}

func TestAdminPaymentForbiddenForUserApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})
	tokens, _ := service.NewHMACTokenManager("test-secret").IssueTokens("user-1", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/payments", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestAdminPaymentStatusForbiddenForUserApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})
	tokens, _ := service.NewHMACTokenManager("test-secret").IssueTokens("user-1", "user")
	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestAdminPaymentStatusUnauthorizedWithoutTokenApi(t *testing.T) {
	handler := api.NewHandler(api.Dependencies{UserRepo: newTestUserRepo(), PaymentRepo: newTestPaymentRepo()})
	req := httptest.NewRequest(http.MethodPatch, "/admin/payments/11111111-1111-1111-1111-111111111111/status", bytes.NewBufferString(`{"status":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
