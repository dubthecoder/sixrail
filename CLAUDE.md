# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Rail Six — GO Transit real-time tracking. Two views: commute dashboard with countdown timer and network health, and a standalone split-flap departure board.

## Architecture

Monorepo with 5 independently deployed microservices + shared module, connected via NATS message bus (`message-bus`) and Redis cache (`cache`):

- **`services/shared/`** — Go module: models, NATS/Redis helpers, Metrolinx client, config, GTFS-RT parsers
- **`services/gtfs-static/`** — GTFS ZIP loader (24h refresh), schedule queries via HTTP (port 8081)
- **`services/realtime-poller/`** — Unified poller: 5 Metrolinx feeds every 30s → Redis + NATS (no HTTP)
- **`services/departures-api/`** — Departure queries, NextService/Fares on-demand, alerts, network health (port 8082)
- **`services/sse-push/`** — NATS → SSE streams to browsers (port 8085)
- **`services/web/`** — SvelteKit frontend, proxies all API/SSE traffic to internal services

### Proxy Architecture

No backend services are publicly exposed. All traffic flows through SvelteKit server routes:

- Browser fetches `/api/*` (same-origin) → SvelteKit `+server.ts` routes → `proxyFetch()` → departures-api
- Browser fetches `/api/sse` → SvelteKit proxies to sse-push service
- SSR `+page.server.ts` loads → `api.ts` functions → gtfs-static (stops) or departures-api (alerts, departures)

**Important:** Go backend routes have **no `/api/` prefix** (e.g., `/departures/{stopCode}`, `/alerts`). The `/api/` prefix exists only in the SvelteKit routing layer. Proxy paths in `+server.ts` files must NOT include `/api/`.

## Commands

### Services (Go workspace)
```bash
cd services
go vet ./shared/...              # vet shared module
go vet ./gtfs-static/...         # vet a specific service
go test ./... -v -short           # run all service tests (skip integration)
go test ./departures-api/... -v   # test a specific service
```

### Individual Service (dev mode)
```bash
cd services/gtfs-static && go run .      # port 8081
cd services/departures-api && go run .   # port 8082
cd services/sse-push && go run .         # port 8085
cd services/realtime-poller && go run .  # no HTTP, polls + publishes
```

### Docker (full local stack)
```bash
docker compose up                 # all services + NATS + Redis
docker compose up message-bus cache  # just infrastructure
```

### Web (SvelteKit)
```bash
cd services/web
npm run dev                       # start dev server (port 5173)
npm run check                     # svelte-kit sync + svelte-check (TypeScript)
npm run lint                      # prettier --check + eslint
npm run format                    # auto-format with prettier
npm run build                     # production build
npx vitest run                    # run all tests
npx vitest run src/lib/display    # run a single test file
```

## Metrolinx API Reference

Full API docs: https://api.openmetrolinx.com/OpenDataAPI/Help/Index/en

## Service Details

### Shared Module (`services/shared/`)
- `models/` — Stop, Route, Departure, NextServiceLine, UnionDeparture, NetworkLine, FareInfo, ServiceGlanceEntry, Alert
- `bus/` — NATS Connect, Publish, Subscribe helpers
- `cache/` — Redis helpers: SetJSON, GetJSON, SetHashJSON, GetHashFieldJSON, GetHashAllJSON, SetMembers, IsMember, SetTimestamp, GetAge
- `config/` — EnvOr, Require, env var constants and defaults
- `metrolinx/` — HTTP client and response parsers (NextService, ServiceGlance, UnionDepartures, Exceptions, Fares)
- `gtfsrt/` — GTFS-RT protobuf parsers (ParseAlerts, ParseTripUpdates, EnrichAlerts)

### GTFS Static (`services/gtfs-static/`)
HTTP API exposing schedule data: stops, routes, departures, trip info, service calendar queries. Downloads and indexes GTFS ZIP at startup (refreshes every 24h).

### Realtime Poller (`services/realtime-poller/`)
Unified poller: fetches all 5 Metrolinx feeds every 30s (exceptions every 60s), writes to Redis hashes/sets/JSON, publishes to NATS subjects.

### Departures API (`services/departures-api/`)
Most complex service — merges GTFS static schedule with real-time data from Redis. Handles NextService (on-demand, 30s cache), Fares (on-demand, 1h cache), Union departures enrichment, alerts, network health.

### SSE Push (`services/sse-push/`)
Subscribes to 5 NATS subjects, broadcasts to connected SSE clients. Event names: alerts, trip-updates, service-glance, exceptions, union-departures.

### Redis Keys
| Key | Type | TTL | Writer | Readers |
|-----|------|-----|--------|---------|
| `transit:alerts` | JSON string | 5m | realtime-poller | departures-api |
| `transit:trip-updates` | Hash (tripID → JSON) | 5m | realtime-poller | departures-api |
| `transit:service-glance` | Hash (tripNum → JSON) | 5m | realtime-poller | departures-api |
| `transit:exceptions` | Set (tripNumbers) | 5m | realtime-poller | departures-api |
| `transit:union-departures` | JSON string | 5m | realtime-poller | departures-api |
| `transit:next-service:{stopCode}` | JSON string | 30s | departures-api | departures-api |
| `transit:fares:{from}:{to}` | JSON string | 1h | departures-api | departures-api |
| `transit:*:updated-at` | String (unix ts) | 5m | realtime-poller | departures-api |

### NATS Subjects
`transit.alerts`, `transit.trip-updates`, `transit.service-glance`, `transit.exceptions`, `transit.union-departures`

### API Routes (via SvelteKit proxy)
- `GET /api/departures/{stopCode}` — departures for a station (optional `?dest=` filter)
- `GET /api/union-departures` — Union Station departures
- `GET /api/alerts` — active service alerts
- `GET /api/network-health` — active trains per GO line
- `GET /api/fares/{from}/{to}` — fare info between two stations
- `GET /api/sse` — SSE stream (proxied to sse-push)
- `GET /health` — web health check

## Web Structure

Two pages:
- `/` — commute dashboard with countdown timer, network health, fares, and alerts for saved commute routes
- `/departures` — standalone split-flap departure board. Defaults to Union Station, with station picker dropdown. Auto-scales font to viewport for TV/kiosk display.

Key files:
- `src/lib/api.ts` — server-only API functions (uses `$env/dynamic/private`). Uses `API_BASE_URL` for departures-api, `GTFS_STATIC_ADDR` for gtfs-static
- `src/lib/api-client.ts` — browser-side fetch wrappers (same-origin, proxied through SvelteKit server routes)
- `src/lib/server/proxy.ts` — `proxyFetch()` helper: forwards requests to `API_BASE_URL`, `getSseUrl()` for SSE
- `src/lib/sse.ts` — SSE client for real-time alerts and union departures
- `src/routes/+page.server.ts` — loads stops and alerts server-side
- `src/routes/+page.svelte` — renders MyCommute (commute dashboard)
- `src/routes/departures/+page.svelte` — split-flap departure board page
- `src/routes/health/+server.ts` — web health check (decoupled from API)
- `src/lib/stores/commute.ts` — localStorage-backed commute route store
- `src/lib/display.ts` — time formatting, countdown, status text/color helpers

Key components:
- `SplitFlapChar` — single CSS flip-animation character tile (keep usage count low to avoid animation lag)
- `SplitFlapBoard` — commute dashboard board using SplitFlapChar
- `MyCommute` — commute dashboard with direction toggle, countdown timer, alerts

## Key Conventions

- Go: stdlib `net/http`, `slog` for logging, no external frameworks. Go workspace (`go.work`) in `services/`
- External Go deps: `jamespfennell/gtfs` (protobuf), `redis/go-redis`, `nats-io/nats.go`
- Frontend: SvelteKit 2 with Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`). No class components
- Svelte 5: `{@const}` must be a direct child of block tags (`{#each}`, `{#if}`, etc.), not nested inside `<div>` or other elements
- Styling: Tailwind CSS 4 via `@tailwindcss/vite` plugin. All palette colors overridden with hex in `app.css` `@theme` block — Tailwind 4 outputs `oklch()` by default which TV browsers (TCL/Google TV) don't support
- Formatting: Prettier with tabs, single quotes, 100 char width. Run `npm run format` before committing
- No user auth — localStorage for favorites and default station
- Input validation on path params (regex) to prevent traversal
- SplitFlapChar causes CSS animation lag when used for many characters (80+). Use plain text for variable-length content like meta-lines
- Metrolinx ServiceGlance returns `"-"` for cars when no data — filter this on the frontend
- `api-client.ts` fetch functions throw `ApiError` on non-ok responses — callers must handle errors
- Departures board fullscreen detection uses both Fullscreen API and viewport-vs-screen comparison for TV browsers

## Environment Variables

### Services (common)
| Variable | Default | Description |
|---|---|---|
| `PORT` | varies | Server port (8081 static, 8082 departures, 8085 sse) |
| `NATS_URL` | `nats://localhost:4222` | Message bus address (NATS) |
| `REDIS_ADDR` | `localhost:6379` | Cache address (Redis) |
| `REDIS_PASSWORD` | — | Redis password |
| `METROLINX_API_KEY` | — | Metrolinx OpenData API key (required for real-time) |
| `METROLINX_BASE_URL` | `https://api.openmetrolinx.com/...` | Metrolinx API base |
| `GTFS_STATIC_URL` | Metrolinx default | URL to GTFS static ZIP |
| `GTFS_STATIC_ADDR` | `http://localhost:8081` | GTFS static service address (used by departures-api) |

### Web
| Variable | Default | Description |
|---|---|---|
| `API_BASE_URL` | `http://localhost:8082` (dev) | Departures API URL for SSR and proxy |
| `GTFS_STATIC_ADDR` | `http://localhost:8081` (dev) | GTFS static service URL for SSR (stops) |
| `SSE_PUSH_URL` | `http://localhost:8085` (dev) | SSE push service URL for proxy |
| `PUBLIC_MAPBOX_TOKEN` | — | Mapbox GL access token |

## Deploy

Railway with Railpack builder. Each service has its own `railway.toml` with `watchPatterns` scoped to its directory + `services/shared/**`, enabling independent deployments.

CI: GitHub Actions in `.github/workflows/` — `api.yml` (all Go services vet + test) and `web.yml` (test, check+lint, build), triggered by path-filtered pushes/PRs.

Local dev: `docker compose up` for full stack, or run individual services with `go run .`.
