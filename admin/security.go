package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	adminCSRFCookieName = "goadmin_csrf"
	maxAdminNoteLength  = 1000
)

func applyAdminSecurityHeaders(ctx *gin.Context) {
	ctx.Header("X-Frame-Options", "DENY")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob:; frame-ancestors 'none'")
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setAdminCSRFCookie(ctx *gin.Context, token string) {
	secure := ctx.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")), "https")
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     adminCSRFCookieName,
		Value:    token,
		Path:     "/" + urlPrefix,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   15 * 60,
	})
}

func validateAdminCSRF(ctx *gin.Context) bool {
	cookie, err := ctx.Request.Cookie(adminCSRFCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false
	}

	formToken := strings.TrimSpace(ctx.PostForm("csrf_token"))
	if formToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formToken)) == 1
}

func validatePaymentStatusUpdateInput(paymentID, status, adminNote string) (string, string, *string, error) {
	paymentID = strings.TrimSpace(paymentID)
	if _, err := uuid.Parse(paymentID); err != nil {
		return "", "", nil, fmt.Errorf("invalid payment id")
	}

	status = strings.ToLower(strings.TrimSpace(status))
	if status != "success" && status != "failed" {
		return "", "", nil, fmt.Errorf("invalid payment status")
	}

	adminNote = strings.TrimSpace(adminNote)
	if adminNote == "" {
		return paymentID, status, nil, nil
	}

	if !utf8.ValidString(adminNote) {
		return "", "", nil, fmt.Errorf("invalid admin note encoding")
	}
	if utf8.RuneCountInString(adminNote) > maxAdminNoteLength {
		return "", "", nil, fmt.Errorf("admin note too long")
	}

	return paymentID, status, &adminNote, nil
}
