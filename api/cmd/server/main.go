package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/teclara/railsix/api/internal/config"
	gtfsstore "github.com/teclara/railsix/api/internal/gtfs"
	"github.com/teclara/railsix/api/internal/handlers"
	"github.com/teclara/railsix/api/internal/metrolinx"
)

func main() {
	cfg := config.Load()

	// Use signal context for graceful shutdown of startup loops and pollers.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	static := gtfsstore.NewEmptyStaticStore()

	// Load GTFS in background so the server starts immediately, but keep retrying
	// until the store is actually ready instead of falling into a 24-hour gap.
	go manageGTFS(ctx, cfg.GTFSStaticURL, static, 24*time.Hour)

	// Realtime cache + background pollers
	rtCache := gtfsstore.NewRealtimeCache()
	var mxClient *metrolinx.Client
	if cfg.MetrolinxAPIKey == "" {
		slog.Info("METROLINX_API_KEY not set — real-time data unavailable")
	} else {
		mxClient = metrolinx.NewClient(cfg.MetrolinxBaseURL, cfg.MetrolinxAPIKey)
		// Validate API key by fetching a GTFS-RT feed
		testData, err := mxClient.Fetch(ctx, "/Gtfs/Feed/Alerts")
		if err != nil {
			slog.Warn("Metrolinx API fetch failed, real-time data unavailable", "error", err)
			mxClient = nil
		} else if _, parseErr := gtfsstore.ParseAlerts(testData); parseErr != nil {
			slog.Warn("Metrolinx API returned invalid data, real-time data unavailable", "error", parseErr)
			mxClient = nil
		} else {
			slog.Info("Metrolinx API key validated, starting real-time pollers")
			gtfsstore.StartAlertPoller(ctx, mxClient, static, rtCache, 30*time.Second)
			gtfsstore.StartTripUpdatePoller(ctx, mxClient, rtCache, 30*time.Second)
			gtfsstore.StartServiceGlancePoller(ctx, mxClient, rtCache, 30*time.Second)
			gtfsstore.StartExceptionsPoller(ctx, mxClient, rtCache, 60*time.Second)
			gtfsstore.StartUnionDeparturesPoller(ctx, mxClient, rtCache, 30*time.Second)
		}
	}

	h := handlers.New(static, rtCache, mxClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/ready", h.Ready)
	mux.HandleFunc("GET /api/stops", h.AllStops)
	mux.HandleFunc("GET /api/departures/{stopCode}", h.StopDepartures)
	mux.HandleFunc("GET /api/union-departures", h.UnionDepartures)
	mux.HandleFunc("GET /api/alerts", h.Alerts)
	mux.HandleFunc("GET /api/network-health", h.NetworkHealth)
	mux.HandleFunc("GET /api/fares/{from}/{to}", h.Fares)

	var handler http.Handler = mux
	handler = corsMiddleware(cfg.AllowedOrigins, handler)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting railsix-api", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

var allowedGTFSHosts = map[string]bool{
	"assets.metrolinx.com":   true,
	"www.metrolinx.com":      true,
	"metrolinx.com":          true,
	"api.openmetrolinx.com":  true,
	"opendata.metrolinx.com": true,
	"gtfs.metrolinx.com":     true,
}

func downloadURL(rawURL string) ([]byte, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("invalid or non-HTTP(S) URL: %s", rawURL)
	}
	if !allowedGTFSHosts[parsed.Hostname()] {
		return nil, fmt.Errorf("host %q not in GTFS allowlist", parsed.Hostname())
	}
	client := &http.Client{Timeout: 60 * time.Second}
	cleanURL := parsed.String()
	resp, err := client.Get(cleanURL) //nolint:G107 // URL validated: scheme + hostname allowlist checked above
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, rawURL)
	}
	const maxBytes = 50 * 1024 * 1024 // 50 MB
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

// manageGTFS keeps retrying startup loads until the store becomes ready, then
// switches to the long-lived daily refresh loop.
func manageGTFS(ctx context.Context, url string, store *gtfsstore.StaticStore, refreshInterval time.Duration) {
	const startupRetryDelay = time.Minute

	for {
		if loadGTFSIntoStore(ctx, url, 5, store) {
			refreshLoop(ctx, url, store, refreshInterval)
			return
		}

		slog.Warn("GTFS static data unavailable after startup attempts, retrying soon", "retryIn", startupRetryDelay)
		if !sleepContext(ctx, startupRetryDelay) {
			return
		}
	}
}

// loadGTFSIntoStore attempts to download and parse GTFS data with exponential
// backoff, loading it into the provided store via Refresh. It returns true once
// the store is ready and false if the attempt batch failed or the context was
// cancelled.
func loadGTFSIntoStore(ctx context.Context, url string, maxAttempts int, store *gtfsstore.StaticStore) bool {
	backoff := 2 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return false
		}

		slog.Info("downloading GTFS static data", "url", url, "attempt", attempt, "maxAttempts", maxAttempts)
		zipData, err := downloadURL(url)
		if err != nil {
			slog.Error("failed to download GTFS static data", "error", err, "attempt", attempt)
			if attempt < maxAttempts {
				slog.Info("retrying GTFS download", "backoff", backoff)
				if !sleepContext(ctx, backoff) {
					return false
				}
				backoff *= 2
			}
			continue
		}

		if err := store.Refresh(zipData); err != nil {
			slog.Error("failed to parse GTFS static data", "error", err, "attempt", attempt)
			if attempt < maxAttempts {
				slog.Info("retrying GTFS download", "backoff", backoff)
				if !sleepContext(ctx, backoff) {
					return false
				}
				backoff *= 2
			}
			continue
		}

		slog.Info("GTFS static data loaded successfully")
		return true
	}

	return false
}

// refreshLoop periodically re-downloads GTFS static data. On failure it retries
// with exponential backoff (up to 3 attempts) before waiting for the next interval.
func refreshLoop(ctx context.Context, url string, static *gtfsstore.StaticStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("GTFS refresh loop stopped")
			return
		case <-ticker.C:
		}

		const maxRetries = 3
		backoff := 5 * time.Second
		var refreshed bool
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if ctx.Err() != nil {
				return
			}

			slog.Info("refreshing GTFS static data", "attempt", attempt)
			data, err := downloadURL(url)
			if err != nil {
				slog.Error("failed to download GTFS refresh", "error", err, "attempt", attempt)
				if attempt < maxRetries {
					if !sleepContext(ctx, backoff) {
						return
					}
					backoff *= 2
				}
				continue
			}

			if err := static.Refresh(data); err != nil {
				slog.Error("failed to parse GTFS refresh", "error", err, "attempt", attempt)
				if attempt < maxRetries {
					if !sleepContext(ctx, backoff) {
						return
					}
					backoff *= 2
				}
				continue
			}

			refreshed = true
			break
		}

		if !refreshed {
			slog.Warn("GTFS refresh failed after all retries, will try again next interval")
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func corsMiddleware(allowedOrigins string, next http.Handler) http.Handler {
	origins := strings.Split(allowedOrigins, ",")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		for _, o := range origins {
			if strings.TrimSpace(o) == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
