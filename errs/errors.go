package errs

import "errors"

var (
	ErrInvalidInput             = errors.New("invalid input")
	ErrUniversityNotFound       = errors.New("university not found")
	ErrUnauthorized             = errors.New("unauthorized")
	ErrSubmissionNotFound       = errors.New("submission not found")
	ErrForbidden                = errors.New("forbidden")
	ErrNoDocumentProvided       = errors.New("no document provided")
	ErrExternalService          = errors.New("external service error")
	ErrDocumentNotFound         = errors.New("document not found")
	ErrDreamTrackerNotFound     = errors.New("dream tracker not found")
	ErrDreamRequirementNotFound = errors.New("dream requirement not found")
)
