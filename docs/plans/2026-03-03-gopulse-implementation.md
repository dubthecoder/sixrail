# GoPulse Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a GO Transit tracking site with a Go API proxy and SvelteKit frontend, deployed on Railway.

**Architecture:** Monorepo with two services — a Go caching proxy for the Metrolinx OpenData API (`api/`) and a SvelteKit SSR frontend (`web/`). Each deployed as separate Railway services via Railpack.

**Tech Stack:** Go 1.22+ (stdlib net/http), SvelteKit 2 (Node adapter), Tailwind CSS 4, Mapbox GL JS, Railway + Railpack.

---

## Phase 1: Go API Foundation

### Task 1: Scaffold Go module and project structure

**Files:**
- Create: `api/go.mod`
- Create: `api/cmd/server/main.go`
- Create: `api/.env.example`

**Step 1: Initialize Go module**

Run: `cd api && go mod init github.com/teclara/gopulse/api`

**Step 2: Create main.go with minimal server**

```go
// api/cmd/server/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
```

**Step 3: Create .env.example**

```
METROLINX_API_KEY=your_api_key_here
PORT=8080
ALLOWED_ORIGINS=http://localhost:5173
```

**Step 4: Run the server and test health endpoint**

Run: `cd api && go run ./cmd/server/`
In another terminal: `curl http://localhost:8080/api/health`
Expected: `{"status":"ok"}`

**Step 5: Commit**

```bash
git add api/
git commit -m "feat(api): scaffold Go module with health endpoint"
```

---

### Task 2: TTL cache

**Files:**
- Create: `api/internal/cache/cache.go`
- Create: `api/internal/cache/cache_test.go`

**Step 1: Write failing tests**

```go
// api/internal/cache/cache_test.go
package cache_test

import (
	"testing"
	"time"

	"github.com/teclara/gopulse/api/internal/cache"
)

func TestCache_SetAndGet(t *testing.T) {
	c := cache.New()
	c.Set("key1", []byte(`{"data":"hello"}`), 5*time.Second)

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if string(val) != `{"data":"hello"}` {
		t.Fatalf("expected hello, got %s", string(val))
	}
}

func TestCache_Expiry(t *testing.T) {
	c := cache.New()
	c.Set("key2", []byte(`expired`), 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key2")
	if ok {
		t.Fatal("expected key2 to be expired")
	}
}

func TestCache_GetStale(t *testing.T) {
	c := cache.New()
	c.Set("key3", []byte(`stale`), 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	val, ok := c.GetStale("key3")
	if !ok {
		t.Fatal("expected key3 to exist as stale")
	}
	if string(val) != `stale` {
		t.Fatalf("expected stale, got %s", string(val))
	}
}

func TestCache_Miss(t *testing.T) {
	c := cache.New()
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected miss for nonexistent key")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/cache/ -v`
Expected: FAIL — package doesn't exist yet

**Step 3: Implement cache**

```go
// api/internal/cache/cache.go
package cache

import (
	"sync"
	"time"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
}

func New() *Cache {
	return &Cache{entries: make(map[string]entry)}
}

func (c *Cache) Set(key string, data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{data: data, expiresAt: time.Now().Add(ttl)}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

// GetStale returns data even if expired — used for fallback when upstream is down.
func (c *Cache) GetStale(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	return e.data, true
}
```

**Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/cache/ -v`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add api/internal/cache/
git commit -m "feat(api): add in-memory TTL cache with stale fallback"
```

---

### Task 3: Config module

**Files:**
- Create: `api/internal/config/config.go`
- Create: `api/internal/config/config_test.go`

**Step 1: Write failing test**

```go
// api/internal/config/config_test.go
package config_test

import (
	"testing"

	"github.com/teclara/gopulse/api/internal/config"
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
```

**Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/config/ -v`
Expected: FAIL

**Step 3: Implement config**

```go
// api/internal/config/config.go
package config

import "os"

type Config struct {
	Port             string
	MetrolinxAPIKey  string
	MetrolinxBaseURL string
	AllowedOrigins   string
}

func Load() Config {
	return Config{
		Port:             envOr("PORT", "8080"),
		MetrolinxAPIKey:  os.Getenv("METROLINX_API_KEY"),
		MetrolinxBaseURL: envOr("METROLINX_BASE_URL", "https://api.openmetrolinx.com/OpenDataAPI/api/V1"),
		AllowedOrigins:   envOr("ALLOWED_ORIGINS", "http://localhost:5173"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 4: Run tests**

Run: `cd api && go test ./internal/config/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add api/internal/config/
git commit -m "feat(api): add config module loading from environment"
```

---

### Task 4: Metrolinx API client

**Files:**
- Create: `api/internal/metrolinx/client.go`
- Create: `api/internal/metrolinx/client_test.go`

**Step 1: Write failing tests with HTTP test server**

```go
// api/internal/metrolinx/client_test.go
package metrolinx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teclara/gopulse/api/internal/metrolinx"
)

func TestClient_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("expected key=test-key, got %s", r.URL.Query().Get("key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"trains":[{"id":"1"}]}`))
	}))
	defer server.Close()

	client := metrolinx.NewClient(server.URL, "test-key")
	data, err := client.Fetch(context.Background(), "/ServiceataGlance/Trains/All")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"trains":[{"id":"1"}]}` {
		t.Fatalf("unexpected response: %s", string(data))
	}
}

func TestClient_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := metrolinx.NewClient(server.URL, "test-key")
	_, err := client.Fetch(context.Background(), "/ServiceataGlance/Trains/All")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/metrolinx/ -v`
Expected: FAIL

**Step 3: Implement client**

```go
// api/internal/metrolinx/client.go
package metrolinx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Fetch(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s?key=%s", c.baseURL, path, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return data, nil
}
```

**Step 4: Run tests**

Run: `cd api && go test ./internal/metrolinx/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add api/internal/metrolinx/
git commit -m "feat(api): add Metrolinx API client with auth and error handling"
```

---

### Task 5: HTTP handlers — departures, trains, alerts

**Files:**
- Create: `api/internal/handlers/handlers.go`
- Create: `api/internal/handlers/handlers_test.go`

**Step 1: Write failing tests**

```go
// api/internal/handlers/handlers_test.go
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teclara/gopulse/api/internal/cache"
	"github.com/teclara/gopulse/api/internal/handlers"
)

type mockFetcher struct {
	response []byte
	err      error
}

func (m *mockFetcher) Fetch(ctx context.Context, path string) ([]byte, error) {
	return m.response, m.err
}

func TestHealthHandler(t *testing.T) {
	h := handlers.New(nil, nil)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestCachedProxy_CacheHit(t *testing.T) {
	c := cache.New()
	c.Set("/ServiceUpdate/UnionDepartures/All", []byte(`{"departures":[]}`), 30*time.Second)

	fetcher := &mockFetcher{response: []byte(`should not be called`)}
	h := handlers.New(fetcher, c)

	req := httptest.NewRequest("GET", "/api/departures/union", nil)
	w := httptest.NewRecorder()
	h.UnionDepartures(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"departures":[]}` {
		t.Fatalf("expected cached data, got %s", w.Body.String())
	}
}

func TestCachedProxy_CacheMiss(t *testing.T) {
	c := cache.New()
	fetcher := &mockFetcher{response: []byte(`{"departures":[{"trip":"123"}]}`)}
	h := handlers.New(fetcher, c)

	req := httptest.NewRequest("GET", "/api/departures/union", nil)
	w := httptest.NewRecorder()
	h.UnionDepartures(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"departures":[{"trip":"123"}]}` {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	// verify it was cached
	val, ok := c.Get("/ServiceUpdate/UnionDepartures/All")
	if !ok {
		t.Fatal("expected value to be cached")
	}
	if string(val) != `{"departures":[{"trip":"123"}]}` {
		t.Fatalf("unexpected cached value: %s", string(val))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/handlers/ -v`
Expected: FAIL

**Step 3: Implement handlers**

```go
// api/internal/handlers/handlers.go
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/teclara/gopulse/api/internal/cache"
)

type Fetcher interface {
	Fetch(ctx context.Context, path string) ([]byte, error)
}

type Handlers struct {
	fetcher Fetcher
	cache   *cache.Cache
}

func New(fetcher Fetcher, cache *cache.Cache) *Handlers {
	return &Handlers{fetcher: fetcher, cache: cache}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handlers) cachedProxy(w http.ResponseWriter, r *http.Request, metrolinxPath string, ttl time.Duration) {
	// Try cache first
	if data, ok := h.cache.Get(metrolinxPath); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	// Fetch from Metrolinx
	data, err := h.fetcher.Fetch(r.Context(), metrolinxPath)
	if err != nil {
		slog.Error("metrolinx fetch failed", "path", metrolinxPath, "error", err)
		// Try stale cache
		if stale, ok := h.cache.GetStale(metrolinxPath); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "STALE")
			w.Header().Set("X-Cache-Stale", "true")
			w.Write(stale)
			return
		}
		http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		return
	}

	h.cache.Set(metrolinxPath, data, ttl)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

func (h *Handlers) UnionDepartures(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/ServiceUpdate/UnionDepartures/All", 30*time.Second)
}

func (h *Handlers) StopDepartures(w http.ResponseWriter, r *http.Request) {
	stopCode := r.PathValue("stopCode")
	h.cachedProxy(w, r, "/Stop/NextService/"+stopCode, 30*time.Second)
}

func (h *Handlers) Trains(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/ServiceataGlance/Trains/All", 30*time.Second)
}

func (h *Handlers) TrainPositions(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/Gtfs/Feed/VehiclePosition", 15*time.Second)
}

func (h *Handlers) ServiceAlerts(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/ServiceUpdate/ServiceAlert/All", 60*time.Second)
}

func (h *Handlers) InfoAlerts(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/ServiceUpdate/InformationAlert/All", 60*time.Second)
}

func (h *Handlers) Exceptions(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/ServiceUpdate/Exceptions/All", 60*time.Second)
}

func (h *Handlers) ScheduleLines(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	h.cachedProxy(w, r, "/Schedule/Line/All/"+date, time.Hour)
}

func (h *Handlers) ScheduleJourney(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")
	from := q.Get("from")
	to := q.Get("to")
	startTime := q.Get("startTime")
	maxJourney := q.Get("maxJourney")
	if maxJourney == "" {
		maxJourney = "3"
	}
	path := "/Schedule/Journey/" + date + "/" + from + "/" + to + "/" + startTime + "/" + maxJourney
	h.cachedProxy(w, r, path, 5*time.Minute)
}

func (h *Handlers) Fares(w http.ResponseWriter, r *http.Request) {
	from := r.PathValue("from")
	to := r.PathValue("to")
	h.cachedProxy(w, r, "/Fares/"+from+"/"+to, 24*time.Hour)
}

func (h *Handlers) AllStops(w http.ResponseWriter, r *http.Request) {
	h.cachedProxy(w, r, "/Stop/All", 24*time.Hour)
}

func (h *Handlers) StopDetails(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	h.cachedProxy(w, r, "/Stop/Details/"+code, 24*time.Hour)
}
```

**Step 4: Run tests**

Run: `cd api && go test ./internal/handlers/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add api/internal/handlers/
git commit -m "feat(api): add cached proxy handlers for all Metrolinx endpoints"
```

---

### Task 6: Wire up main.go with all routes and CORS

**Files:**
- Modify: `api/cmd/server/main.go`

**Step 1: Update main.go to wire everything together**

```go
// api/cmd/server/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/teclara/gopulse/api/internal/cache"
	"github.com/teclara/gopulse/api/internal/config"
	"github.com/teclara/gopulse/api/internal/handlers"
	"github.com/teclara/gopulse/api/internal/metrolinx"
)

func main() {
	cfg := config.Load()

	client := metrolinx.NewClient(cfg.MetrolinxBaseURL, cfg.MetrolinxAPIKey)
	c := cache.New()
	h := handlers.New(client, c)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/departures/union", h.UnionDepartures)
	mux.HandleFunc("GET /api/departures/{stopCode}", h.StopDepartures)
	mux.HandleFunc("GET /api/trains", h.Trains)
	mux.HandleFunc("GET /api/trains/positions", h.TrainPositions)
	mux.HandleFunc("GET /api/alerts/service", h.ServiceAlerts)
	mux.HandleFunc("GET /api/alerts/info", h.InfoAlerts)
	mux.HandleFunc("GET /api/exceptions", h.Exceptions)
	mux.HandleFunc("GET /api/schedule/lines/{date}", h.ScheduleLines)
	mux.HandleFunc("GET /api/schedule/journey", h.ScheduleJourney)
	mux.HandleFunc("GET /api/fares/{from}/{to}", h.Fares)
	mux.HandleFunc("GET /api/stops", h.AllStops)
	mux.HandleFunc("GET /api/stops/{code}", h.StopDetails)

	handler := corsMiddleware(cfg.AllowedOrigins, mux)

	slog.Info("starting gopulse-api", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
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
```

**Step 2: Run all tests**

Run: `cd api && go test ./... -v`
Expected: All PASS

**Step 3: Manual smoke test**

Run: `cd api && go run ./cmd/server/`
Test: `curl http://localhost:8080/api/health`
Expected: `{"status":"ok"}`

**Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "feat(api): wire up all routes with CORS middleware"
```

---

### Task 7: Railway config for API service

**Files:**
- Create: `api/railway.toml`

**Step 1: Create railway.toml**

```toml
[build]
builder = "RAILPACK"
watchPatterns = ["api/**"]

[deploy]
startCommand = "server"
healthcheckPath = "/api/health"
healthcheckTimeout = 300
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 5
```

Note: Railpack auto-detects Go and builds the binary. The binary name defaults to the directory name of the main package. Since our entrypoint is at `cmd/server/`, the binary is named `server`.

**Step 2: Commit**

```bash
git add api/railway.toml
git commit -m "feat(api): add Railway config with Railpack build"
```

---

## Phase 2: SvelteKit Frontend Foundation

### Task 8: Scaffold SvelteKit project

**Files:**
- Create: `web/` (entire SvelteKit scaffold)

**Step 1: Create SvelteKit project**

Run:
```bash
cd /home/wadhah/github/gopulse
npm create svelte@latest web
```

Select: Skeleton project, TypeScript, ESLint + Prettier

**Step 2: Install dependencies**

Run:
```bash
cd web
npm install
npm install -D tailwindcss @tailwindcss/vite
npm install @sveltejs/adapter-node
```

**Step 3: Configure Node adapter**

Update `web/svelte.config.js`:
```js
import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter()
	}
};

export default config;
```

**Step 4: Configure Tailwind CSS**

Update `web/vite.config.ts`:
```ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()]
});
```

Add to `web/src/app.css`:
```css
@import 'tailwindcss';
```

Import in `web/src/routes/+layout.svelte`:
```svelte
<script>
	import '../app.css';
	let { children } = $props();
</script>

{@render children()}
```

**Step 5: Create .env.example**

```
API_BASE_URL=http://localhost:8080
PUBLIC_MAPBOX_TOKEN=your_mapbox_token_here
```

**Step 6: Verify it runs**

Run: `cd web && npm run dev`
Expected: SvelteKit dev server starts at `http://localhost:5173`

**Step 7: Commit**

```bash
git add web/
git commit -m "feat(web): scaffold SvelteKit with Tailwind CSS and Node adapter"
```

---

### Task 9: API client library for frontend

**Files:**
- Create: `web/src/lib/api.ts`

**Step 1: Create API client**

```ts
// web/src/lib/api.ts
import { env } from '$env/dynamic/private';

const baseUrl = env.API_BASE_URL || 'http://localhost:8080';

async function fetchApi<T>(path: string): Promise<T> {
	const res = await fetch(`${baseUrl}${path}`);
	if (!res.ok) {
		throw new Error(`API error: ${res.status} ${res.statusText}`);
	}
	return res.json();
}

export function getUnionDepartures() {
	return fetchApi('/api/departures/union');
}

export function getStopDepartures(stopCode: string) {
	return fetchApi(`/api/departures/${stopCode}`);
}

export function getTrains() {
	return fetchApi('/api/trains');
}

export function getTrainPositions() {
	return fetchApi('/api/trains/positions');
}

export function getServiceAlerts() {
	return fetchApi('/api/alerts/service');
}

export function getInfoAlerts() {
	return fetchApi('/api/alerts/info');
}

export function getExceptions() {
	return fetchApi('/api/exceptions');
}

export function getScheduleLines(date: string) {
	return fetchApi(`/api/schedule/lines/${date}`);
}

export function getScheduleJourney(params: {
	date: string;
	from: string;
	to: string;
	startTime: string;
	maxJourney?: string;
}) {
	const query = new URLSearchParams({
		date: params.date,
		from: params.from,
		to: params.to,
		startTime: params.startTime,
		maxJourney: params.maxJourney || '3'
	});
	return fetchApi(`/api/schedule/journey?${query}`);
}

export function getFares(from: string, to: string) {
	return fetchApi(`/api/fares/${from}/${to}`);
}

export function getAllStops() {
	return fetchApi('/api/stops');
}

export function getStopDetails(code: string) {
	return fetchApi(`/api/stops/${code}`);
}
```

**Step 2: Verify build**

Run: `cd web && npm run check`
Expected: No type errors

**Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(web): add API client library for all Go service endpoints"
```

---

### Task 10: Shared layout with navigation and alert banner

**Files:**
- Create: `web/src/lib/components/Nav.svelte`
- Create: `web/src/lib/components/AlertBanner.svelte`
- Modify: `web/src/routes/+layout.svelte`
- Create: `web/src/routes/+layout.server.ts`

**Step 1: Create Nav component**

```svelte
<!-- web/src/lib/components/Nav.svelte -->
<script lang="ts">
	import { page } from '$app/stores';
</script>

<nav class="bg-green-700 text-white">
	<div class="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
		<a href="/" class="text-xl font-bold tracking-tight">GoPulse</a>
		<div class="flex gap-4 text-sm">
			<a href="/stations" class:font-bold={$page.url.pathname === '/stations'}>Stations</a>
			<a href="/map" class:font-bold={$page.url.pathname === '/map'}>Map</a>
			<a href="/schedule" class:font-bold={$page.url.pathname === '/schedule'}>Schedule</a>
			<a href="/journey" class:font-bold={$page.url.pathname === '/journey'}>Journey</a>
			<a href="/alerts" class:font-bold={$page.url.pathname === '/alerts'}>Alerts</a>
		</div>
	</div>
</nav>
```

**Step 2: Create AlertBanner component**

```svelte
<!-- web/src/lib/components/AlertBanner.svelte -->
<script lang="ts">
	let { alerts = [] }: { alerts: any[] } = $props();
</script>

{#if alerts.length > 0}
	<div class="bg-amber-100 border-b border-amber-300 px-4 py-2 text-sm text-amber-900">
		<strong>Service Alert:</strong> {alerts[0].Message || alerts[0].message || 'Service disruption reported'}
		{#if alerts.length > 1}
			<a href="/alerts" class="underline ml-2">+{alerts.length - 1} more</a>
		{/if}
	</div>
{/if}
```

**Step 3: Create layout server load for alerts**

```ts
// web/src/routes/+layout.server.ts
import { getServiceAlerts } from '$lib/api';

export async function load() {
	try {
		const alerts = await getServiceAlerts();
		return { alerts: Array.isArray(alerts) ? alerts : [] };
	} catch {
		return { alerts: [] };
	}
}
```

**Step 4: Update layout**

```svelte
<!-- web/src/routes/+layout.svelte -->
<script>
	import '../app.css';
	import Nav from '$lib/components/Nav.svelte';
	import AlertBanner from '$lib/components/AlertBanner.svelte';

	let { data, children } = $props();
</script>

<div class="min-h-screen bg-gray-50">
	<Nav />
	<AlertBanner alerts={data.alerts} />
	<main class="max-w-6xl mx-auto px-4 py-6">
		{@render children()}
	</main>
</div>
```

**Step 5: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 6: Commit**

```bash
git add web/src/lib/components/ web/src/routes/+layout.svelte web/src/routes/+layout.server.ts
git commit -m "feat(web): add nav, alert banner, and shared layout"
```

---

### Task 11: Homepage — station picker with localStorage

**Files:**
- Create: `web/src/lib/components/StationPicker.svelte`
- Create: `web/src/lib/stores/favorites.ts`
- Create: `web/src/routes/+page.svelte`
- Create: `web/src/routes/+page.server.ts`

**Step 1: Create favorites store**

```ts
// web/src/lib/stores/favorites.ts
import { browser } from '$app/environment';
import { writable } from 'svelte/store';

function createFavorites() {
	const initial = browser ? JSON.parse(localStorage.getItem('favorites') || '[]') : [];
	const { subscribe, set, update } = writable<string[]>(initial);

	return {
		subscribe,
		toggle(stopCode: string) {
			update((faves) => {
				const next = faves.includes(stopCode)
					? faves.filter((f) => f !== stopCode)
					: [...faves, stopCode];
				if (browser) localStorage.setItem('favorites', JSON.stringify(next));
				return next;
			});
		}
	};
}

export const favorites = createFavorites();

function createDefaultStation() {
	const initial = browser ? localStorage.getItem('defaultStation') || '' : '';
	const { subscribe, set } = writable<string>(initial);

	return {
		subscribe,
		set(code: string) {
			if (browser) localStorage.setItem('defaultStation', code);
			set(code);
		}
	};
}

export const defaultStation = createDefaultStation();
```

**Step 2: Create StationPicker component**

```svelte
<!-- web/src/lib/components/StationPicker.svelte -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { defaultStation } from '$lib/stores/favorites';

	let { stops = [] }: { stops: any[] } = $props();
	let query = $state('');

	let filtered = $derived(
		query.length > 0
			? stops.filter((s: any) =>
					(s.StopName || s.Name || '').toLowerCase().includes(query.toLowerCase())
				)
			: stops
	);

	function selectStation(stopCode: string) {
		defaultStation.set(stopCode);
		goto(`/departures/${stopCode}`);
	}
</script>

<div class="w-full max-w-md mx-auto">
	<input
		type="text"
		bind:value={query}
		placeholder="Search for a station..."
		class="w-full px-4 py-3 border border-gray-300 rounded-lg text-lg focus:outline-none focus:ring-2 focus:ring-green-600"
	/>
	{#if query.length > 0 && filtered.length > 0}
		<ul class="mt-2 bg-white border border-gray-200 rounded-lg shadow-lg max-h-60 overflow-y-auto">
			{#each filtered.slice(0, 10) as stop}
				<li>
					<button
						onclick={() => selectStation(stop.StopCode || stop.Code)}
						class="w-full text-left px-4 py-2 hover:bg-green-50 cursor-pointer"
					>
						{stop.StopName || stop.Name}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
```

**Step 3: Create homepage server load**

```ts
// web/src/routes/+page.server.ts
import { getAllStops } from '$lib/api';

export async function load() {
	try {
		const stops = await getAllStops();
		return { stops: Array.isArray(stops) ? stops : [] };
	} catch {
		return { stops: [] };
	}
}
```

**Step 4: Create homepage**

```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { defaultStation } from '$lib/stores/favorites';
	import StationPicker from '$lib/components/StationPicker.svelte';

	let { data } = $props();

	$effect(() => {
		if (browser) {
			const saved = localStorage.getItem('defaultStation');
			if (saved) {
				goto(`/departures/${saved}`);
			}
		}
	});
</script>

<div class="flex flex-col items-center justify-center py-20">
	<h1 class="text-4xl font-bold text-gray-900 mb-2">GoPulse</h1>
	<p class="text-gray-600 mb-8">Real-time GO Transit tracking</p>
	<StationPicker stops={data.stops} />
</div>
```

**Step 5: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 6: Commit**

```bash
git add web/src/lib/stores/ web/src/lib/components/StationPicker.svelte web/src/routes/+page.svelte web/src/routes/+page.server.ts
git commit -m "feat(web): add homepage with station picker and localStorage"
```

---

### Task 12: Departures page

**Files:**
- Create: `web/src/lib/components/DepartureBoard.svelte`
- Create: `web/src/routes/departures/[stopCode]/+page.svelte`
- Create: `web/src/routes/departures/[stopCode]/+page.server.ts`

**Step 1: Create DepartureBoard component**

```svelte
<!-- web/src/lib/components/DepartureBoard.svelte -->
<script lang="ts">
	let { departures = [] }: { departures: any[] } = $props();
</script>

<div class="overflow-x-auto">
	<table class="w-full text-sm">
		<thead class="bg-green-700 text-white">
			<tr>
				<th class="px-4 py-2 text-left">Line</th>
				<th class="px-4 py-2 text-left">Destination</th>
				<th class="px-4 py-2 text-left">Scheduled</th>
				<th class="px-4 py-2 text-left">Status</th>
				<th class="px-4 py-2 text-left">Platform</th>
			</tr>
		</thead>
		<tbody>
			{#each departures as dep}
				<tr class="border-b border-gray-200 hover:bg-gray-50">
					<td class="px-4 py-2 font-medium">{dep.LineName || dep.Line || '—'}</td>
					<td class="px-4 py-2">{dep.Destination || dep.DirectionName || '—'}</td>
					<td class="px-4 py-2">{dep.ScheduledTime || dep.Time || '—'}</td>
					<td class="px-4 py-2">
						<span class={dep.Late || dep.Delayed ? 'text-red-600 font-medium' : 'text-green-600'}>
							{dep.Status || (dep.Late ? 'Delayed' : 'On Time')}
						</span>
					</td>
					<td class="px-4 py-2">{dep.Platform || dep.Track || '—'}</td>
				</tr>
			{:else}
				<tr>
					<td colspan="5" class="px-4 py-8 text-center text-gray-500">
						No active departures at this time.
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
```

**Step 2: Create server load**

```ts
// web/src/routes/departures/[stopCode]/+page.server.ts
import { getStopDepartures, getStopDetails } from '$lib/api';

export async function load({ params }) {
	const [departures, stopDetails] = await Promise.all([
		getStopDepartures(params.stopCode).catch(() => []),
		getStopDetails(params.stopCode).catch(() => null)
	]);

	return {
		stopCode: params.stopCode,
		departures: Array.isArray(departures) ? departures : [],
		stopDetails
	};
}
```

**Step 3: Create departures page**

```svelte
<!-- web/src/routes/departures/[stopCode]/+page.svelte -->
<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { onMount } from 'svelte';
	import DepartureBoard from '$lib/components/DepartureBoard.svelte';
	import { favorites } from '$lib/stores/favorites';

	let { data } = $props();
	let isFavorite = $derived($favorites.includes(data.stopCode));

	onMount(() => {
		const interval = setInterval(() => invalidateAll(), 30_000);
		return () => clearInterval(interval);
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">
			{data.stopDetails?.StopName || data.stopDetails?.Name || `Station ${data.stopCode}`}
		</h1>
		<button
			onclick={() => favorites.toggle(data.stopCode)}
			class="text-2xl"
			aria-label={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
		>
			{isFavorite ? '\u2605' : '\u2606'}
		</button>
	</div>
	<p class="text-sm text-gray-500">Auto-refreshes every 30 seconds</p>
	<DepartureBoard departures={data.departures} />
</div>
```

**Step 4: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/lib/components/DepartureBoard.svelte web/src/routes/departures/
git commit -m "feat(web): add departures page with auto-refresh and favorites"
```

---

### Task 13: Live train map page

**Files:**
- Create: `web/src/routes/map/+page.svelte`
- Create: `web/src/routes/map/+page.server.ts`

**Step 1: Create server load**

```ts
// web/src/routes/map/+page.server.ts
import { getTrainPositions } from '$lib/api';

export async function load() {
	try {
		const positions = await getTrainPositions();
		return { positions };
	} catch {
		return { positions: null };
	}
}
```

**Step 2: Create map page**

```svelte
<!-- web/src/routes/map/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { env } from '$env/dynamic/public';

	let { data } = $props();
	let mapContainer: HTMLDivElement;
	let map: any;

	onMount(async () => {
		const mapboxgl = (await import('mapbox-gl')).default;
		await import('mapbox-gl/dist/mapbox-gl.css');

		mapboxgl.accessToken = env.PUBLIC_MAPBOX_TOKEN || '';

		map = new mapboxgl.Map({
			container: mapContainer,
			style: 'mapbox://styles/mapbox/light-v11',
			center: [-79.38, 43.65], // Toronto
			zoom: 9
		});

		map.addControl(new mapboxgl.NavigationControl());

		if (data.positions?.entity) {
			for (const entity of data.positions.entity) {
				const vp = entity.vehicle?.position;
				if (vp?.latitude && vp?.longitude) {
					new mapboxgl.Marker({ color: '#15803d' })
						.setLngLat([vp.longitude, vp.latitude])
						.setPopup(
							new mapboxgl.Popup().setHTML(
								`<strong>Trip ${entity.vehicle?.trip?.tripId || '—'}</strong><br/>
								 Route: ${entity.vehicle?.trip?.routeId || '—'}`
							)
						)
						.addTo(map);
				}
			}
		}

		// Poll for updates
		const interval = setInterval(async () => {
			try {
				const res = await fetch('/map/__data.json?x-sveltekit-invalidated=1');
				// SvelteKit will re-run the load function
			} catch { /* ignore */ }
		}, 15_000);

		return () => {
			clearInterval(interval);
			map?.remove();
		};
	});
</script>

<svelte:head>
	<link href="https://api.mapbox.com/mapbox-gl-js/v3.4.0/mapbox-gl.css" rel="stylesheet" />
</svelte:head>

<div class="space-y-4">
	<h1 class="text-2xl font-bold">Live Train Map</h1>
	<div bind:this={mapContainer} class="w-full h-[600px] rounded-lg border border-gray-200"></div>
</div>
```

**Step 3: Install mapbox-gl**

Run: `cd web && npm install mapbox-gl`

**Step 4: Verify build**

Run: `cd web && npm run check`
Expected: No errors (may need to install `@types/mapbox-gl` — do so if needed)

**Step 5: Commit**

```bash
git add web/src/routes/map/ web/package.json web/package-lock.json
git commit -m "feat(web): add live train map with Mapbox GL"
```

---

### Task 14: Alerts page

**Files:**
- Create: `web/src/routes/alerts/+page.svelte`
- Create: `web/src/routes/alerts/+page.server.ts`

**Step 1: Create server load**

```ts
// web/src/routes/alerts/+page.server.ts
import { getServiceAlerts, getInfoAlerts, getExceptions } from '$lib/api';

export async function load() {
	const [serviceAlerts, infoAlerts, exceptions] = await Promise.all([
		getServiceAlerts().catch(() => []),
		getInfoAlerts().catch(() => []),
		getExceptions().catch(() => [])
	]);

	return {
		serviceAlerts: Array.isArray(serviceAlerts) ? serviceAlerts : [],
		infoAlerts: Array.isArray(infoAlerts) ? infoAlerts : [],
		exceptions: Array.isArray(exceptions) ? exceptions : []
	};
}
```

**Step 2: Create alerts page**

```svelte
<!-- web/src/routes/alerts/+page.svelte -->
<script lang="ts">
	let { data } = $props();
	let filter = $state('all');
</script>

<div class="space-y-4">
	<h1 class="text-2xl font-bold">Service Alerts</h1>

	<div class="flex gap-2">
		<button onclick={() => filter = 'all'} class="px-3 py-1 rounded {filter === 'all' ? 'bg-green-700 text-white' : 'bg-gray-200'}">All</button>
		<button onclick={() => filter = 'service'} class="px-3 py-1 rounded {filter === 'service' ? 'bg-red-600 text-white' : 'bg-gray-200'}">Service</button>
		<button onclick={() => filter = 'info'} class="px-3 py-1 rounded {filter === 'info' ? 'bg-blue-600 text-white' : 'bg-gray-200'}">Info</button>
		<button onclick={() => filter = 'exceptions'} class="px-3 py-1 rounded {filter === 'exceptions' ? 'bg-amber-600 text-white' : 'bg-gray-200'}">Cancellations</button>
	</div>

	{#if filter === 'all' || filter === 'service'}
		{#each data.serviceAlerts as alert}
			<div class="bg-red-50 border-l-4 border-red-500 p-4 rounded">
				<p class="font-medium text-red-900">{alert.Message || alert.Title || 'Service disruption'}</p>
				{#if alert.UpdatedTime}<p class="text-sm text-red-700 mt-1">{alert.UpdatedTime}</p>{/if}
			</div>
		{/each}
	{/if}

	{#if filter === 'all' || filter === 'info'}
		{#each data.infoAlerts as alert}
			<div class="bg-blue-50 border-l-4 border-blue-500 p-4 rounded">
				<p class="font-medium text-blue-900">{alert.Message || alert.Title || 'Information'}</p>
				{#if alert.UpdatedTime}<p class="text-sm text-blue-700 mt-1">{alert.UpdatedTime}</p>{/if}
			</div>
		{/each}
	{/if}

	{#if filter === 'all' || filter === 'exceptions'}
		{#each data.exceptions as exc}
			<div class="bg-amber-50 border-l-4 border-amber-500 p-4 rounded">
				<p class="font-medium text-amber-900">{exc.Message || exc.Title || 'Schedule exception'}</p>
				{#if exc.TripNumber}<p class="text-sm text-amber-700 mt-1">Trip: {exc.TripNumber}</p>{/if}
			</div>
		{/each}
	{/if}

	{#if data.serviceAlerts.length === 0 && data.infoAlerts.length === 0 && data.exceptions.length === 0}
		<p class="text-gray-500 py-8 text-center">No active alerts.</p>
	{/if}
</div>
```

**Step 3: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/routes/alerts/
git commit -m "feat(web): add alerts page with filter tabs"
```

---

### Task 15: Schedule and journey pages

**Files:**
- Create: `web/src/routes/schedule/+page.svelte`
- Create: `web/src/routes/schedule/+page.server.ts`
- Create: `web/src/routes/journey/+page.svelte`
- Create: `web/src/routes/journey/+page.server.ts`

**Step 1: Create schedule server load**

```ts
// web/src/routes/schedule/+page.server.ts
import { getScheduleLines } from '$lib/api';

export async function load({ url }) {
	const date = url.searchParams.get('date') || new Date().toISOString().split('T')[0];
	try {
		const lines = await getScheduleLines(date);
		return { lines: Array.isArray(lines) ? lines : [], date };
	} catch {
		return { lines: [], date };
	}
}
```

**Step 2: Create schedule page**

```svelte
<!-- web/src/routes/schedule/+page.svelte -->
<script lang="ts">
	import { goto } from '$app/navigation';

	let { data } = $props();
	let date = $state(data.date);

	function changeDate() {
		goto(`/schedule?date=${date}`);
	}
</script>

<div class="space-y-4">
	<h1 class="text-2xl font-bold">Schedule</h1>

	<div class="flex gap-2 items-center">
		<input type="date" bind:value={date} onchange={changeDate}
			class="px-3 py-2 border border-gray-300 rounded" />
	</div>

	<div class="grid gap-3">
		{#each data.lines as line}
			<div class="bg-white border border-gray-200 rounded-lg p-4">
				<p class="font-medium">{line.LineName || line.Name || 'Line'}</p>
				<p class="text-sm text-gray-500">{line.Direction || ''}</p>
			</div>
		{:else}
			<p class="text-gray-500 py-8 text-center">No lines found for this date.</p>
		{/each}
	</div>
</div>
```

**Step 3: Create journey server load**

```ts
// web/src/routes/journey/+page.server.ts
import { getAllStops, getScheduleJourney, getFares } from '$lib/api';

export async function load({ url }) {
	const stops = await getAllStops().catch(() => []);
	const from = url.searchParams.get('from');
	const to = url.searchParams.get('to');
	const date = url.searchParams.get('date');
	const startTime = url.searchParams.get('startTime');

	if (!from || !to || !date || !startTime) {
		return { stops: Array.isArray(stops) ? stops : [], journeys: null, fares: null };
	}

	const [journeys, fares] = await Promise.all([
		getScheduleJourney({ date, from, to, startTime }).catch(() => null),
		getFares(from, to).catch(() => null)
	]);

	return {
		stops: Array.isArray(stops) ? stops : [],
		journeys,
		fares
	};
}
```

**Step 4: Create journey page**

```svelte
<!-- web/src/routes/journey/+page.svelte -->
<script lang="ts">
	import { goto } from '$app/navigation';

	let { data } = $props();
	let from = $state('');
	let to = $state('');
	let date = $state(new Date().toISOString().split('T')[0]);
	let startTime = $state('08:00');

	function search() {
		if (!from || !to) return;
		goto(`/journey?from=${from}&to=${to}&date=${date}&startTime=${startTime}`);
	}
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Journey Planner</h1>

	<div class="bg-white border border-gray-200 rounded-lg p-4 space-y-3">
		<div class="grid grid-cols-2 gap-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">From</label>
				<select bind:value={from} class="w-full px-3 py-2 border border-gray-300 rounded">
					<option value="">Select station</option>
					{#each data.stops as stop}
						<option value={stop.StopCode || stop.Code}>{stop.StopName || stop.Name}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">To</label>
				<select bind:value={to} class="w-full px-3 py-2 border border-gray-300 rounded">
					<option value="">Select station</option>
					{#each data.stops as stop}
						<option value={stop.StopCode || stop.Code}>{stop.StopName || stop.Name}</option>
					{/each}
				</select>
			</div>
		</div>
		<div class="grid grid-cols-2 gap-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Date</label>
				<input type="date" bind:value={date} class="w-full px-3 py-2 border border-gray-300 rounded" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Depart after</label>
				<input type="time" bind:value={startTime} class="w-full px-3 py-2 border border-gray-300 rounded" />
			</div>
		</div>
		<button onclick={search}
			class="w-full bg-green-700 text-white py-2 rounded font-medium hover:bg-green-800">
			Search
		</button>
	</div>

	{#if data.fares}
		<div class="bg-green-50 border border-green-200 rounded-lg p-4">
			<p class="font-medium">Fare Information</p>
			<pre class="text-sm mt-1">{JSON.stringify(data.fares, null, 2)}</pre>
		</div>
	{/if}

	{#if data.journeys}
		<div class="space-y-3">
			<h2 class="text-lg font-medium">Results</h2>
			<pre class="bg-white border rounded-lg p-4 text-sm overflow-x-auto">{JSON.stringify(data.journeys, null, 2)}</pre>
		</div>
	{/if}
</div>
```

**Step 5: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 6: Commit**

```bash
git add web/src/routes/schedule/ web/src/routes/journey/
git commit -m "feat(web): add schedule browser and journey planner pages"
```

---

### Task 16: Stations page

**Files:**
- Create: `web/src/routes/stations/+page.svelte`
- Create: `web/src/routes/stations/+page.server.ts`

**Step 1: Create server load**

```ts
// web/src/routes/stations/+page.server.ts
import { getAllStops } from '$lib/api';

export async function load() {
	try {
		const stops = await getAllStops();
		return { stops: Array.isArray(stops) ? stops : [] };
	} catch {
		return { stops: [] };
	}
}
```

**Step 2: Create stations page**

```svelte
<!-- web/src/routes/stations/+page.svelte -->
<script lang="ts">
	import { favorites } from '$lib/stores/favorites';

	let { data } = $props();
	let query = $state('');

	let filtered = $derived(
		query.length > 0
			? data.stops.filter((s: any) =>
					(s.StopName || s.Name || '').toLowerCase().includes(query.toLowerCase())
				)
			: data.stops
	);

	let favoriteStops = $derived(
		data.stops.filter((s: any) => $favorites.includes(s.StopCode || s.Code))
	);
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Stations</h1>

	<input type="text" bind:value={query} placeholder="Search stations..."
		class="w-full px-4 py-2 border border-gray-300 rounded-lg" />

	{#if favoriteStops.length > 0 && query.length === 0}
		<div>
			<h2 class="text-lg font-medium mb-2">Favorites</h2>
			<div class="grid gap-2">
				{#each favoriteStops as stop}
					<a href="/departures/{stop.StopCode || stop.Code}"
						class="block bg-green-50 border border-green-200 rounded-lg p-3 hover:bg-green-100">
						{stop.StopName || stop.Name}
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<div>
		<h2 class="text-lg font-medium mb-2">All Stations</h2>
		<div class="grid gap-2">
			{#each filtered as stop}
				<a href="/departures/{stop.StopCode || stop.Code}"
					class="block bg-white border border-gray-200 rounded-lg p-3 hover:bg-gray-50">
					{stop.StopName || stop.Name}
				</a>
			{:else}
				<p class="text-gray-500">No stations found.</p>
			{/each}
		</div>
	</div>
</div>
```

**Step 3: Verify build**

Run: `cd web && npm run check`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/routes/stations/
git commit -m "feat(web): add stations page with search and favorites"
```

---

## Phase 3: Deploy & CI

### Task 17: Railway config for web service

**Files:**
- Create: `web/railway.toml`

**Step 1: Create railway.toml**

```toml
[build]
builder = "RAILPACK"
watchPatterns = ["web/**"]

[deploy]
startCommand = "node build/index.js"
healthcheckPath = "/"
healthcheckTimeout = 300
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 5
```

**Step 2: Commit**

```bash
git add web/railway.toml
git commit -m "feat(web): add Railway config with Railpack build"
```

---

### Task 18: GitHub Actions CI workflows

**Files:**
- Create: `.github/workflows/api.yml`
- Create: `.github/workflows/web.yml`

**Step 1: Create API workflow**

```yaml
# .github/workflows/api.yml
name: API CI

on:
  push:
    paths: ['api/**']
  pull_request:
    paths: ['api/**']

jobs:
  test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: api
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -v
      - run: go vet ./...
```

**Step 2: Create web workflow**

```yaml
# .github/workflows/web.yml
name: Web CI

on:
  push:
    paths: ['web/**']
  pull_request:
    paths: ['web/**']

jobs:
  check:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm ci
      - run: npm run check
      - run: npm run lint
      - run: npm run build
```

**Step 3: Commit**

```bash
git add .github/
git commit -m "ci: add GitHub Actions workflows for API and web"
```

---

### Task 19: CLAUDE.md project config

**Files:**
- Create: `CLAUDE.md`

**Step 1: Create CLAUDE.md**

```markdown
# GoPulse

GO Transit tracking site — real-time departures, live map, alerts, schedules, and fares.

## Architecture

Monorepo with two services:
- `api/` — Go caching proxy for Metrolinx OpenData API
- `web/` — SvelteKit frontend with Tailwind CSS and Mapbox

## Development

### API (Go)
```bash
cd api
cp .env.example .env  # fill in METROLINX_API_KEY
go run ./cmd/server/
```

### Web (SvelteKit)
```bash
cd web
cp .env.example .env  # fill in PUBLIC_MAPBOX_TOKEN, API_BASE_URL
npm install
npm run dev
```

## Testing

- API: `cd api && go test ./... -v`
- Web: `cd web && npm run check && npm run lint`

## Deploy

Railway with Railpack. Each service has its own `railway.toml`.
- API root directory: `api/`
- Web root directory: `web/`

## Key Conventions

- Go: stdlib `net/http`, `slog` for logging, no external frameworks
- Frontend: SvelteKit 2 with Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`)
- Styling: Tailwind CSS
- No user auth — localStorage for personalization
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md project config"
```

---

## Summary

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| 1: Go API | Tasks 1-7 | Fully working API proxy with caching, all 13 endpoints, CORS, Railway config |
| 2: SvelteKit | Tasks 8-16 | Complete frontend: homepage, departures, map, alerts, schedule, journey, stations |
| 3: Deploy & CI | Tasks 17-19 | Railway configs, GitHub Actions, project documentation |

Total: **19 tasks**, each independently committable, TDD where applicable.
