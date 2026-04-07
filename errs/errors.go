package errs

import "errors"

var (
	ErrInvalidInput                = errors.New("invalid input")
	ErrUniversityNotFound          = errors.New("university not found")
	ErrUnauthorized                = errors.New("unauthorized")
	ErrSubmissionNotFound          = errors.New("submission not found")
	ErrForbidden                   = errors.New("forbidden")
	ErrNoDocumentProvided          = errors.New("no document provided")
	ErrExternalService             = errors.New("external service error")
	ErrDocumentNotFound            = errors.New("document not found")
	ErrSubscriptionNotFound        = errors.New("subscription not found")
	ErrPaymentNotFound             = errors.New("payment not found")
	ErrPaymentNotPending           = errors.New("payment not pending")
	ErrInvalidPaymentStatus        = errors.New("invalid payment status")
	ErrPremiumSubscriptionNotFound = errors.New("premium subscription not found")
)
