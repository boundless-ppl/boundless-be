package model_test

import (
	"errors"
	"testing"
	"time"

	"boundless-be/model"
)

func TestNewUserSuccessModel(t *testing.T) {
	user, err := model.NewUser("u1", "Alice Doe", "admin", "alice@example.com", "hashed")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if user.UserID != "u1" || user.NamaLengkap != "Alice Doe" || user.Role != "admin" || user.Email != "alice@example.com" || user.PasswordHash != "hashed" {
		t.Fatal("unexpected user value")
	}
	if user.CreatedAt.IsZero() || user.CreatedAt.After(time.Now()) {
		t.Fatal("unexpected created at")
	}
}

func TestNewUserInvalidEmailModel(t *testing.T) {
	_, err := model.NewUser("u1", "Alice Doe", "admin", "alice", "hashed")
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", model.ErrInvalidInput, err)
	}
}

func TestNewUserEmptyValueModel(t *testing.T) {
	_, err := model.NewUser("", "Alice Doe", "admin", "alice@example.com", "hashed")
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("expected %v, got %v", model.ErrInvalidInput, err)
	}
}

func TestPasswordComplexityValidModel(t *testing.T) {
	if !model.IsPasswordComplex("Secret123!") {
		t.Fatal("expected password to be complex")
	}
}

func TestPasswordComplexityInvalidModel(t *testing.T) {
	if model.IsPasswordComplex("secret") {
		t.Fatal("expected password to be invalid")
	}
}
