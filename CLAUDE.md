# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Six Rail — GO Transit real-time tracking. Two views: commute dashboard with countdown timer and network health, and a standalone split-flap departure board.

## Architecture

Monorepo with two independently deployed services:

- **`api/`** — Go API server. Single gateway to Metrolinx OpenData API. Downloads GTFS static data at startup (refreshes every 24h), polls multiple real-time feeds in background goroutines.
- **`web/`** — SvelteKit frontend. Server-side `+page.server.ts` loads initial data from Go API. Client-side polling via SvelteKit API proxy routes (`/api/*` → Go API).

Data flow: Browser → SvelteKit SSR/proxy routes → Go API → Metrolinx OpenData API

## Commands

### API (Go)
```bash
cd api
go run ./cmd/server/              # start dev server (port 8080)
go test ./... -v                  # run all tests
go test ./internal/gtfs/ -v       # run gtfs package tests
go vet ./...                      # static analysis
```

### Web (SvelteKit)
```bash
cd web
npm run dev                       # start dev server (port 5173)
npm run check                     # svelte-kit sync + svelte-check (TypeScript)
npm run lint                      # prettier --check + eslint
npm run format                    # auto-format with prettier
npm run build                     # production build
```

## Go API Structure

Entry point: `api/cmd/server/main.go` — sets up GTFS stores, pollers, routes, middleware (CORS).

Internal packages under `api/internal/`:
- `config/` — env var loading (`METROLINX_API_KEY`, `GTFS_STATIC_URL`, `PORT`, `ALLOWED_ORIGINS`)
- `models/` — shared data structs (`Stop`, `Route`, `VehiclePosition`, `Alert`, `Departure`, `ServiceGlanceEntry`, `FareInfo`, `NetworkLine`)
- `metrolinx/` — HTTP client for Metrolinx API (10s timeout, 10MB body limit)
- `metrolinx/responses.go` — Metrolinx-specific response parsers (NextService, ServiceGlance, UnionDepartures, Fares)
- `gtfs/static.go` — GTFS ZIP parser + thread-safe store. Indexes trips, stop_times, and calendar for O(1) departure lookups. Refreshes every 24h.
- `gtfs/realtime.go` — GTFS-RT protobuf parser, `RealtimeCache` (positions, alerts, trip updates, occupancy, service glance, union departures), background pollers
- `gtfs/departures.go` — departure query logic: merges GTFS static schedule with real-time trip updates, handles service calendar, timezone, and past-midnight trips
- `handlers/` — HTTP handlers with dependency injection. Stop code validated via `^[A-Za-z0-9]{2,10}$`

### API Routes
- `GET /api/health` — health check
- `GET /api/stops` — all GO Transit stops
- `GET /api/departures/{stopCode}` — departures for a station (optional `?dest=` filter)
- `GET /api/union-departures` — Union Station departures (polled from Metrolinx every 30s)
- `GET /api/alerts` — active service alerts
- `GET /api/network-health` — active trains per GO line
- `GET /api/fares/{from}/{to}` — fare info between two stations (1h cache)

### Background Pollers
Started in `main.go` when `METROLINX_API_KEY` is configured:
- **AlertPoller** (30s) — GTFS-RT alerts, enriched with route names from static data
- **TripUpdatePoller** (30s) — GTFS-RT trip updates for delays/cancellations
- **ServiceGlancePoller** (30s) — Metrolinx ServiceGlance API for cars count, isInMotion, lat/lon
- **ExceptionsPoller** (60s) — Metrolinx service exceptions (cancelled trips)
- **UnionDeparturesPoller** (30s) — Metrolinx Union Station departure board
- **OccupancyPoller** (30s) — GTFS-RT VehiclePosition feed for occupancy status strings

### Middleware
- **CORS** — allows configured origins, GET + OPTIONS methods

## Web Structure

Two pages:
- `/` — commute dashboard with countdown timer, network health, fares, and alerts for saved commute routes
- `/board` — standalone split-flap departure board. Defaults to Union Station, with station picker dropdown. Auto-scales font to viewport for TV/kiosk display.

Key files:
- `src/lib/api.ts` — server-only API functions (uses `$env/dynamic/private`)
- `src/lib/api-client.ts` — browser-side fetch wrappers and TypeScript types (`Departure`, `UnionDeparture`, `FareInfo`, `NetworkLine`)
- `src/routes/api/*/+server.ts` — SvelteKit proxy endpoints forwarding to Go API
- `src/routes/+page.server.ts` — loads stops and alerts server-side
- `src/routes/+page.svelte` — renders CommuteDashboard
- `src/routes/board/+page.svelte` — split-flap departure board page
- `src/lib/stores/favorites.ts` — localStorage-backed writable stores (`favorites`, `defaultStation`)

Key components:
- `SplitFlapChar` — single CSS flip-animation character tile (keep usage count low to avoid animation lag)
- `SplitFlapBoard` — commute dashboard board using SplitFlapChar
- `CommuteDashboard` — commute tracking with countdown timer

## Key Conventions

- Go: stdlib `net/http`, `slog` for logging, no external frameworks. Only external dep is `jamespfennell/gtfs` for protobuf parsing
- Frontend: SvelteKit 2 with Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`). No class components
- Svelte 5: `{@const}` must be a direct child of block tags (`{#each}`, `{#if}`, etc.), not nested inside `<div>` or other elements
- Styling: Tailwind CSS 4 via `@tailwindcss/vite` plugin
- Formatting: Prettier with tabs, single quotes, 100 char width. Run `npm run format` before committing
- No user auth — localStorage for favorites and default station
- Input validation on path params (regex) to prevent traversal
- SplitFlapChar causes CSS animation lag when used for many characters (80+). Use plain text for variable-length content like meta-lines.
- Metrolinx ServiceGlance returns `"-"` for cars when no data — filter this on the frontend
- npm overrides for transitive deps can break SvelteKit SSR silently (e.g., cookie 0.7 breaks `@sveltejs/kit` which requires cookie ^0.6.0)

## Environment Variables

### API
| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `METROLINX_API_KEY` | — | Metrolinx OpenData API key (required for real-time) |
| `GTFS_STATIC_URL` | Metrolinx default | URL to GTFS static ZIP |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins (comma-separated) |

### Web
| Variable | Description |
|---|---|
| `API_BASE_URL` | Go API base URL (must include `http://` and port, e.g. `http://localhost:8080`) |
| `PUBLIC_MAPBOX_TOKEN` | Mapbox GL access token |

## Deploy

Railway with Railpack builder. Each service has its own `railway.toml` and watches only its directory.
- API: builds Go binary named `out`, health check at `/api/health`
- Web: `node build/index.js` (Node adapter), health check at `/`
- Internal networking: web connects to API via `http://sixrail-api.railway.internal:8080` (must include protocol + port)

CI: GitHub Actions in `.github/workflows/` — `api.yml` (Go test+vet) and `web.yml` (check+lint+build), triggered by path-filtered pushes/PRs.
