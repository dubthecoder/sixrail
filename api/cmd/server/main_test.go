package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddleware_AllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	called := false
	handler := corsMiddleware("http://localhost:5173, https://railsix.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "https://railsix.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://railsix.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://railsix.com")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want %q", got, "Origin")
	}
}

func TestCORSMiddleware_OptionsRequestShortCircuits(t *testing.T) {
	t.Parallel()

	called := false
	handler := corsMiddleware("https://railsix.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/alerts", nil)
	req.Header.Set("Origin", "https://railsix.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("expected preflight request to skip next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, "GET, OPTIONS")
	}
}

func TestDownloadURL_RejectsInvalidScheme(t *testing.T) {
	t.Parallel()

	_, err := downloadURL("ftp://assets.metrolinx.com/GO-GTFS.zip")
	if err == nil {
		t.Fatal("expected invalid scheme to fail")
	}
	if !strings.Contains(err.Error(), "invalid or non-HTTP(S) URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadURL_RejectsDisallowedHost(t *testing.T) {
	t.Parallel()

	_, err := downloadURL("https://example.com/GO-GTFS.zip")
	if err == nil {
		t.Fatal("expected disallowed host to fail")
	}
	if !strings.Contains(err.Error(), "not in GTFS allowlist") {
		t.Fatalf("unexpected error: %v", err)
	}
}
