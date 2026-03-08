// api/internal/config/config_test.go
package config_test

import (
	"testing"

	"github.com/dubthecoder/railsix/api/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := config.Load()
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.MetrolinxBaseURL != "https://api.openmetrolinx.com/OpenDataAPI/api/V1" {
		t.Fatalf("unexpected base URL: %s", cfg.MetrolinxBaseURL)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("METROLINX_API_KEY", "test-key")
	t.Setenv("ALLOWED_ORIGINS", "https://example.com")

	cfg := config.Load()
	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.MetrolinxAPIKey != "test-key" {
		t.Fatalf("expected api key test-key, got %s", cfg.MetrolinxAPIKey)
	}
	if cfg.AllowedOrigins != "https://example.com" {
		t.Fatalf("expected allowed origins, got %s", cfg.AllowedOrigins)
	}
}
