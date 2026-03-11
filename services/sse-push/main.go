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
// Only subscribe to subjects the browser client actually uses.
// trip-updates, service-glance, and exceptions are consumed server-side
// via Redis by departures-api — no need to push them to browsers.
var natsToSSE = map[string]string{
	"transit.alerts":           "alerts",
	"transit.union-departures": "union-departures",
}

type natsConnection interface {
	IsConnected() bool
}

type brokerClientCounter interface {
	ClientCount() int
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
		if _, err := bus.Subscribe(nc, subject, func(data []byte) {
			broker.Broadcast(SSEEvent{Name: name, Data: data})
		}); err != nil {
			slog.Error("failed to subscribe to NATS subject", "subject", subject, "error", err)
			os.Exit(1)
		}
		slog.Info("subscribed to NATS subject", "subject", subject, "sse_event", name)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sse", sseHandler(broker, allowedOrigins))
	mux.HandleFunc("GET /health", healthHandler(broker, nc))

	srv := &http.Server{
		Addr:        ":" + port,
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
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

		// Send keepalive comments every 15s to prevent proxy/LB idle timeouts.
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case <-r.Context().Done():
				slog.Info("SSE client disconnected", "clients", broker.ClientCount()-1)
				return
			case evt := <-ch:
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Name, evt.Data)
				flusher.Flush()
			case <-keepalive.C:
				fmt.Fprint(w, ":keepalive\n\n")
				flusher.Flush()
			}
		}
	}
}

func healthHandler(broker brokerClientCounter, nc natsConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		status := "ok"
		checks := map[string]any{
			"nats": map[string]string{"status": "ok"},
		}
		if nc == nil || !nc.IsConnected() {
			statusCode = http.StatusServiceUnavailable
			status = "error"
			checks["nats"] = map[string]string{
				"status":  "error",
				"message": "nats disconnected",
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"clients": broker.ClientCount(),
			"checks":  checks,
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
