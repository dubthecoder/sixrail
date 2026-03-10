package config

import (
	"fmt"
	"os"
)

// Environment variable name constants.
const (
	EnvNATSURL         = "NATS_URL"
	EnvRedisAddr       = "REDIS_ADDR"
	EnvRedisPassword   = "REDIS_PASSWORD"
	EnvMetrolinxAPIKey = "METROLINX_API_KEY"
	EnvMetrolinxBase   = "METROLINX_BASE_URL"
	EnvPort            = "PORT"
	EnvAllowedOrigins  = "ALLOWED_ORIGINS"
	EnvGTFSStaticURL   = "GTFS_STATIC_URL"
	EnvGTFSStaticAddr  = "GTFS_STATIC_ADDR"
	EnvDeparturesAddr  = "DEPARTURES_ADDR"
	EnvSSEPushAddr     = "SSE_PUSH_ADDR"
)

// Default values for environment variables.
const (
	DefaultNATSURL        = "nats://localhost:4222"
	DefaultRedisAddr      = "localhost:6379"
	DefaultMetrolinxBase  = "https://api.openmetrolinx.com/OpenDataAPI/api/V1"
	DefaultGTFSStaticURL  = "https://assets.metrolinx.com/raw/upload/Documents/Metrolinx/Open%20Data/GO-GTFS.zip"
	DefaultGTFSStaticAddr = "http://localhost:8081"
	DefaultDeparturesAddr = "http://localhost:8082"
	DefaultSSEPushAddr    = "http://localhost:8085"
)

// EnvOr returns the value of the environment variable named by key,
// or fallback if the variable is not set or empty.
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Require returns the value of the environment variable named by key,
// or an error if the variable is not set or empty.
func Require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
