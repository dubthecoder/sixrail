package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/teclara/sixrail/api/internal/config"
	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
	"github.com/teclara/sixrail/api/internal/handlers"
	"github.com/teclara/sixrail/api/internal/metrolinx"
)

func main() {
	cfg := config.Load()

	// Download and parse GTFS static data
	slog.Info("downloading GTFS static data", "url", cfg.GTFSStaticURL)
	zipData, err := downloadURL(cfg.GTFSStaticURL)
	if err != nil {
		slog.Error("failed to download GTFS static data", "error", err)
		os.Exit(1)
	}
	static, err := gtfsstore.NewStaticStore(zipData)
	if err != nil {
		slog.Error("failed to parse GTFS static data", "error", err)
		os.Exit(1)
	}

	// Start daily GTFS refresh
	go refreshLoop(cfg.GTFSStaticURL, static, 24*time.Hour)

	// Realtime cache + background pollers
	rtCache := gtfsstore.NewRealtimeCache()
	ctx := context.Background()
	var mxClient *metrolinx.Client
	if cfg.MetrolinxAPIKey == "" {
		slog.Info("METROLINX_API_KEY not set — using simulated vehicle positions")
		gtfsstore.StartSimulatedPositionPoller(ctx, static, rtCache, 10*time.Second)
	} else {
		mxClient = metrolinx.NewClient(cfg.MetrolinxBaseURL, cfg.MetrolinxAPIKey)
		// Validate API key by fetching AND parsing a GTFS-RT feed
		testData, err := mxClient.Fetch(ctx, "/Gtfs/Feed/VehiclePosition")
		if err != nil {
			slog.Warn("Metrolinx API fetch failed, falling back to simulated positions", "error", err)
			gtfsstore.StartSimulatedPositionPoller(ctx, static, rtCache, 10*time.Second)
			mxClient = nil
		} else if _, parseErr := gtfsstore.ParsePositions(testData); parseErr != nil {
			slog.Warn("Metrolinx API returned invalid JSON, falling back to simulated positions", "error", parseErr)
			gtfsstore.StartSimulatedPositionPoller(ctx, static, rtCache, 10*time.Second)
			mxClient = nil
		} else {
			slog.Info("Metrolinx API key validated, starting real-time pollers")
			gtfsstore.StartPositionPoller(ctx, mxClient, static, rtCache, 10*time.Second)
			gtfsstore.StartAlertPoller(ctx, mxClient, static, rtCache, 30*time.Second)
			gtfsstore.StartTripUpdatePoller(ctx, mxClient, rtCache, 30*time.Second)
			gtfsstore.StartServiceGlancePoller(ctx, mxClient, rtCache, 30*time.Second)
			gtfsstore.StartExceptionsPoller(ctx, mxClient, rtCache, 60*time.Second)
		}
	}

	h := handlers.New(static, rtCache, mxClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/stops", h.AllStops)
	mux.HandleFunc("GET /api/departures/{stopCode}", h.StopDepartures)
	mux.HandleFunc("GET /api/union-departures", h.UnionDepartures)
	mux.HandleFunc("GET /api/shapes", h.RouteShapes)
	mux.HandleFunc("GET /api/trip/{tripId}", h.TripDetail)
	mux.HandleFunc("GET /api/positions", h.Positions)
	mux.HandleFunc("GET /api/alerts", h.Alerts)
	mux.HandleFunc("GET /api/network-health", h.NetworkHealth)
	mux.HandleFunc("GET /api/fares/{from}/{to}", h.Fares)

	handler := corsMiddleware(cfg.AllowedOrigins, mux)

	slog.Info("starting sixrail-api", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func downloadURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	const maxBytes = 50 * 1024 * 1024 // 50 MB
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

func refreshLoop(url string, static *gtfsstore.StaticStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		slog.Info("refreshing GTFS static data")
		data, err := downloadURL(url)
		if err != nil {
			slog.Error("failed to download GTFS refresh", "error", err)
			continue
		}
		if err := static.Refresh(data); err != nil {
			slog.Error("failed to parse GTFS refresh", "error", err)
		}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
