package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/teclara/railsix/shared/bus"
	"github.com/teclara/railsix/shared/cache"
	"github.com/teclara/railsix/shared/config"
	"github.com/teclara/railsix/shared/metrolinx"
	"github.com/teclara/railsix/shared/models"
)

const cacheTTL = 5 * time.Minute

func main() {
	apiKey, err := config.Require(config.EnvMetrolinxAPIKey)
	if err != nil {
		slog.Error("missing required config", "error", err)
		os.Exit(1)
	}

	baseURL := config.EnvOr(config.EnvMetrolinxBase, config.DefaultMetrolinxBase)
	natsURL := config.EnvOr(config.EnvNATSURL, config.DefaultNATSURL)
	redisAddr := config.EnvOr(config.EnvRedisAddr, config.DefaultRedisAddr)
	redisPassword := config.EnvOr(config.EnvRedisPassword, "")
	gtfsStaticAddr := config.EnvOr(config.EnvGTFSStaticAddr, config.DefaultGTFSStaticAddr)

	nc, err := bus.Connect(natsURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("connected to NATS", "url", natsURL)

	rc, err := cache.Connect(redisAddr, redisPassword)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rc.Close()
	slog.Info("connected to Redis", "addr", redisAddr)

	mx := metrolinx.NewClient(baseURL, apiKey)
	lookup := newHTTPRouteLookup(gtfsStaticAddr)
	healthRedis := newRedisReadiness(rc)

	healthPort := config.EnvOr(config.EnvPort, config.EnvOr("HEALTH_PORT", "8083"))
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", healthHandler(nc, healthRedis, lookup))
		slog.Info("health endpoint listening", "port", healthPort)
		if err := http.ListenAndServe(":"+healthPort, mux); err != nil {
			slog.Error("health server failed", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("realtime-poller started, polling every 30s")

	// Run first poll immediately
	var tickCount int
	runPollCycle(ctx, mx, lookup, nc, rc, tickCount)
	tickCount++

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down realtime-poller")
			return
		case <-ticker.C:
			runPollCycle(ctx, mx, lookup, nc, rc, tickCount)
			tickCount++
		}
	}
}

func runPollCycle(ctx context.Context, mx *metrolinx.Client, lookup RouteLookup, nc *nats.Conn, rc *redis.Client, tickCount int) {
	includeExceptions := tickCount%2 == 0
	result := pollAll(ctx, mx, lookup, includeExceptions)

	if result.hasAlerts {
		if err := cache.SetJSON(ctx, rc, "transit:alerts", result.alerts, cacheTTL); err != nil {
			slog.Error("cache alerts failed", "error", err)
		}
		if err := bus.Publish(nc, "transit.alerts", result.alerts); err != nil {
			slog.Error("publish alerts failed", "error", err)
		}
		if err := cache.SetTimestamp(ctx, rc, "transit:alerts:updated-at", cacheTTL); err != nil {
			slog.Error("cache alerts timestamp failed", "error", err)
		}
		slog.Info("polled alerts", "count", len(result.alerts))
	}

	if result.hasTripUpdates {
		if err := cache.SetHashJSON(ctx, rc, "transit:trip-updates", result.tripUpdates, cacheTTL); err != nil {
			slog.Error("cache trip updates failed", "error", err)
		}
		if err := bus.Publish(nc, "transit.trip-updates", result.tripUpdates); err != nil {
			slog.Error("publish trip updates failed", "error", err)
		}
		if err := cache.SetTimestamp(ctx, rc, "transit:trip-updates:updated-at", cacheTTL); err != nil {
			slog.Error("cache trip updates timestamp failed", "error", err)
		}
		slog.Info("polled trip updates", "count", len(result.tripUpdates))
	}

	if result.hasServiceGlance {
		glanceMap := make(map[string]models.ServiceGlanceEntry, len(result.serviceGlance))
		for _, e := range result.serviceGlance {
			glanceMap[e.TripNumber] = e
		}
		if err := cache.SetHashJSON(ctx, rc, "transit:service-glance", glanceMap, cacheTTL); err != nil {
			slog.Error("cache service glance failed", "error", err)
		}
		if err := bus.Publish(nc, "transit.service-glance", result.serviceGlance); err != nil {
			slog.Error("publish service glance failed", "error", err)
		}
		if err := cache.SetTimestamp(ctx, rc, "transit:service-glance:updated-at", cacheTTL); err != nil {
			slog.Error("cache service glance timestamp failed", "error", err)
		}
		slog.Info("polled service glance", "count", len(result.serviceGlance))
	}

	if result.hasExceptions {
		if err := cache.SetHashJSON(ctx, rc, "transit:exceptions", result.exceptions, cacheTTL); err != nil {
			slog.Error("cache exceptions failed", "error", err)
		}
		if err := bus.Publish(nc, "transit.exceptions", result.exceptions); err != nil {
			slog.Error("publish exceptions failed", "error", err)
		}
		if err := cache.SetTimestamp(ctx, rc, "transit:exceptions:updated-at", cacheTTL); err != nil {
			slog.Error("cache exceptions timestamp failed", "error", err)
		}
		slog.Info("polled exceptions", "count", len(result.exceptions))
	}

	if result.hasUnionDepartures {
		if err := cache.SetJSON(ctx, rc, "transit:union-departures", result.unionDepartures, cacheTTL); err != nil {
			slog.Error("cache union departures failed", "error", err)
		}
		if err := bus.Publish(nc, "transit.union-departures", result.unionDepartures); err != nil {
			slog.Error("publish union departures failed", "error", err)
		}
		if err := cache.SetTimestamp(ctx, rc, "transit:union-departures:updated-at", cacheTTL); err != nil {
			slog.Error("cache union departures timestamp failed", "error", err)
		}
		slog.Info("polled union departures", "count", len(result.unionDepartures))
	}
}
