package dto_test

import (
	"encoding/json"
	"testing"

	"boundless-be/dto"
)

func TestRegisterRequestJsonMappingDto(t *testing.T) {
	payload := `{"nama_lengkap":"Alice Doe","email":"alice@example.com","password":"Secret123!"}`
	var req dto.RegisterRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if req.NamaLengkap != "Alice Doe" || req.Email != "alice@example.com" || req.Password != "Secret123!" {
		t.Fatal("unexpected register request mapping")
	}
}

func TestLoginRequestJsonMappingDto(t *testing.T) {
	payload := `{"email":"alice@example.com","password":"Secret123!"}`
	var req dto.LoginRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if req.Email != "alice@example.com" || req.Password != "Secret123!" {
		t.Fatal("unexpected login request mapping")
	}
}

func TestAuthResponseJsonMappingDto(t *testing.T) {
	raw, err := json.Marshal(dto.AuthResponse{
		AccessToken:  "a",
		RefreshToken: "r",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(raw) != `{"access_token":"a","refresh_token":"r"}` {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

func TestErrorResponseJsonMappingDto(t *testing.T) {
	raw, err := json.Marshal(dto.ErrorResponse{Error: "authentication failed"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(raw) != `{"error":"authentication failed"}` {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}
