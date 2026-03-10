# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Rail Six — GO Transit real-time tracking. Two views: commute dashboard with countdown timer and network health, and a standalone split-flap departure board.

## Architecture

Monorepo with 6 independently deployed microservices + shared module, connected via NATS message bus and Redis shared cache:

- **`services/shared/`** — Go module: models, NATS/Redis helpers, Metrolinx client, config, GTFS-RT parsers
- **`services/gtfs-static/`** — GTFS ZIP loader (24h refresh), schedule queries via HTTP (port 8081)
- **`services/realtime-poller/`** — Unified poller: 5 Metrolinx feeds every 30s → Redis + NATS (no HTTP)
- **`services/departures-api/`** — Departure queries, NextService/Fares on-demand, alerts, network health (port 8082)
- **`services/api-gateway/`** — Thin routing layer, CORS, health aggregation, proxies to all services (port 8080)
- **`services/sse-push/`** — NATS → SSE streams to browsers (port 8085)
- **`web/`** — SvelteKit frontend. SSR loads initial data from API gateway. Browser-side fetches go directly to API gateway (CORS enabled)

Data flow: Browser → API Gateway → departures-api/gtfs-static → Redis/Metrolinx

Legacy monolith in `api/` is deprecated — see `api/DEPRECATED.md`.

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
cd services/api-gateway && go run .      # port 8080
cd services/sse-push && go run .         # port 8085
cd services/realtime-poller && go run .  # no HTTP, polls + publishes
```

### Docker (full local stack)
```bash
docker compose up                 # all services + NATS + Redis
docker compose up nats redis      # just infrastructure
```

### Web (SvelteKit)
```bash
cd web
npm run dev                       # start dev server (port 5173)
npm run check                     # svelte-kit sync + svelte-check (TypeScript)
npm run lint                      # prettier --check + eslint
npm run format                    # auto-format with prettier
npm run build                     # production build
npx vitest run                    # run all tests
```

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

### API Gateway (`services/api-gateway/`)
Thin proxy — routes `/api/*` to gtfs-static and departures-api. Handles CORS. Health endpoint aggregates staleness from Redis timestamps.

### SSE Push (`services/sse-push/`)
Subscribes to 5 NATS subjects, broadcasts to connected SSE clients. Event names: alerts, trip-updates, service-glance, exceptions, union-departures.

### Redis Keys
| Key | Type | TTL | Writer | Readers |
|-----|------|-----|--------|---------|
| `transit:alerts` | JSON string | 5m | realtime-poller | api-gateway, departures-api |
| `transit:trip-updates` | Hash (tripID → JSON) | 5m | realtime-poller | departures-api |
| `transit:service-glance` | Hash (tripNum → JSON) | 5m | realtime-poller | departures-api, api-gateway |
| `transit:exceptions` | Set (tripNumbers) | 5m | realtime-poller | departures-api |
| `transit:union-departures` | JSON string | 5m | realtime-poller | departures-api, api-gateway |
| `transit:next-service:{stopCode}` | JSON string | 30s | departures-api | departures-api |
| `transit:fares:{from}:{to}` | JSON string | 1h | departures-api | departures-api |
| `transit:*:updated-at` | String (unix ts) | 5m | realtime-poller | api-gateway (health) |

### NATS Subjects
`transit.alerts`, `transit.trip-updates`, `transit.service-glance`, `transit.exceptions`, `transit.union-departures`

### API Routes (via gateway)
- `GET /api/health` — aggregated health check
- `GET /api/ready` — gtfs-static readiness
- `GET /api/stops` — all GO Transit stops
- `GET /api/departures/{stopCode}` — departures for a station (optional `?dest=` filter)
- `GET /api/union-departures` — Union Station departures
- `GET /api/alerts` — active service alerts
- `GET /api/network-health` — active trains per GO line
- `GET /api/fares/{from}/{to}` — fare info between two stations
- `GET /api/sse` — SSE stream for real-time updates

## Web Structure

Two pages:
- `/` — commute dashboard with countdown timer, network health, fares, and alerts for saved commute routes
- `/departures` — standalone split-flap departure board. Defaults to Union Station, with station picker dropdown. Auto-scales font to viewport for TV/kiosk display.

Key files:
- `src/lib/api.ts` — server-only API functions (uses `$env/dynamic/private`)
- `src/lib/api-client.ts` — browser-side fetch wrappers using `PUBLIC_API_URL` env var
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
| `PORT` | varies | Server port (8080 gateway, 8081 static, 8082 departures, 8085 sse) |
| `NATS_URL` | `nats://localhost:4222` | NATS server address |
| `REDIS_ADDR` | `localhost:6379` | Redis server address |
| `REDIS_PASSWORD` | — | Redis password |
| `METROLINX_API_KEY` | — | Metrolinx OpenData API key (required for real-time) |
| `METROLINX_BASE_URL` | `https://api.openmetrolinx.com/...` | Metrolinx API base |
| `GTFS_STATIC_URL` | Metrolinx default | URL to GTFS static ZIP |
| `GTFS_STATIC_ADDR` | `http://localhost:8081` | GTFS static service address |
| `DEPARTURES_ADDR` | `http://localhost:8082` | Departures API service address |
| `SSE_PUSH_ADDR` | `http://localhost:8085` | SSE push service address |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins (comma-separated) |

### Web
| Variable | Description |
|---|---|
| `API_BASE_URL` | API gateway URL for SSR (e.g. `http://localhost:8080`) |
| `PUBLIC_API_URL` | API gateway URL for browser-side fetches |
| `PUBLIC_MAPBOX_TOKEN` | Mapbox GL access token |

## Deploy

Railway with Railpack builder. Each service has its own `railway.toml` and watches only its directory + shared.

CI: GitHub Actions in `.github/workflows/` — one workflow per service, triggered by path-filtered pushes/PRs.

Local dev: `docker compose up` for full stack, or run individual services with `go run .`.
