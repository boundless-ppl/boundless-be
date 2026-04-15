package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
)

type MeStatusRepository interface {
	FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error)
	FindLatestPendingPaymentByUser(ctx context.Context, userID string, reference time.Time) (model.Payment, error)
}

type MeStatus struct {
	IsPremium         bool
	PremiumStartAt    *string
	PremiumEndAt      *string
	HasPendingPayment bool
	TransactionID     *string
}

func BuildMeStatus(ctx context.Context, repo MeStatusRepository, userID string, now time.Time) (MeStatus, error) {
	status := MeStatus{}

	premiumSub, err := repo.FindCurrentPremiumSubscription(ctx, userID, now)
	if err != nil {
		if !errors.Is(err, errs.ErrPremiumSubscriptionNotFound) {
			return MeStatus{}, err
		}
	} else {
		status.IsPremium = true
		startAt := premiumSub.StartDate.UTC().Format(time.RFC3339)
		endAt := premiumSub.EndDate.UTC().Format(time.RFC3339)
		status.PremiumStartAt = &startAt
		status.PremiumEndAt = &endAt
	}

	pendingPayment, err := repo.FindLatestPendingPaymentByUser(ctx, userID, now)
	if err != nil {
		if !errors.Is(err, errs.ErrPaymentNotFound) && !errors.Is(err, sql.ErrNoRows) {
			return MeStatus{}, err
		}
		return status, nil
	}

	if pendingPayment.ProofDocumentID != nil && (pendingPayment.ExpiredAt == nil || pendingPayment.ExpiredAt.After(now)) {
		status.HasPendingPayment = true
		transactionID := pendingPayment.TransactionID
		status.TransactionID = &transactionID
	}

	return status, nil
}
