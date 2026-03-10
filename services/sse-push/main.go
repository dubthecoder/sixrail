package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/teclara/railsix/shared/bus"
	"github.com/teclara/railsix/shared/config"
)

// natsToSSE maps NATS subjects to SSE event names.
var natsToSSE = map[string]string{
	"transit.alerts":           "alerts",
	"transit.trip-updates":     "trip-updates",
	"transit.service-glance":   "service-glance",
	"transit.exceptions":       "exceptions",
	"transit.union-departures": "union-departures",
}

func main() {
	natsURL := config.EnvOr(config.EnvNATSURL, config.DefaultNATSURL)
	port := config.EnvOr(config.EnvPort, "8085")
	allowedOrigins := parseOrigins(os.Getenv(config.EnvAllowedOrigins))

	// Connect to NATS.
	nc, err := bus.Connect(natsURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("connected to NATS", "url", natsURL)

	broker := NewBroker()

	// Subscribe to each NATS subject and fan out to the broker.
	for subject, eventName := range natsToSSE {
		name := eventName // capture for closure
		if err := bus.Subscribe(nc, subject, func(data []byte) {
			broker.Broadcast(SSEEvent{Name: name, Data: data})
		}); err != nil {
			slog.Error("failed to subscribe to NATS subject", "subject", subject, "error", err)
			os.Exit(1)
		}
		slog.Info("subscribed to NATS subject", "subject", subject, "sse_event", name)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sse", sseHandler(broker, allowedOrigins))
	mux.HandleFunc("GET /health", healthHandler(broker))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("sse-push listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func sseHandler(broker *Broker, allowedOrigins map[string]struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// CORS.
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, allowed := allowedOrigins[origin]; allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		ch := broker.Subscribe()
		defer broker.Unsubscribe(ch)

		slog.Info("SSE client connected", "clients", broker.ClientCount())

		for {
			select {
			case <-r.Context().Done():
				slog.Info("SSE client disconnected", "clients", broker.ClientCount()-1)
				return
			case evt := <-ch:
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Name, evt.Data)
				flusher.Flush()
			}
		}
	}
}

func healthHandler(broker *Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"clients": broker.ClientCount(),
		})
	}
}

func parseOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	if raw == "" {
		origins["http://localhost:5173"] = struct{}{}
		return origins
	}
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = struct{}{}
		}
	}
	return origins
}
