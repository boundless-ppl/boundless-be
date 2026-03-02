package model

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

var ErrInvalidInput = errors.New("invalid input")

func NewUser(userID, fullName, role, email, passwordHash string) (User, error) {
	if userID == "" || strings.TrimSpace(fullName) == "" || strings.TrimSpace(role) == "" || strings.TrimSpace(passwordHash) == "" {
		return User{}, ErrInvalidInput
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(email)); err != nil {
		return User{}, ErrInvalidInput
	}

	return User{
		UserID:       userID,
		NamaLengkap:  strings.TrimSpace(fullName),
		Role:         strings.TrimSpace(role),
		Email:        strings.ToLower(strings.TrimSpace(email)),
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}, nil
}

func IsPasswordComplex(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasNumber = true
		default:
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}
