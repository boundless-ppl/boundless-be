package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
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
	userRepo       repository.UserRepository
	secretKey      []byte
	mu             sync.RWMutex
	revokedTokenID map[string]time.Time
}

func NewAuthService(userRepo repository.UserRepository) *AuthService {
	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		log.Fatal("AUTH_SECRET environment variable is required")
	}

	return &AuthService{
		userRepo:       userRepo,
		secretKey:      []byte(secret),
		revokedTokenID: map[string]time.Time{},
	}
}

func (s *AuthService) Register(ctx context.Context, fullName, role, email, password string) (AuthTokens, error) {
	if !model.IsPasswordComplex(password) {
		return AuthTokens{}, ErrInvalidInput
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return AuthTokens{}, ErrInvalidInput
	}

	userID := newID()
	user, err := model.NewUser(userID, fullName, role, email, passwordHash)
	if err != nil {
		return AuthTokens{}, ErrInvalidInput
	}

	if _, err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			return AuthTokens{}, repository.ErrEmailExists
		}
		return AuthTokens{}, ErrInvalidInput
	}

	return s.issueTokens(user.UserID, user.Role)
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
		if updateErr := s.userRepo.Update(ctx, user); updateErr != nil {
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

	return s.issueTokens(user.UserID, user.Role)
}

func (s *AuthService) ValidateToken(token string) (string, error) {
	claims, err := s.ValidateAccessToken(token)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func (s *AuthService) ValidateAccessToken(token string) (TokenClaims, error) {
	claims, err := s.parseToken(token)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.TokenType != "access" {
		return TokenClaims{}, ErrInvalidToken
	}
	if s.isRevoked(claims.TokenID) {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.ExpiresAt.Before(time.Now()) {
		return TokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}

func (s *AuthService) Logout(token string) error {
	claims, err := s.parseToken(token)
	if err != nil {
		return ErrInvalidToken
	}
	s.mu.Lock()
	now := time.Now()
	for id, exp := range s.revokedTokenID {
		if exp.Before(now) {
			delete(s.revokedTokenID, id)
		}
	}
	s.revokedTokenID[claims.TokenID] = claims.ExpiresAt
	s.mu.Unlock()
	return nil
}

func (s *AuthService) issueTokens(userID, role string) (AuthTokens, error) {
	access, err := s.createToken("access", userID, role, AccessTokenDuration)
	if err != nil {
		return AuthTokens{}, ErrInvalidInput
	}
	refresh, err := s.createToken("refresh", userID, role, RefreshTokenDuration)
	if err != nil {
		return AuthTokens{}, ErrInvalidInput
	}
	return AuthTokens{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (s *AuthService) createToken(tokenType, userID, role string, duration time.Duration) (string, error) {
	exp := time.Now().Add(duration).Unix()
	tokenID := newID()
	payload := strings.Join([]string{tokenID, tokenType, userID, role, strconv.FormatInt(exp, 10)}, "|")
	payloadEncoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := sign(s.secretKey, payloadEncoded)
	return payloadEncoded + "." + sig, nil
}

func (s *AuthService) parseToken(token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return TokenClaims{}, ErrInvalidToken
	}
	payloadEncoded := parts[0]
	sig := parts[1]
	if sign(s.secretKey, payloadEncoded) != sig {
		return TokenClaims{}, ErrInvalidToken
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}

	items := strings.Split(string(payloadRaw), "|")
	if len(items) != 5 {
		return TokenClaims{}, ErrInvalidToken
	}

	expUnix, err := strconv.ParseInt(items[4], 10, 64)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}

	return TokenClaims{
		TokenID:   items[0],
		TokenType: items[1],
		UserID:    items[2],
		Role:      items[3],
		ExpiresAt: time.Unix(expUnix, 0),
	}, nil
}

func (s *AuthService) isRevoked(tokenID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.revokedTokenID[tokenID]
	return exists
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

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(