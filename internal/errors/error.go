package errors

import "net/http"

type AppError struct {
	Message string `json:"message"`
	Code    int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewNotFound(msg string) *AppError {
	return &AppError{
		Message: msg,
		Code:    http.StatusNotFound,
	}
}

func NewBadRequest(msg string) *AppError {
	return &AppError{
		Message: msg,
		Code:    http.StatusBadRequest,
	}
}

func NewInternal(msg string) *AppError {
	return &AppError{
		Message: msg,
		Code:    http.StatusInternalServerError,
	}
}
