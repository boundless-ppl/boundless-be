package debug_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"boundless-be/debug"
)

func TestProtectedDebugEndpointAllowsLoopback(t *testing.T) {
	handler := debugTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestProtectedDebugEndpointRejectsPublicIP(t *testing.T) {
	handler := debugTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func debugTestHandler() http.Handler {
	mux := http.NewServeMux()
	debug.RegisterPprofRoutesForTest(mux)
	return mux
}
