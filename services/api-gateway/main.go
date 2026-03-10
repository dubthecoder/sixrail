package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/teclara/railsix/shared/cache"
	"github.com/teclara/railsix/shared/config"
)

func main() {
	port := config.EnvOr(config.EnvPort, "8080")
	allowedOrigins := config.EnvOr(config.EnvAllowedOrigins, "http://localhost:5173")
	redisAddr := config.EnvOr(config.EnvRedisAddr, config.DefaultRedisAddr)
	redisPassword := config.EnvOr(config.EnvRedisPassword, "")
	gtfsStaticAddr := config.EnvOr(config.EnvGTFSStaticAddr, config.DefaultGTFSStaticAddr)
	departuresAddr := config.EnvOr(config.EnvDeparturesAddr, config.DefaultDeparturesAddr)
	ssePushAddr := config.EnvOr(config.EnvSSEPushAddr, config.DefaultSSEPushAddr)

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	defer rdb.Close()

	proxy := &http.Client{Timeout: 30 * time.Second}

	mux := http.NewServeMux()

	// Health: aggregates staleness from Redis timestamps.
	mux.HandleFunc("GET /api/health", healthHandler(rdb))

	// Ready: proxy to gtfs-static.
	mux.HandleFunc("GET /api/ready", proxyHandler(proxy, gtfsStaticAddr, "/ready"))

	// Stops: proxy to gtfs-static.
	mux.HandleFunc("GET /api/stops", proxyHandler(proxy, gtfsStaticAddr, "/stops"))

	// Departures: proxy to departures-api.
	mux.HandleFunc("GET /api/departures/{stopCode}", func(w http.ResponseWriter, r *http.Request) {
		stopCode := r.PathValue("stopCode")
		target := departuresAddr + "/departures/" + stopCode
		if q := r.URL.RawQuery; q != "" {
			target += "?" + q
		}
		proxyRequest(proxy, w, r, target)
	})

	// Union departures: proxy to departures-api.
	mux.HandleFunc("GET /api/union-departures", proxyHandler(proxy, departuresAddr, "/union-departures"))

	// Alerts: proxy to departures-api.
	mux.HandleFunc("GET /api/alerts", proxyHandler(proxy, departuresAddr, "/alerts"))

	// Network health: proxy to departures-api.
	mux.HandleFunc("GET /api/network-health", proxyHandler(proxy, departuresAddr, "/network-health"))

	// Fares: proxy to departures-api.
	mux.HandleFunc("GET /api/fares/{from}/{to}", func(w http.ResponseWriter, r *http.Request) {
		from := r.PathValue("from")
		to := r.PathValue("to")
		proxyRequest(proxy, w, r, departuresAddr+"/fares/"+from+"/"+to)
	})

	// SSE: proxy to sse-push.
	mux.HandleFunc("GET /api/sse", proxyHandler(proxy, ssePushAddr, "/sse"))

	handler := corsMiddleware(allowedOrigins, mux)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("api-gateway listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down api-gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// proxyHandler returns a handler that proxies requests to upstream+path.
func proxyHandler(client *http.Client, upstream, path string) http.HandlerFunc {
	target := strings.TrimRight(upstream, "/") + path
	return func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(client, w, r, target)
	}
}

// proxyRequest forwards a request to the given target URL and copies the response back.
func proxyRequest(client *http.Client, w http.ResponseWriter, r *http.Request, target string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("proxy request failed", "target", target, "error", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers.
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// healthHandler reads transit:*:updated-at keys from Redis and reports staleness.
func healthHandler(rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		feeds := []string{"alerts", "trip-updates", "service-glance", "exceptions", "union-departures"}
		allOK := true
		parts := make([]string, 0, len(feeds))
		for _, feed := range feeds {
			key := "transit:" + feed + ":updated-at"
			age, err := cache.GetAge(r.Context(), rdb, key)
			if err != nil {
				parts = append(parts, fmt.Sprintf(`"%s":"unknown"`, feed))
				allOK = false
			} else {
				parts = append(parts, fmt.Sprintf(`"%s":"%.0fs"`, feed, age.Seconds()))
				if age > 5*time.Minute {
					allOK = false
				}
			}
		}

		status := "ok"
		if !allOK {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"%s","feeds":{%s}}`, status, strings.Join(parts, ","))
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
