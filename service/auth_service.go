package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"boundless-be/model"
	"boundless-be/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidInput       = errors.New("invalid input")
	ErrAccountLocked      = errors.New("account locked")
)

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 24 * time.Hour
	LockWindowDuration   = 5 * time.Minute
	LockDuration         = 15 * time.Minute
	MaxFailedAttempts    = 5
)

type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

type TokenClaims struct {
	TokenID   string
	TokenType string
	UserID    string
	Role      string
	ExpiresAt time.Time
}

type AuthService struct {
	userRepo      repository.UserRepository
	tokenProvider *HMACTokenManager
}

func NewAuthService(userRepo repository.UserRepository) *AuthService {
	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		log.Fatal("AUTH_SECRET environment variable is required")
	}

	return &AuthService{
		userRepo:      userRepo,
		tokenProvider: NewHMACTokenManager(secret),
	}
}

func (s *AuthService) Register(ctx context.Context, fullName, role, email, password string) error {
	if !model.IsPasswordComplex(password) {
		return ErrInvalidInput
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return ErrInvalidInput
	}

	userID := newID()
	user, err := model.NewUser(userID, fullName, role, email, passwordHash)
	if err != nil {
		return ErrInvalidInput
	}

	if _, err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			return repository.ErrEmailExists
		}
		return ErrInvalidInput
	}

	return nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (AuthTokens, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return AuthTokens{}, ErrInvalidCredentials
	}

	now := time.Now()
	if user.LockedUntil.After(now) {
		return AuthTokens{}, ErrAccountLocked
	}

	if !checkPassword(user.PasswordHash, password) {
		user = trackFailedLogin(user, now)
		if err := s.userRepo.Update(ctx, user); err != nil {
			return AuthTokens{}, ErrInvalidCredentials
		}
		if user.LockedUntil.After(now) {
			return AuthTokens{}, ErrAccountLocked
		}
		return AuthTokens{}, ErrInvalidCredentials
	}

	user.FailedLoginCount = 0
	user.FirstFailedAt = time.Time{}
	user.LockedUntil = time.Time{}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return AuthTokens{}, ErrInvalidCredentials
	}

	return s.tokenProvider.IssueTokens(user.UserID, user.Role)
}

func (s *AuthService) ValidateToken(token string) (string, error) {
	return s.tokenProvider.ValidateToken(token)
}

func (s *AuthService) ValidateAccessToken(token string) (TokenClaims, error) {
	return s.tokenProvider.ValidateAccessToken(token)
}

func (s *AuthService) Logout(token string) error {
	return s.tokenProvider.Revoke(token)
}

func trackFailedLogin(user model.User, now time.Time) model.User {
	if user.FirstFailedAt.IsZero() || now.Sub(user.FirstFailedAt) > LockWindowDuration {
		user.FirstFailedAt = now
		user.FailedLoginCount = 1
		return user
	}

	user.FailedLoginCount++
	if user.FailedLoginCount >= MaxFailedAttempts {
		user.LockedUntil = now.Add(LockDuration)
		user.FailedLoginCount = 0
		user.FirstFailedAt = time.Time{}
	}
	return user
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func checkPassword(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
