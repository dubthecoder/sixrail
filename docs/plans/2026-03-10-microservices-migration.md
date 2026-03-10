# Microservices Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Decompose the monolithic Go API into independently deployable microservices connected via NATS message bus and Redis shared cache, enabling independent iteration and deployment without redeploying a monolith.

**Architecture:** A unified poller service fetches all 5 Metrolinx real-time feeds in parallel every 30s, writes to Redis, and publishes to NATS. A separate GTFS static service handles the 24h schedule data lifecycle. A departures API merges static + realtime data and handles on-demand NextService/Fares calls. An API gateway routes external requests. An SSE push service streams real-time updates to browsers. The SvelteKit frontend drops its proxy layer.

**Tech Stack:** Go 1.25, NATS (message bus), Redis (shared state), SvelteKit 2/Svelte 5 (frontend), Railway (deploy)

## Data Source Audit

All 8 Metrolinx data sources are required — none can be dropped:

| Source | Frequency | Unique Data | Why Required |
|--------|-----------|-------------|--------------|
| GTFS Static ZIP | 24h | Stops, routes, trips, stop_times, calendar | Schedule backbone |
| GTFS-RT Alerts | 30s poll | Service advisories, route associations | Alert banner, per-departure alerts |
| GTFS-RT Trip Updates | 30s poll | Per-stop delay (seconds), schedule relationship | Delay minutes, "Delayed +Xm" status |
| ServiceGlance | 30s poll | Cars count, lat/lon, isInMotion per trip | Cars display, motion flag, network health |
| Exceptions | 60s poll | Cancelled trip numbers | **29 cancellations invisible to GTFS-RT** (tested 2026-03-09) |
| Union Departures | 30s poll | Platform, PROCEED/WAIT, stops for Union | Union board page, PROCEED/WAIT status |
| NextService | On-demand (30s TTL) | Real-time actual platform per stop | **Trains land on different platforms at different stations** |
| Fares | On-demand (1h TTL) | Fare categories and amounts | Fare display |

---

## Service Inventory

| # | Service | Dir | Port | Description |
|---|---------|-----|------|-------------|
| 0 | `shared` | `services/shared/` | — | Go module: models, NATS/Redis helpers, Metrolinx client, config |
| 1 | `gtfs-static` | `services/gtfs-static/` | 8081 | GTFS ZIP loader, schedule queries via HTTP |
| 2 | `realtime-poller` | `services/realtime-poller/` | — | Unified poller: all 5 Metrolinx feeds → Redis + NATS |
| 3 | `departures-api` | `services/departures-api/` | 8082 | Departure queries, NextService on-demand, fares on-demand |
| 4 | `api-gateway` | `services/api-gateway/` | 8080 | Routes requests, CORS, serves cached data, health aggregation |
| 5 | `sse-push` | `services/sse-push/` | 8085 | NATS → SSE streams to browsers |
| 6 | `web` | `web/` | 5173 | SvelteKit frontend (no proxy layer) |

**6 services** (+ shared module) instead of the original 13. Down from the current 2.

### Why each service is separate

| Service | Independent deploy reason |
|---------|--------------------------|
| gtfs-static | 24h lifecycle, memory-heavy (ZIP parsing), rarely changes |
| realtime-poller | Poll logic changes independently, no HTTP surface, restart doesn't affect queries |
| departures-api | Most complex logic (merging), most frequent iteration target |
| api-gateway | CORS/routing changes, rate limiting, no business logic |
| sse-push | Scale SSE connections independently from request/response traffic |
| web | UI changes deploy without touching any backend |

## NATS Subjects

| Subject | Publisher | Payload | Frequency |
|---------|-----------|---------|-----------|
| `transit.alerts` | realtime-poller | `[]Alert` JSON | 30s |
| `transit.trip-updates` | realtime-poller | `map[string]RawTripUpdate` JSON | 30s |
| `transit.service-glance` | realtime-poller | `map[string]ServiceGlanceEntry` JSON | 30s |
| `transit.exceptions` | realtime-poller | `[]string` (cancelled trip numbers) | 60s |
| `transit.union-departures` | realtime-poller | `[]UnionDeparture` JSON | 30s |

## Redis Keys

| Key | Type | TTL | Writer | Readers |
|-----|------|-----|--------|---------|
| `transit:alerts` | JSON string | 5m | realtime-poller | api-gateway, departures-api |
| `transit:trip-updates` | Hash (tripID → JSON) | 5m | realtime-poller | departures-api |
| `transit:service-glance` | Hash (tripNum → JSON) | 5m | realtime-poller | departures-api, api-gateway (network-health) |
| `transit:exceptions` | Set (tripNumbers) | 5m | realtime-poller | departures-api |
| `transit:union-departures` | JSON string | 5m | realtime-poller | departures-api, api-gateway |
| `transit:next-service:{stopCode}` | JSON string | 30s | departures-api | departures-api |
| `transit:fares:{from}:{to}` | JSON string | 1h | departures-api | departures-api |
| `transit:*:updated-at` | String (unix ts) | 5m | realtime-poller | api-gateway (health) |

## Data Flow

```
Metrolinx APIs (5 feeds)
        │
        ▼
┌──────────────────┐     publish     ┌──────┐     stream     ┌─────────┐
│ realtime-poller  │ ──────────────▶ │ NATS │ ─────────────▶ │sse-push │──▶ Browser (SSE)
│ (unified, 30s)   │                 └──────┘                └─────────┘
│                  │     write
│                  │ ──────────────▶ ┌───────┐
└──────────────────┘                 │ Redis │
                                     └───┬───┘
                                         │ read
        ┌────────────────────────────────┤
        ▼                                ▼
┌──────────────┐  HTTP query   ┌────────────────┐
│ gtfs-static  │ ◀──────────── │ departures-api │
│ (schedule)   │               │ (merge+enrich) │
└──────────────┘               └───────┬────────┘
                                       │
                               ┌───────▼────────┐
                               │  api-gateway   │ ◀──── Browser (REST)
                               │ (routes, CORS) │
                               └────────────────┘
                                       ▲
                               ┌───────┴────────┐
                               │   web (SSR)    │
                               └────────────────┘
```

---

## Phase 1: Foundation — Shared Module + Infrastructure

### Task 1.1: Create Go workspace and shared module

**Files:**
- Create: `services/go.work`
- Create: `services/shared/go.mod`
- Create: `services/shared/models/models.go`
- Create: `services/shared/models/models_test.go`

**Step 1: Create directory structure**

```bash
mkdir -p services/shared/models
```

**Step 2: Initialize Go workspace**

Create `services/go.work`:
```go
go 1.25.8

use (
    ./shared
)
```

**Step 3: Initialize shared module**

```bash
cd services/shared && go mod init github.com/teclara/railsix/shared
```

**Step 4: Copy models from monolith**

Copy all structs from `api/internal/models/models.go` → `services/shared/models/models.go`. Change package path to `github.com/teclara/railsix/shared/models`. Keep all JSON tags identical.

**Step 5: Write test to verify model serialization round-trips**

Create `services/shared/models/models_test.go`:
```go
package models

import (
    "encoding/json"
    "testing"
)

func TestDepartureRoundTrip(t *testing.T) {
    d := Departure{
        Line: "LW", LineName: "Lakeshore West",
        Destination: "Hamilton", ScheduledTime: "14:30",
        Status: "On Time", Platform: "3",
    }
    b, err := json.Marshal(d)
    if err != nil {
        t.Fatal(err)
    }
    var got Departure
    if err := json.Unmarshal(b, &got); err != nil {
        t.Fatal(err)
    }
    if got.Line != d.Line || got.ScheduledTime != d.ScheduledTime {
        t.Errorf("round-trip mismatch: got %+v", got)
    }
}

func TestAlertRoundTrip(t *testing.T) {
    a := Alert{
        ID: "alert-1", Headline: "Delay on LW",
        RouteNames: []string{"Lakeshore West"},
    }
    b, err := json.Marshal(a)
    if err != nil {
        t.Fatal(err)
    }
    var got Alert
    if err := json.Unmarshal(b, &got); err != nil {
        t.Fatal(err)
    }
    if got.ID != a.ID || len(got.RouteNames) != 1 {
        t.Errorf("round-trip mismatch: got %+v", got)
    }
}
```

**Step 6: Run tests**

```bash
cd services/shared && go test ./... -v
```
Expected: PASS

**Step 7: Commit**

```bash
git add services/go.work services/shared/
git commit -m "feat: create shared Go module with models for microservices"
```

---

### Task 1.2: Add NATS helper to shared module

**Files:**
- Create: `services/shared/bus/bus.go`
- Create: `services/shared/bus/bus_test.go`

**Step 1: Add NATS dependency**

```bash
cd services/shared && go get github.com/nats-io/nats.go@latest
```

**Step 2: Write failing test**

Create `services/shared/bus/bus_test.go`:
```go
package bus

import (
    "context"
    "encoding/json"
    "testing"
    "time"
)

func TestPubSub(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    conn, err := Connect("nats://localhost:4222")
    if err != nil {
        t.Fatalf("connect: %v", err)
    }
    defer conn.Close()

    type msg struct {
        Value string `json:"value"`
    }

    received := make(chan msg, 1)
    err = Subscribe(conn, "test.subject", func(data []byte) {
        var m msg
        json.Unmarshal(data, &m)
        received <- m
    })
    if err != nil {
        t.Fatal(err)
    }

    if err := Publish(conn, "test.subject", msg{Value: "hello"}); err != nil {
        t.Fatal(err)
    }

    select {
    case got := <-received:
        if got.Value != "hello" {
            t.Errorf("expected hello, got %s", got.Value)
        }
    case <-ctx.Done():
        t.Fatal("timeout waiting for message")
    }
}
```

**Step 3: Implement NATS helpers**

Create `services/shared/bus/bus.go`:
```go
package bus

import (
    "encoding/json"
    "fmt"
    "log/slog"
    "time"

    "github.com/nats-io/nats.go"
)

func Connect(url string) (*nats.Conn, error) {
    nc, err := nats.Connect(url,
        nats.RetryOnFailedConnect(true),
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2*time.Second),
        nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
            slog.Warn("NATS disconnected", "error", err)
        }),
        nats.ReconnectHandler(func(_ *nats.Conn) {
            slog.Info("NATS reconnected")
        }),
    )
    if err != nil {
        return nil, fmt.Errorf("nats connect: %w", err)
    }
    return nc, nil
}

func Publish(nc *nats.Conn, subject string, v any) error {
    data, err := json.Marshal(v)
    if err != nil {
        return fmt.Errorf("marshal: %w", err)
    }
    return nc.Publish(subject, data)
}

func Subscribe(nc *nats.Conn, subject string, handler func(data []byte)) error {
    _, err := nc.Subscribe(subject, func(msg *nats.Msg) {
        handler(msg.Data)
    })
    return err
}
```

**Step 4: Run tests (skip integration)**

```bash
cd services/shared && go test ./... -v -short
```
Expected: PASS

**Step 5: Commit**

```bash
git add services/shared/
git commit -m "feat: add NATS pub/sub helpers to shared module"
```

---

### Task 1.3: Add Redis helper to shared module

**Files:**
- Create: `services/shared/cache/cache.go`
- Create: `services/shared/cache/cache_test.go`

**Step 1: Add Redis dependency**

```bash
cd services/shared && go get github.com/redis/go-redis/v9@latest
```

**Step 2: Write failing test**

Create `services/shared/cache/cache_test.go`:
```go
package cache

import (
    "context"
    "testing"
    "time"
)

func TestSetGetJSON(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    ctx := context.Background()
    c, err := Connect("localhost:6379", "")
    if err != nil {
        t.Fatal(err)
    }
    defer c.Close()

    type item struct{ Name string `json:"name"` }
    if err := SetJSON(ctx, c, "test:item", item{Name: "foo"}, 10*time.Second); err != nil {
        t.Fatal(err)
    }
    var got item
    if err := GetJSON(ctx, c, "test:item", &got); err != nil {
        t.Fatal(err)
    }
    if got.Name != "foo" {
        t.Errorf("expected foo, got %s", got.Name)
    }
}

func TestSetGetHash(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    ctx := context.Background()
    c, err := Connect("localhost:6379", "")
    if err != nil {
        t.Fatal(err)
    }
    defer c.Close()

    type entry struct{ Value int `json:"value"` }
    items := map[string]entry{"a": {Value: 1}, "b": {Value: 2}}
    if err := SetHashJSON(ctx, c, "test:hash", items, 10*time.Second); err != nil {
        t.Fatal(err)
    }
    var got entry
    if err := GetHashFieldJSON(ctx, c, "test:hash", "a", &got); err != nil {
        t.Fatal(err)
    }
    if got.Value != 1 {
        t.Errorf("expected 1, got %d", got.Value)
    }
}
```

**Step 3: Implement Redis helpers**

Create `services/shared/cache/cache.go`:
```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

func Connect(addr, password string) (*redis.Client, error) {
    c := redis.NewClient(&redis.Options{Addr: addr, Password: password})
    if err := c.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("redis ping: %w", err)
    }
    return c, nil
}

func SetJSON(ctx context.Context, c *redis.Client, key string, v any, ttl time.Duration) error {
    data, err := json.Marshal(v)
    if err != nil {
        return fmt.Errorf("marshal: %w", err)
    }
    return c.Set(ctx, key, data, ttl).Err()
}

func GetJSON(ctx context.Context, c *redis.Client, key string, dest any) error {
    data, err := c.Get(ctx, key).Bytes()
    if err != nil {
        return fmt.Errorf("get %s: %w", key, err)
    }
    return json.Unmarshal(data, dest)
}

func SetHashJSON[V any](ctx context.Context, c *redis.Client, key string, items map[string]V, ttl time.Duration) error {
    pipe := c.Pipeline()
    for field, v := range items {
        data, err := json.Marshal(v)
        if err != nil {
            return fmt.Errorf("marshal field %s: %w", field, err)
        }
        pipe.HSet(ctx, key, field, data)
    }
    pipe.Expire(ctx, key, ttl)
    _, err := pipe.Exec(ctx)
    return err
}

func GetHashFieldJSON(ctx context.Context, c *redis.Client, key, field string, dest any) error {
    data, err := c.HGet(ctx, key, field).Bytes()
    if err != nil {
        return fmt.Errorf("hget %s.%s: %w", key, field, err)
    }
    return json.Unmarshal(data, dest)
}

func GetHashAllJSON[V any](ctx context.Context, c *redis.Client, key string) (map[string]V, error) {
    raw, err := c.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("hgetall %s: %w", key, err)
    }
    result := make(map[string]V, len(raw))
    for field, data := range raw {
        var v V
        if err := json.Unmarshal([]byte(data), &v); err != nil {
            return nil, fmt.Errorf("unmarshal field %s: %w", field, err)
        }
        result[field] = v
    }
    return result, nil
}

func SetMembers(ctx context.Context, c *redis.Client, key string, members []string, ttl time.Duration) error {
    pipe := c.Pipeline()
    pipe.Del(ctx, key)
    if len(members) > 0 {
        args := make([]any, len(members))
        for i, m := range members { args[i] = m }
        pipe.SAdd(ctx, key, args...)
    }
    pipe.Expire(ctx, key, ttl)
    _, err := pipe.Exec(ctx)
    return err
}

func IsMember(ctx context.Context, c *redis.Client, key, member string) (bool, error) {
    return c.SIsMember(ctx, key, member).Result()
}

func SetTimestamp(ctx context.Context, c *redis.Client, key string, ttl time.Duration) error {
    return c.Set(ctx, key, time.Now().Unix(), ttl).Err()
}

func GetAge(ctx context.Context, c *redis.Client, key string) (time.Duration, error) {
    ts, err := c.Get(ctx, key).Int64()
    if err != nil {
        return 0, err
    }
    return time.Since(time.Unix(ts, 0)), nil
}
```

**Step 4: Run tests**

```bash
cd services/shared && go test ./... -v -short
```
Expected: PASS

**Step 5: Commit**

```bash
git add services/shared/
git commit -m "feat: add Redis cache helpers to shared module"
```

---

### Task 1.4: Add shared config and Metrolinx client

**Files:**
- Create: `services/shared/config/config.go`
- Create: `services/shared/config/config_test.go`
- Create: `services/shared/metrolinx/client.go`
- Create: `services/shared/metrolinx/responses.go`
- Create: `services/shared/metrolinx/client_test.go`

**Step 1: Create config helper**

Create `services/shared/config/config.go`:
```go
package config

import (
    "fmt"
    "os"
)

func EnvOr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func Require(key string) (string, error) {
    v := os.Getenv(key)
    if v == "" {
        return "", fmt.Errorf("required env var %s is not set", key)
    }
    return v, nil
}

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
)

const (
    DefaultNATSURL        = "nats://localhost:4222"
    DefaultRedisAddr      = "localhost:6379"
    DefaultMetrolinxBase  = "https://api.openmetrolinx.com/OpenDataAPI/api/V1"
    DefaultGTFSStaticURL  = "https://assets.metrolinx.com/raw/upload/Documents/Metrolinx/Open%20Data/GO-GTFS.zip"
    DefaultGTFSStaticAddr = "http://localhost:8081"
    DefaultDeparturesAddr = "http://localhost:8082"
)
```

**Step 2: Write config test**

Create `services/shared/config/config_test.go`:
```go
package config

import (
    "os"
    "testing"
)

func TestEnvOr(t *testing.T) {
    if got := EnvOr("UNLIKELY_VAR_12345", "fallback"); got != "fallback" {
        t.Errorf("expected fallback, got %s", got)
    }
    os.Setenv("UNLIKELY_VAR_12345", "custom")
    defer os.Unsetenv("UNLIKELY_VAR_12345")
    if got := EnvOr("UNLIKELY_VAR_12345", "fallback"); got != "custom" {
        t.Errorf("expected custom, got %s", got)
    }
}

func TestRequire(t *testing.T) {
    os.Setenv("TEST_REQUIRED", "value")
    defer os.Unsetenv("TEST_REQUIRED")
    val, err := Require("TEST_REQUIRED")
    if err != nil || val != "value" {
        t.Errorf("expected value, got %s err %v", val, err)
    }
    _, err = Require("MISSING_REQUIRED")
    if err == nil {
        t.Error("expected error for missing required var")
    }
}
```

**Step 3: Copy Metrolinx client**

Copy `api/internal/metrolinx/client.go` → `services/shared/metrolinx/client.go`.
Copy `api/internal/metrolinx/responses.go` → `services/shared/metrolinx/responses.go`.
Update imports to `github.com/teclara/railsix/shared/models`.

**Step 4: Write client test**

Create `services/shared/metrolinx/client_test.go`:
```go
package metrolinx

import "testing"

func TestNewClient(t *testing.T) {
    c := NewClient("https://example.com", "key")
    if c == nil {
        t.Fatal("expected non-nil client")
    }
}

func TestParseMetrolinxTime(t *testing.T) {
    tests := []struct{ input, want string }{
        {"2026-03-10 14:30:00", "14:30"},
        {"invalid", "--:--"},
        {"", "--:--"},
    }
    for _, tt := range tests {
        if got := parseMetrolinxTime(tt.input); got != tt.want {
            t.Errorf("parseMetrolinxTime(%q) = %q, want %q", tt.input, got, tt.want)
        }
    }
}
```

**Step 5: Run all tests**

```bash
cd services/shared && go test ./... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add services/shared/
git commit -m "feat: add config helpers and metrolinx client to shared module"
```

---

### Task 1.5: Add GTFS-RT parsers to shared module

**Files:**
- Create: `services/shared/gtfsrt/types.go`
- Create: `services/shared/gtfsrt/parser.go`
- Create: `services/shared/gtfsrt/parser_test.go`

**Step 1: Extract types**

Create `services/shared/gtfsrt/types.go` — copy `RawAlert`, `RawTripUpdate`, `RawStopTimeUpdate` and the JSON struct types from `api/internal/gtfs/realtime.go`.

**Step 2: Extract parsers**

Create `services/shared/gtfsrt/parser.go` — copy `ParseAlerts`, `ParseTripUpdates`, `EnrichAlerts`, `englishText` from `api/internal/gtfs/realtime.go`.

**Step 3: Write test**

Create `services/shared/gtfsrt/parser_test.go`:
```go
package gtfsrt

import "testing"

func TestParseAlertsEmpty(t *testing.T) {
    alerts, err := ParseAlerts([]byte(`{"entity":[]}`))
    if err != nil {
        t.Fatal(err)
    }
    if len(alerts) != 0 {
        t.Errorf("expected 0 alerts, got %d", len(alerts))
    }
}

func TestParseTripUpdatesEmpty(t *testing.T) {
    updates, err := ParseTripUpdates([]byte(`{"entity":[]}`))
    if err != nil {
        t.Fatal(err)
    }
    if len(updates) != 0 {
        t.Errorf("expected 0 updates, got %d", len(updates))
    }
}
```

**Step 4: Run tests, commit**

```bash
cd services/shared && go test ./... -v
git add services/shared/gtfsrt/
git commit -m "feat: extract GTFS-RT parsers to shared module"
```

---

### Task 1.6: Docker Compose for local dev

**Files:**
- Create: `docker-compose.yml`

**Step 1: Create compose file**

```yaml
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
      - "8222:8222"
    command: ["--js"]

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: ["redis-server", "--save", "60", "1"]
```

**Step 2: Verify**

```bash
docker compose up -d && docker compose ps
```
Expected: Both running.

**Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "infra: add docker-compose for NATS and Redis"
```

---

## Phase 2: GTFS Static Service

### Task 2.1: Create GTFS static service

This service owns the GTFS ZIP lifecycle and exposes schedule data via HTTP. It replaces direct `StaticStore` access.

**Files:**
- Create: `services/gtfs-static/go.mod`
- Create: `services/gtfs-static/store/store.go` (copy from `api/internal/gtfs/static.go`)
- Create: `services/gtfs-static/store/download.go` (copy GTFS download logic from `api/cmd/server/main.go`)
- Create: `services/gtfs-static/main.go`
- Create: `services/gtfs-static/main_test.go`
- Modify: `services/go.work` (add `./gtfs-static`)

**Step 1: Initialize module, add to workspace**

```bash
mkdir -p services/gtfs-static/store
cd services/gtfs-static && go mod init github.com/teclara/railsix/gtfs-static
```

Add `./gtfs-static` to `services/go.work` use block.

**Step 2: Copy static store**

Copy `api/internal/gtfs/static.go` → `services/gtfs-static/store/store.go`. Update imports to use shared models.

Extract `manageGTFS()` and `downloadURL()` from `api/cmd/server/main.go` → `services/gtfs-static/store/download.go`.

**Step 3: Create HTTP server**

Create `services/gtfs-static/main.go` with these endpoints:

| Endpoint | Maps to StaticStore method |
|----------|---------------------------|
| `GET /ready` | `store.Ready()` |
| `GET /stops` | `store.AllStops()` |
| `GET /stops/{code}/ids` | `store.StopIDsForCode(code)` |
| `GET /departures/{stopID}` | `store.DeparturesForStop(stopID)` |
| `GET /trips/{tripID}` | `store.GetTrip(tripID)` |
| `GET /routes/{routeID}` | `store.GetRoute(routeID)` |
| `GET /trips/{tripID}/remaining-stops?stopID=...` | `store.RemainingStopNames(tripID, stopIDs)` |
| `GET /trips/{tripID}/is-last-stop?stopID=...` | `store.IsLastStop(tripID, stopIDs)` |
| `GET /trips/{tripID}/is-express` | `store.IsExpress(tripID)` |
| `GET /services/{serviceID}/active?date=YYYY-MM-DD` | `store.IsServiceActive(serviceID, date)` |
| `GET /trips/{tripID}/arrival?destID=...&originID=...` | `store.ArrivalTimeAtStop(tripID, destIDs, originIDs...)` |
| `GET /stop-name/{stopID}` | `store.GetStopName(stopID)` |

All responses are JSON. Each handler is thin — validate input, call store method, return JSON.

**Step 4: Write test**

Create `services/gtfs-static/main_test.go`:
```go
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestReadyBeforeLoad(t *testing.T) {
    req := httptest.NewRequest("GET", "/ready", nil)
    w := httptest.NewRecorder()
    mux := http.NewServeMux()
    ready := false
    mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
        if !ready {
            http.Error(w, "loading", http.StatusServiceUnavailable)
            return
        }
        w.Write([]byte(`{"status":"ok"}`))
    })
    mux.ServeHTTP(w, req)
    if w.Code != http.StatusServiceUnavailable {
        t.Errorf("expected 503, got %d", w.Code)
    }
}
```

**Step 5: Run tests, commit**

```bash
cd services/gtfs-static && go test ./... -v
git add services/gtfs-static/ services/go.work
git commit -m "feat: create gtfs-static microservice"
```

---

## Phase 3: Unified Realtime Poller

### Task 3.1: Create realtime-poller service

One service, one goroutine, 4 parallel fetches every 30s (exceptions every 60s). Writes to Redis and publishes to NATS.

**Files:**
- Create: `services/realtime-poller/go.mod`
- Create: `services/realtime-poller/main.go`
- Create: `services/realtime-poller/poller.go`
- Create: `services/realtime-poller/poller_test.go`
- Modify: `services/go.work`

**Step 1: Initialize module**

```bash
mkdir -p services/realtime-poller
cd services/realtime-poller && go mod init github.com/teclara/railsix/realtime-poller
```

**Step 2: Write test**

Create `services/realtime-poller/poller_test.go`:
```go
package main

import (
    "context"
    "testing"

    "github.com/teclara/railsix/shared/gtfsrt"
    "github.com/teclara/railsix/shared/models"
)

type mockFetcher struct{ data []byte }

func (m *mockFetcher) Fetch(ctx context.Context, path string) ([]byte, error) {
    return m.data, nil
}

type mockRouteLookup struct{}

func (m *mockRouteLookup) GetRoute(id string) (models.Route, bool) {
    return models.Route{ShortName: "LW", LongName: "Lakeshore West"}, true
}

func TestFetchAndParseAlerts(t *testing.T) {
    json := `{"entity":[{"id":"1","alert":{
        "header_text":{"translation":[{"language":"en","text":"Test"}]},
        "description_text":{"translation":[{"language":"en","text":"Details"}]},
        "informed_entity":[{"route_id":"r1"}]
    }}]}`
    fetcher := &mockFetcher{data: []byte(json)}
    raw, err := fetchAlerts(context.Background(), fetcher)
    if err != nil {
        t.Fatal(err)
    }
    if len(raw) != 1 || raw[0].Headline != "Test" {
        t.Errorf("unexpected: %+v", raw)
    }
    enriched := gtfsrt.EnrichAlerts(raw, &mockRouteLookup{})
    if enriched[0].RouteNames[0] != "Lakeshore West" {
        t.Errorf("expected Lakeshore West, got %v", enriched[0].RouteNames)
    }
}
```

**Step 3: Implement poller**

Create `services/realtime-poller/poller.go`:
```go
package main

import (
    "context"
    "log/slog"
    "sync"

    "github.com/teclara/railsix/shared/gtfsrt"
    "github.com/teclara/railsix/shared/metrolinx"
)

type Fetcher interface {
    Fetch(ctx context.Context, path string) ([]byte, error)
}

func fetchAlerts(ctx context.Context, f Fetcher) ([]gtfsrt.RawAlert, error) {
    data, err := f.Fetch(ctx, "/Gtfs/Feed/Alerts")
    if err != nil {
        return nil, err
    }
    return gtfsrt.ParseAlerts(data)
}

func fetchTripUpdates(ctx context.Context, f Fetcher) (map[string]gtfsrt.RawTripUpdate, error) {
    data, err := f.Fetch(ctx, "/Gtfs/Feed/TripUpdates")
    if err != nil {
        return nil, err
    }
    return gtfsrt.ParseTripUpdates(data)
}

// pollAll fetches all 5 feeds in parallel. Exceptions only fetched when includeExceptions is true.
func pollAll(ctx context.Context, mx *metrolinx.Client, lookup RouteLookup, includeExceptions bool) PollResult {
    var result PollResult
    var wg sync.WaitGroup

    wg.Add(4)

    go func() {
        defer wg.Done()
        raw, err := fetchAlerts(ctx, mx)
        if err != nil {
            slog.Error("alerts fetch failed", "error", err)
            return
        }
        result.mu.Lock()
        result.alerts = gtfsrt.EnrichAlerts(raw, lookup)
        result.hasAlerts = true
        result.mu.Unlock()
    }()

    go func() {
        defer wg.Done()
        updates, err := fetchTripUpdates(ctx, mx)
        if err != nil {
            slog.Error("trip updates fetch failed", "error", err)
            return
        }
        result.mu.Lock()
        result.tripUpdates = updates
        result.hasTripUpdates = true
        result.mu.Unlock()
    }()

    go func() {
        defer wg.Done()
        entries, err := mx.GetServiceGlance(ctx)
        if err != nil {
            slog.Error("service glance fetch failed", "error", err)
            return
        }
        result.mu.Lock()
        result.serviceGlance = entries
        result.hasServiceGlance = true
        result.mu.Unlock()
    }()

    go func() {
        defer wg.Done()
        deps, err := mx.GetUnionDepartures(ctx)
        if err != nil {
            slog.Error("union departures fetch failed", "error", err)
            return
        }
        result.mu.Lock()
        result.unionDepartures = deps
        result.hasUnionDepartures = true
        result.mu.Unlock()
    }()

    if includeExceptions {
        wg.Add(1)
        go func() {
            defer wg.Done()
            cancelled, err := mx.GetExceptions(ctx)
            if err != nil {
                slog.Error("exceptions fetch failed", "error", err)
                return
            }
            result.mu.Lock()
            result.exceptions = cancelled
            result.hasExceptions = true
            result.mu.Unlock()
        }()
    }

    wg.Wait()
    return result
}
```

Where `PollResult` holds the fetched data:
```go
type PollResult struct {
    mu                  sync.Mutex
    alerts              []models.Alert
    hasAlerts           bool
    tripUpdates         map[string]gtfsrt.RawTripUpdate
    hasTripUpdates      bool
    serviceGlance       []models.ServiceGlanceEntry
    hasServiceGlance    bool
    exceptions          map[string]bool
    hasExceptions       bool
    unionDepartures     []models.UnionDeparture
    hasUnionDepartures  bool
}
```

**Step 4: Implement main loop**

Create `services/realtime-poller/main.go`:
```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/teclara/railsix/shared/bus"
    "github.com/teclara/railsix/shared/cache"
    "github.com/teclara/railsix/shared/config"
    "github.com/teclara/railsix/shared/metrolinx"
    "github.com/teclara/railsix/shared/models"
)

const (
    pollInterval = 30 * time.Second
    cacheTTL     = 5 * time.Minute
)

// RouteLookup calls gtfs-static for route names (for alert enrichment).
type RouteLookup interface {
    GetRoute(id string) (models.Route, bool)
}

func main() {
    apiKey, err := config.Require(config.EnvMetrolinxAPIKey)
    if err != nil {
        slog.Error("missing API key", "error", err)
        os.Exit(1)
    }

    mx := metrolinx.NewClient(
        config.EnvOr(config.EnvMetrolinxBase, config.DefaultMetrolinxBase),
        apiKey,
    )
    nc, err := bus.Connect(config.EnvOr(config.EnvNATSURL, config.DefaultNATSURL))
    if err != nil {
        slog.Error("NATS connect failed", "error", err)
        os.Exit(1)
    }
    defer nc.Close()

    rc, err := cache.Connect(
        config.EnvOr(config.EnvRedisAddr, config.DefaultRedisAddr),
        config.EnvOr(config.EnvRedisPassword, ""),
    )
    if err != nil {
        slog.Error("Redis connect failed", "error", err)
        os.Exit(1)
    }
    defer rc.Close()

    lookup := newHTTPRouteLookup(config.EnvOr(config.EnvGTFSStaticAddr, config.DefaultGTFSStaticAddr))

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    slog.Info("realtime-poller starting", "interval", pollInterval)

    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()
    tickCount := 0

    poll := func() {
        tickCount++
        // Exceptions every 60s (every other tick)
        includeExceptions := tickCount%2 == 0

        result := pollAll(ctx, mx, lookup, includeExceptions)

        // Write to Redis + publish to NATS for each successful fetch
        if result.hasAlerts {
            cache.SetJSON(ctx, rc, "transit:alerts", result.alerts, cacheTTL)
            cache.SetTimestamp(ctx, rc, "transit:alerts:updated-at", cacheTTL)
            bus.Publish(nc, "transit.alerts", result.alerts)
            slog.Info("alerts published", "count", len(result.alerts))
        }
        if result.hasTripUpdates {
            cache.SetHashJSON(ctx, rc, "transit:trip-updates", result.tripUpdates, cacheTTL)
            cache.SetTimestamp(ctx, rc, "transit:trip-updates:updated-at", cacheTTL)
            bus.Publish(nc, "transit.trip-updates", result.tripUpdates)
            slog.Info("trip updates published", "count", len(result.tripUpdates))
        }
        if result.hasServiceGlance {
            m := make(map[string]models.ServiceGlanceEntry, len(result.serviceGlance))
            for _, e := range result.serviceGlance {
                m[e.TripNumber] = e
            }
            cache.SetHashJSON(ctx, rc, "transit:service-glance", m, cacheTTL)
            cache.SetTimestamp(ctx, rc, "transit:service-glance:updated-at", cacheTTL)
            bus.Publish(nc, "transit.service-glance", m)
            slog.Info("service glance published", "count", len(m))
        }
        if result.hasExceptions {
            members := make([]string, 0, len(result.exceptions))
            for k := range result.exceptions {
                members = append(members, k)
            }
            cache.SetMembers(ctx, rc, "transit:exceptions", members, cacheTTL)
            cache.SetTimestamp(ctx, rc, "transit:exceptions:updated-at", cacheTTL)
            bus.Publish(nc, "transit.exceptions", members)
            slog.Info("exceptions published", "count", len(members))
        }
        if result.hasUnionDepartures {
            cache.SetJSON(ctx, rc, "transit:union-departures", result.unionDepartures, cacheTTL)
            cache.SetTimestamp(ctx, rc, "transit:union-departures:updated-at", cacheTTL)
            bus.Publish(nc, "transit.union-departures", result.unionDepartures)
            slog.Info("union departures published", "count", len(result.unionDepartures))
        }
    }

    poll() // immediate first poll
    go func() {
        for {
            select {
            case <-ticker.C:
                poll()
            case <-ctx.Done():
                return
            }
        }
    }()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    <-sigCh
    cancel()
}
```

Include `httpRouteLookup` (calls gtfs-static `/routes/{id}`):
```go
type httpRouteLookup struct {
    baseURL string
    client  *http.Client
}

func newHTTPRouteLookup(baseURL string) *httpRouteLookup {
    return &httpRouteLookup{baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (l *httpRouteLookup) GetRoute(id string) (models.Route, bool) {
    resp, err := l.client.Get(l.baseURL + "/routes/" + url.PathEscape(id))
    if err != nil || resp.StatusCode != 200 {
        return models.Route{}, false
    }
    defer resp.Body.Close()
    var route models.Route
    if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
        return models.Route{}, false
    }
    return route, true
}
```

**Step 5: Run tests, commit**

```bash
cd services/realtime-poller && go test ./... -v
git add services/realtime-poller/ services/go.work
git commit -m "feat: create unified realtime-poller microservice"
```

---

## Phase 4: Departures API

### Task 4.1: Create departures-api service

The most complex service. Ports `GetDepartures()` + `StopDepartures()` handler + NextService enrichment + Fares. Reads from Redis (realtime) + gtfs-static HTTP (schedule).

**Files:**
- Create: `services/departures-api/go.mod`
- Create: `services/departures-api/staticclient.go`
- Create: `services/departures-api/redisclient.go`
- Create: `services/departures-api/departures.go`
- Create: `services/departures-api/departures_test.go`
- Create: `services/departures-api/main.go`
- Modify: `services/go.work`

**Step 1: Initialize module**

```bash
mkdir -p services/departures-api
cd services/departures-api && go mod init github.com/teclara/railsix/departures-api
```

**Step 2: Create gtfs-static HTTP client**

Create `services/departures-api/staticclient.go` — thin wrapper calling the gtfs-static service. One method per StaticStore method used by `GetDepartures()`:

```go
// Methods needed:
// StopIDsForCode(code) → []string
// DeparturesForStop(stopID) → []ScheduledDeparture
// IsLastStop(tripID, stopIDs) → bool
// IsServiceActive(serviceID, date) → bool
// GetRoute(routeID) → (Route, bool)
// GetStopName(stopID) → string
// RemainingStopNames(tripID, stopIDs) → []string
// IsExpress(tripID) → bool
// ArrivalTimeAtStop(tripID, destIDs, originIDs...) → (Duration, bool)
// GetTrip(tripID) → (TripInfo, bool)
```

Each method does `GET` to gtfs-static and deserializes JSON.

**Step 3: Create Redis realtime client**

Create `services/departures-api/redisclient.go` — wraps Redis reads for the departure merging logic:

```go
// Methods needed (replacing RealtimeCache getters):
// GetTripUpdate(tripID) → (RawTripUpdate, bool)
// GetServiceGlanceEntry(tripNum) → (ServiceGlanceEntry, bool)
// IsTripCancelled(tripNum) → bool
// GetUnionDepartureByTrip(tripNum) → (UnionDeparture, bool)
// GetUnionDepartures() → []UnionDeparture
// GetAlerts() → []Alert
// GetNextService(stopCode) → ([]NextServiceLine, bool)
// SetNextService(stopCode, lines)
// GetFares(from, to) → ([]FareInfo, bool)
// SetFares(from, to, fares)
```

NextService and Fares use Redis TTL keys instead of in-memory TTL maps.

**Step 4: Port departure merging logic**

Create `services/departures-api/departures.go` — port `GetDepartures()` from `api/internal/gtfs/departures.go`. Replace:
- `static.Xxx()` → `staticClient.Xxx()`
- `rt.Xxx()` → `redisClient.Xxx()`
- Logic stays identical

**Step 5: Create HTTP server**

Create `services/departures-api/main.go` with endpoints:

| Endpoint | Logic |
|----------|-------|
| `GET /departures/{stopCode}?dest=` | Validates code, calls `GetDepartures()`, enriches with NextService + Union + alerts (same as current `StopDepartures` handler) |
| `GET /union-departures` | Reads from Redis, enriches with ServiceGlance + exceptions + alerts (same as current `UnionDepartures` handler) |
| `GET /fares/{from}/{to}` | Check Redis cache → miss calls `mx.GetFares()` → store in Redis with 1h TTL |
| `GET /network-health` | Read all ServiceGlance from Redis hash, group by line code, return counts |
| `GET /alerts` | Read alerts from Redis, return slim response |

Note: Network health and alerts handlers are simple enough to live here rather than as separate services. The api-gateway will proxy to this service for these endpoints.

**Step 6: Write test**

Create `services/departures-api/departures_test.go` — test with mock static client and mock Redis client. Port existing departure tests from `api/internal/gtfs/` if any.

**Step 7: Run tests, commit**

```bash
cd services/departures-api && go test ./... -v
git add services/departures-api/ services/go.work
git commit -m "feat: create departures-api microservice"
```

---

## Phase 5: API Gateway

### Task 5.1: Create API gateway

Thin routing layer. No business logic. Routes to departures-api and gtfs-static. Handles CORS.

**Files:**
- Create: `services/api-gateway/go.mod`
- Create: `services/api-gateway/main.go`
- Create: `services/api-gateway/main_test.go`
- Modify: `services/go.work`

**Step 1: Initialize module**

```bash
mkdir -p services/api-gateway
cd services/api-gateway && go mod init github.com/teclara/railsix/api-gateway
```

**Step 2: Implement**

Route table:

| External Route | Proxied To |
|---|---|
| `GET /api/health` | Self — aggregates health from all services via Redis timestamps |
| `GET /api/ready` | gtfs-static `/ready` |
| `GET /api/stops` | gtfs-static `/stops` |
| `GET /api/departures/{stopCode}` | departures-api `/departures/{stopCode}` |
| `GET /api/union-departures` | departures-api `/union-departures` |
| `GET /api/alerts` | departures-api `/alerts` |
| `GET /api/network-health` | departures-api `/network-health` |
| `GET /api/fares/{from}/{to}` | departures-api `/fares/{from}/{to}` |
| `GET /api/sse` | sse-push `/sse` (proxy or redirect) |

Each proxy is simple: build URL, forward request, copy response. No body parsing.

CORS middleware: same logic as current `api/cmd/server/main.go`.

Health endpoint: reads `transit:*:updated-at` keys from Redis, reports staleness.

**Step 3: Write test**

Test CORS middleware and health endpoint.

**Step 4: Run tests, commit**

```bash
cd services/api-gateway && go test ./... -v
git add services/api-gateway/ services/go.work
git commit -m "feat: create api-gateway microservice"
```

---

## Phase 6: SSE Push Service

### Task 6.1: Create SSE push service

Subscribes to all NATS subjects, broadcasts to connected SSE clients.

**Files:**
- Create: `services/sse-push/go.mod`
- Create: `services/sse-push/main.go`
- Create: `services/sse-push/broker.go`
- Create: `services/sse-push/broker_test.go`
- Modify: `services/go.work`

**Step 1: Initialize module**

```bash
mkdir -p services/sse-push
cd services/sse-push && go mod init github.com/teclara/railsix/sse-push
```

**Step 2: Implement broker**

Create `services/sse-push/broker.go`:
```go
package main

import "sync"

type SSEEvent struct {
    Name string
    Data []byte
}

type Broker struct {
    mu      sync.RWMutex
    clients map[chan SSEEvent]struct{}
}

func NewBroker() *Broker {
    return &Broker{clients: make(map[chan SSEEvent]struct{})}
}

func (b *Broker) Subscribe() chan SSEEvent {
    ch := make(chan SSEEvent, 64)
    b.mu.Lock()
    b.clients[ch] = struct{}{}
    b.mu.Unlock()
    return ch
}

func (b *Broker) Unsubscribe(ch chan SSEEvent) {
    b.mu.Lock()
    delete(b.clients, ch)
    b.mu.Unlock()
    close(ch)
}

func (b *Broker) Broadcast(event SSEEvent) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for ch := range b.clients {
        select {
        case ch <- event:
        default: // slow client, drop
        }
    }
}
```

**Step 3: Write broker test**

Create `services/sse-push/broker_test.go`:
```go
package main

import "testing"

func TestBrokerSubscribeBroadcast(t *testing.T) {
    b := NewBroker()
    ch := b.Subscribe()
    defer b.Unsubscribe(ch)

    b.Broadcast(SSEEvent{Name: "test", Data: []byte(`{"ok":true}`)})

    select {
    case event := <-ch:
        if event.Name != "test" {
            t.Errorf("expected test, got %s", event.Name)
        }
    default:
        t.Error("expected message")
    }
}

func TestBrokerUnsubscribe(t *testing.T) {
    b := NewBroker()
    ch := b.Subscribe()
    b.Unsubscribe(ch)

    // Should not panic
    b.Broadcast(SSEEvent{Name: "test", Data: []byte(`{}`)})
}
```

**Step 4: Implement main**

Create `services/sse-push/main.go`:
- Subscribe to 5 NATS subjects: `transit.alerts`, `transit.trip-updates`, `transit.service-glance`, `transit.exceptions`, `transit.union-departures`
- Map subjects to SSE event names: `alerts`, `trip-updates`, `service-glance`, `exceptions`, `union-departures`
- `GET /sse` handler: set SSE headers, subscribe to broker, write events in `event: name\ndata: json\n\n` format, unsubscribe on disconnect
- CORS headers for SSE endpoint

**Step 5: Run tests, commit**

```bash
cd services/sse-push && go test ./... -v
git add services/sse-push/ services/go.work
git commit -m "feat: create SSE push microservice"
```

---

## Phase 7: Update Web Frontend

### Task 7.1: Remove SvelteKit proxy routes

**Files:**
- Delete: `web/src/routes/api/stops/+server.ts`
- Delete: `web/src/routes/api/departures/[stopCode]/+server.ts`
- Delete: `web/src/routes/api/union-departures/+server.ts`
- Delete: `web/src/routes/api/alerts/+server.ts`
- Delete: `web/src/routes/api/network-health/+server.ts`
- Delete: `web/src/routes/api/fares/[from]/[to]/+server.ts`

**Step 1: Delete all proxy routes**

```bash
rm -rf web/src/routes/api/
```

**Step 2: Update api-client.ts**

Update `web/src/lib/api-client.ts` — change fetch URLs from relative `/api/*` to the API gateway base URL. Use an env var `PUBLIC_API_URL` for the gateway address.

Browser-side fetches go directly to the gateway (which handles CORS).

**Step 3: Update api.ts (server-side)**

Update `web/src/lib/api.ts` — SSR calls go to the gateway's internal Railway address.

**Step 4: Verify**

```bash
cd web && npm run check && npm run lint
```

**Step 5: Commit**

```bash
git add -A web/
git commit -m "refactor: remove SvelteKit proxy routes, point to API gateway"
```

---

### Task 7.2: Add SSE client for real-time updates

**Files:**
- Create: `web/src/lib/sse.ts`
- Modify: `web/src/routes/departures/+page.svelte`
- Modify: `web/src/routes/+page.svelte`

**Step 1: Create SSE client**

Create `web/src/lib/sse.ts`:
```typescript
type SSEHandler = (data: any) => void;

const handlers = new Map<string, SSEHandler[]>();
let eventSource: EventSource | null = null;

export function connectSSE(url: string) {
    if (eventSource) return;
    eventSource = new EventSource(url);

    for (const event of ['alerts', 'union-departures']) {
        eventSource.addEventListener(event, (e: MessageEvent) => {
            const data = JSON.parse(e.data);
            for (const handler of handlers.get(event) || []) {
                handler(data);
            }
        });
    }

    eventSource.onerror = () => {
        console.warn('SSE connection lost, auto-reconnecting...');
    };
}

export function onSSE(event: string, handler: SSEHandler): () => void {
    if (!handlers.has(event)) handlers.set(event, []);
    handlers.get(event)!.push(handler);
    return () => {
        const list = handlers.get(event);
        if (list) {
            const idx = list.indexOf(handler);
            if (idx >= 0) list.splice(idx, 1);
        }
    };
}

export function disconnectSSE() {
    eventSource?.close();
    eventSource = null;
    handlers.clear();
}
```

**Step 2: Use SSE for broadcast data**

In components, replace `setInterval` polling for alerts with SSE:
```typescript
onSSE('alerts', (data) => { alerts = data; });
```

Keep request/response polling for per-station departures (not broadcast).

**Step 3: Verify, commit**

```bash
cd web && npm run check && npm run lint
git add web/src/lib/sse.ts web/src/routes/ web/src/lib/api-client.ts
git commit -m "feat: add SSE client for real-time alerts and union departures"
```

---

## Phase 8: Infrastructure & Deployment

### Task 8.1: Railway configs for each service

**Files:**
- Create: `services/gtfs-static/railway.toml`
- Create: `services/realtime-poller/railway.toml`
- Create: `services/departures-api/railway.toml`
- Create: `services/api-gateway/railway.toml`
- Create: `services/sse-push/railway.toml`

HTTP services template:
```toml
[build]
builder = "RAILPACK"
watchPatterns = ["services/<name>/**", "services/shared/**"]
checkSuites = "required"

[deploy]
startCommand = "./out"
healthcheckPath = "/<path>"
healthcheckTimeout = 300
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 5
```

Poller (no HTTP):
```toml
[build]
builder = "RAILPACK"
watchPatterns = ["services/realtime-poller/**", "services/shared/**"]
checkSuites = "required"

[deploy]
startCommand = "./out"
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 5
```

**Step 1: Create all files, commit**

```bash
git add services/*/railway.toml
git commit -m "infra: add Railway configs for all microservices"
```

---

### Task 8.2: CI workflows

**Files:**
- Create: `.github/workflows/shared.yml`
- Create: `.github/workflows/gtfs-static.yml`
- Create: `.github/workflows/realtime-poller.yml`
- Create: `.github/workflows/departures-api.yml`
- Create: `.github/workflows/api-gateway.yml`
- Create: `.github/workflows/sse-push.yml`

Each workflow: `go vet ./...` + `go test ./... -v -race -short` on path-filtered pushes/PRs. Triggered by changes in `services/<name>/**` or `services/shared/**`.

**Step 1: Create all workflows, commit**

```bash
git add .github/workflows/
git commit -m "ci: add GitHub Actions for all microservices"
```

---

### Task 8.3: Docker Compose for full local stack

**Files:**
- Modify: `docker-compose.yml`

Add all 5 backend services with Dockerfiles, connecting to NATS + Redis. Wire env vars for inter-service communication using Docker network hostnames.

```yaml
services:
  nats:
    image: nats:2-alpine
    ports: ["4222:4222", "8222:8222"]
    command: ["--js"]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  gtfs-static:
    build: { context: services, dockerfile: gtfs-static/Dockerfile }
    ports: ["8081:8081"]
    environment:
      PORT: "8081"

  realtime-poller:
    build: { context: services, dockerfile: realtime-poller/Dockerfile }
    depends_on: [nats, redis, gtfs-static]
    environment:
      NATS_URL: nats://nats:4222
      REDIS_ADDR: redis:6379
      METROLINX_API_KEY: ${METROLINX_API_KEY}
      GTFS_STATIC_ADDR: http://gtfs-static:8081

  departures-api:
    build: { context: services, dockerfile: departures-api/Dockerfile }
    ports: ["8082:8082"]
    depends_on: [redis, gtfs-static]
    environment:
      PORT: "8082"
      REDIS_ADDR: redis:6379
      GTFS_STATIC_ADDR: http://gtfs-static:8081
      METROLINX_API_KEY: ${METROLINX_API_KEY}

  api-gateway:
    build: { context: services, dockerfile: api-gateway/Dockerfile }
    ports: ["8080:8080"]
    depends_on: [departures-api, redis]
    environment:
      PORT: "8080"
      REDIS_ADDR: redis:6379
      GTFS_STATIC_ADDR: http://gtfs-static:8081
      DEPARTURES_ADDR: http://departures-api:8082
      SSE_ADDR: http://sse-push:8085
      ALLOWED_ORIGINS: http://localhost:5173

  sse-push:
    build: { context: services, dockerfile: sse-push/Dockerfile }
    ports: ["8085:8085"]
    depends_on: [nats]
    environment:
      PORT: "8085"
      NATS_URL: nats://nats:4222
      ALLOWED_ORIGINS: http://localhost:5173
```

Dockerfile template per service:
```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY shared/ shared/
COPY <name>/ <name>/
COPY go.work .
RUN cd <name> && go build -o /out .

FROM alpine:3.19
COPY --from=builder /out /out
CMD ["/out"]
```

**Step 1: Create compose + Dockerfiles, commit**

```bash
git add docker-compose.yml services/*/Dockerfile
git commit -m "infra: full docker-compose for local dev"
```

---

## Phase 9: Cleanup

### Task 9.1: Archive monolith

Add `api/DEPRECATED.md` noting the monolith is replaced by `services/`. Keep running in parallel during transition.

### Task 9.2: Update CLAUDE.md

Update architecture section, commands, and env vars for the new microservices layout.

---

## Migration Checklist

- [ ] Phase 1: Shared module (models, NATS, Redis, config, Metrolinx client, GTFS-RT parsers)
- [ ] Phase 2: GTFS Static service
- [ ] Phase 3: Unified realtime poller
- [ ] Phase 4: Departures API (most complex — merging, NextService, Fares, alerts, network health)
- [ ] Phase 5: API Gateway
- [ ] Phase 6: SSE Push service
- [ ] Phase 7: Update web frontend (remove proxies, add SSE)
- [ ] Phase 8: Railway configs, CI workflows, Docker Compose
- [ ] Phase 9: Archive monolith, update docs

## Risk Mitigation

1. **Run monolith and microservices in parallel** during transition
2. **Departures API is the highest-risk service** — complex merging logic with many HTTP calls to gtfs-static. Port tests first, verify parity. Consider batch endpoints on gtfs-static to reduce round-trips.
3. **gtfs-static latency** — `GetDepartures()` calls multiple store methods per request. Mitigate with: (a) batch endpoint returning all data for a stop code in one call, (b) HTTP keep-alive connection pooling
4. **NATS + Redis are new dependencies** — use Railway's managed Redis; NATS is a single binary, low-ops
5. **Exceptions cannot be dropped** — 29 cancellations proved invisible to GTFS-RT (tested 2026-03-09)
6. **NextService must stay** — trains use different platforms at different stations
