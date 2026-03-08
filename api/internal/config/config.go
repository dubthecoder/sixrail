// api/internal/config/config.go
package config

import "os"

type Config struct {
	Port             string
	MetrolinxAPIKey  string
	MetrolinxBaseURL string
	AllowedOrigins   string
	GTFSStaticURL    string
	APIKey           string // optional: if set, all endpoints (except /api/health) require this key
}

func Load() Config {
	return Config{
		Port:             envOr("PORT", "8080"),
		MetrolinxAPIKey:  os.Getenv("METROLINX_API_KEY"),
		MetrolinxBaseURL: envOr("METROLINX_BASE_URL", "https://api.openmetrolinx.com/OpenDataAPI/api/V1"),
		AllowedOrigins:   envOr("ALLOWED_ORIGINS", "http://localhost:5173"),
		GTFSStaticURL:    envOr("GTFS_STATIC_URL", "https://assets.metrolinx.com/raw/upload/Documents/Metrolinx/Open%20Data/GO-GTFS.zip"),
		APIKey:           os.Getenv("API_KEY"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
