package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidatePaymentStatusUpdateInputSuccess(t *testing.T) {
	paymentID := "11111111-1111-1111-1111-111111111111"
	id, status, note, err := validatePaymentStatusUpdateInput(paymentID, " success ", " ok ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != paymentID {
		t.Fatalf("unexpected payment id: %s", id)
	}
	if status != "success" {
		t.Fatalf("unexpected status: %s", status)
	}
	if note == nil || *note != "ok" {
		t.Fatalf("unexpected note: %#v", note)
	}
}

func TestValidatePaymentStatusUpdateInputRejectsInvalidID(t *testing.T) {
	_, _, _, err := validatePaymentStatusUpdateInput("invalid", "success", "")
	if err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestValidatePaymentStatusUpdateInputRejectsInvalidStatus(t *testing.T) {
	_, _, _, err := validatePaymentStatusUpdateInput("11111111-1111-1111-1111-111111111111", "approved", "")
	if err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestValidatePaymentStatusUpdateInputRejectsLongNote(t *testing.T) {
	longNote := strings.Repeat("a", maxAdminNoteLength+1)
	_, _, _, err := validatePaymentStatusUpdateInput("11111111-1111-1111-1111-111111111111", "success", longNote)
	if err == nil {
		t.Fatal("expected long note error")
	}
}

func TestValidateAdminCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("POST", "/goadmin/payment-panel/status", strings.NewReader("csrf_token=abc123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req
	ctx.Request.AddCookie(&http.Cookie{Name: adminCSRFCookieName, Value: "abc123"})

	if !validateAdminCSRF(ctx) {
		t.Fatal("expected csrf token to be valid")
	}
}

func TestValidateAdminCSRFFailsWhenMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("POST", "/goadmin/payment-panel/status", strings.NewReader("csrf_token=abc123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req
	ctx.Request.AddCookie(&http.Cookie{Name: adminCSRFCookieName, Value: "xyz999"})

	if validateAdminCSRF(ctx) {
		t.Fatal("expected csrf token mismatch to fail")
	}
}

func TestApplyAdminSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/goadmin/payment-panel/status", nil)

	applyAdminSecurityHeaders(ctx)

	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %s", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff, got %s", got)
	}
}
