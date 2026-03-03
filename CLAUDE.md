# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Six Rail — GO Transit real-time tracking. Fullscreen Mapbox map with station markers, live vehicle positions, search overlay, alerts dropdown, and departures panel.

## Architecture

Monorepo with two independently deployed services:

- **`api/`** — Go caching proxy. Single gateway to Metrolinx OpenData API. Downloads GTFS static data at startup (refreshes every 24h), polls GTFS-RT positions (10s) and alerts (30s) in background goroutines, and caches departures with TTL + stale fallback.
- **`web/`** — SvelteKit frontend. Server-side `+page.server.ts` loads initial data from Go API. Client-side polling via SvelteKit API proxy routes (`/api/*` → Go API). Mapbox GL JS renders the map with GeoJSON layers.

Data flow: Browser → SvelteKit SSR/proxy routes → Go API → Metrolinx OpenData API

## Commands

### API (Go)
```bash
cd api
go run ./cmd/server/              # start dev server (port 8080)
go test ./... -v                  # run all tests
go test ./internal/cache/ -v      # run single package tests
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

Entry point: `api/cmd/server/main.go` — sets up GTFS stores, pollers, cache, routes, CORS middleware.

Internal packages under `api/internal/`:
- `config/` — env var loading (`METROLINX_API_KEY`, `GTFS_STATIC_URL`, `PORT`, `ALLOWED_ORIGINS`)
- `models/` — shared data structs (`Stop`, `Route`, `VehiclePosition`, `Alert`)
- `metrolinx/` — HTTP client for Metrolinx API (10s timeout, 10MB body limit)
- `gtfs/static.go` — GTFS ZIP parser + thread-safe store with 24h refresh
- `gtfs/realtime.go` — GTFS-RT protobuf parser, `RealtimeCache`, background pollers, route enrichment
- `cache/` — generic TTL cache with `sync.RWMutex`, 5min eviction loop, stale data fallback
- `handlers/` — HTTP handlers with dependency injection. Stop code validated via `^[A-Za-z0-9]{2,10}$`

API routes: `GET /api/health`, `/api/stops`, `/api/departures/{stopCode}`, `/api/positions`, `/api/alerts`

## Web Structure

Single fullscreen map page. All old multi-page routes have been removed.

Key files:
- `src/lib/api.ts` — server-only API functions (uses `$env/dynamic/private`)
- `src/lib/api-client.ts` — browser-side fetch wrappers for client polling
- `src/routes/api/*/+server.ts` — SvelteKit proxy endpoints forwarding to Go API
- `src/routes/+page.server.ts` — loads stops, positions, alerts server-side
- `src/routes/+page.svelte` — fullscreen Mapbox map with overlay components
- `src/lib/stores/favorites.ts` — localStorage-backed writable stores (`favorites`, `defaultStation`)

Components:
- `SearchOverlay` — station search autocomplete (top center)
- `AlertsDropdown` — alert bell + count badge with dropdown (top right)
- `DeparturesPanel` — responsive: bottom sheet (mobile) / side panel (desktop). Auto-refreshes every 30s
- `DepartureBoard` — departure table used inside DeparturesPanel

## Key Conventions

- Go: stdlib `net/http`, `slog` for logging, no external frameworks. Only external dep is `jamespfennell/gtfs` for protobuf parsing
- Frontend: SvelteKit 2 with Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`). No class components
- Styling: Tailwind CSS 4 via `@tailwindcss/vite` plugin
- Formatting: Prettier with tabs, single quotes, 100 char width. Run `npm run format` before committing
- No user auth — localStorage for favorites and default station
- Input validation on path params (regex) to prevent traversal

## Deploy

Railway with Railpack builder. Each service has its own `railway.toml` and watches only its directory.
- API: builds Go binary named `out`, health check at `/api/health`
- Web: `node build/index.js` (Node adapter), health check at `/`

CI: GitHub Actions in `.github/workflows/` — `api.yml` (Go test+vet) and `web.yml` (check+lint+build), triggered by path-filtered pushes/PRs.
