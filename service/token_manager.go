package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HMACTokenManager struct {
	secretKey      []byte
	mu             sync.RWMutex
	revokedTokenID map[string]time.Time
}

func NewHMACTokenManager(secret string) *HMACTokenManager {
	return &HMACTokenManager{
		secretKey:      []byte(secret),
		revokedTokenID: map[string]time.Time{},
	}
}

func (m *HMACTokenManager) IssueTokens(userID, role string) (AuthTokens, error) {
	access := m.createToken("access", userID, role, AccessTokenDuration)
	refresh := m.createToken("refresh", userID, role, RefreshTokenDuration)
	return AuthTokens{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (m *HMACTokenManager) ValidateToken(token string) (string, error) {
	claims, err := m.ValidateAccessToken(token)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func (m *HMACTokenManager) ValidateAccessToken(token string) (TokenClaims, error) {
	claims, err := m.parseToken(token)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.TokenType != "access" {
		return TokenClaims{}, ErrInvalidToken
	}
	if m.isRevoked(claims.TokenID) {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.ExpiresAt.Before(time.Now()) {
		return TokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *HMACTokenManager) Revoke(token string) error {
	claims, err := m.parseToken(token)
	if err != nil {
		return ErrInvalidToken
	}
	m.mu.Lock()
	now := time.Now()
	for id, exp := range m.revokedTokenID {
		if exp.Before(now) {
			delete(m.revokedTokenID, id)
		}
	}
	m.revokedTokenID[claims.TokenID] = claims.ExpiresAt
	m.mu.Unlock()
	return nil
}

func (m *HMACTokenManager) createToken(tokenType, userID, role string, duration time.Duration) string {
	exp := time.Now().Add(duration).Unix()
	tokenID := newID()
	payload := strings.Join([]string{tokenID, tokenType, userID, role, strconv.FormatInt(exp, 10)}, "|")
	payloadEncoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := sign(m.secretKey, payloadEncoded)
	return payloadEncoded + "." + sig
}

func (m *HMACTokenManager) parseToken(token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return TokenClaims{}, ErrInvalidToken
	}
	payloadEncoded := parts[0]
	sig := parts[1]
	expectedSig := sign(m.secretKey, payloadEncoded)
	if !hmac.Equal([]byte(expectedSig), []byte(sig)) {
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

func (m *HMACTokenManager) isRevoked(tokenID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.revokedTokenID[tokenID]
	return exists
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
