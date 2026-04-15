package service_test

import (
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/service"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type fakeMeStatusRepo struct {
	premiumSub     model.UserSubscription
	premiumErr     error
	pendingPayment model.Payment
	pendingErr     error
}

func (f *fakeMeStatusRepo) FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error) {
	if f.premiumErr != nil {
		return model.UserSubscription{}, f.premiumErr
	}
	if f.premiumSub.UserSubscriptionID == "" {
		return model.UserSubscription{}, errs.ErrPremiumSubscriptionNotFound
	}
	return f.premiumSub, nil
}

func (f *fakeMeStatusRepo) FindLatestPendingPaymentByUser(ctx context.Context, userID string, reference time.Time) (model.Payment, error) {
	if f.pendingErr != nil {
		return model.Payment{}, f.pendingErr
	}
	if f.pendingPayment.PaymentID == "" {
		return model.Payment{}, errs.ErrPaymentNotFound
	}
	return f.pendingPayment, nil
}

func TestBuildMeStatusPremiumAndPendingPaymentService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	endAt := now.Add(24 * time.Hour)
	repo := &fakeMeStatusRepo{
		premiumSub: model.UserSubscription{
			UserSubscriptionID: "us-1",
			StartDate:          time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			EndDate:            time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC),
		},
		pendingPayment: model.Payment{
			PaymentID:       "pay-1",
			TransactionID:   "TX-1",
			ProofDocumentID: ptrStringService("doc-1"),
			ExpiredAt:       &endAt,
		},
	}

	status, err := service.BuildMeStatus(context.Background(), repo, "u-1", now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !status.IsPremium {
		t.Fatal("expected premium user")
	}
	if status.PremiumStartAt == nil || status.PremiumEndAt == nil {
		t.Fatalf("expected premium dates, got %+v", status)
	}
	if !status.HasPendingPayment {
		t.Fatal("expected pending payment flag")
	}
	if status.TransactionID == nil || *status.TransactionID != "TX-1" {
		t.Fatalf("expected transaction id TX-1, got %+v", status.TransactionID)
	}
}

func TestBuildMeStatusNotFoundCasesService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	repo := &fakeMeStatusRepo{pendingErr: sql.ErrNoRows}

	status, err := service.BuildMeStatus(context.Background(), repo, "u-2", now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status.IsPremium || status.HasPendingPayment {
		t.Fatalf("expected empty status, got %+v", status)
	}
	if status.PremiumStartAt != nil || status.PremiumEndAt != nil || status.TransactionID != nil {
		t.Fatalf("expected nil pointers, got %+v", status)
	}
}

func TestBuildMeStatusReturnsSubscriptionErrorService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	repo := &fakeMeStatusRepo{premiumErr: errors.New("db subscription down")}

	_, err := service.BuildMeStatus(context.Background(), repo, "u-3", now)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}

func TestBuildMeStatusReturnsPendingPaymentErrorService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	repo := &fakeMeStatusRepo{pendingErr: errors.New("db payment down")}

	_, err := service.BuildMeStatus(context.Background(), repo, "u-4", now)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}

func TestBuildMeStatusIgnoresExpiredPendingPaymentService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	repo := &fakeMeStatusRepo{pendingPayment: model.Payment{
		PaymentID:       "pay-expired",
		TransactionID:   "TX-EXPIRED",
		ProofDocumentID: ptrStringService("doc-2"),
		ExpiredAt:       &expired,
	}}

	status, err := service.BuildMeStatus(context.Background(), repo, "u-5", now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status.HasPendingPayment || status.TransactionID != nil {
		t.Fatalf("expected no pending payment, got %+v", status)
	}
}

func TestBuildMeStatusIgnoresPendingWithoutProofService(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	repo := &fakeMeStatusRepo{pendingPayment: model.Payment{
		PaymentID:     "pay-no-proof",
		TransactionID: "TX-NO-PROOF",
		ExpiredAt:     ptrTimeService(now.Add(2 * time.Hour)),
	}}

	status, err := service.BuildMeStatus(context.Background(), repo, "u-6", now)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status.HasPendingPayment || status.TransactionID != nil {
		t.Fatalf("expected no pending payment, got %+v", status)
	}
}

func ptrStringService(value string) *string {
	return &value
}

func ptrTimeService(value time.Time) *time.Time {
	return &value
}
