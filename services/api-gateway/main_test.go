package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware("http://localhost:5173,http://example.com", inner)

	req := httptest.NewRequest("GET", "/api/stops", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected allowed origin, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware("http://localhost:5173", inner)

	req := httptest.NewRequest("GET", "/api/stops", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no ACAO header, got %q", got)
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for OPTIONS")
	})
	handler := corsMiddleware("http://localhost:5173", inner)

	req := httptest.NewRequest("OPTIONS", "/api/departures/UN", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestProxyHandler_UpstreamDown(t *testing.T) {
	client := &http.Client{}
	// Use a definitely-unreachable address
	handler := proxyHandler(client, "http://127.0.0.1:1", "/health")

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}
