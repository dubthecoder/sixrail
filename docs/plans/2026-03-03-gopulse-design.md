# GoPulse Design Document

**Date**: 2026-03-03
**Status**: Approved

## Overview

GoPulse is a GO Transit tracking mini site for the general public — commuters, trip planners, and anyone checking train status. It provides real-time departures, live train map, service alerts, schedule browsing, journey planning, and fare lookup.

## Architecture

Two microservices deployed on Railway:

```
┌─────────────┐     ┌──────────────────┐     ┌──────────────────┐
│  SvelteKit   │────▶│   Go API Service  │────▶│ Metrolinx API    │
│  (Railway)   │     │   (Railway)       │     │ openmetrolinx.com│
│              │     │  - In-memory cache│     └──────────────────┘
│  SSR + hydra │     │  - Rate limiting  │
│  Mapbox map  │     │  - Unified REST   │
│  localStorage│     │    JSON endpoints │
└─────────────┘     └──────────────────┘
```

- **gopulse-api**: Go service that caches and proxies the Metrolinx OpenData API
- **gopulse-web**: SvelteKit frontend with SSR, Tailwind CSS, Mapbox GL JS

Connected via Railway internal networking.

## Data Source

Metrolinx OpenData API at `api.openmetrolinx.com`. Requires API key registration. Supports JSON, XML, and protobuf responses. We use JSON.

## Go API Service

### Endpoints

| Endpoint | Metrolinx Source | Cache TTL | Description |
|---|---|---|---|
| `GET /api/departures/union` | `ServiceUpdate/UnionDepartures/All` | 30s | Live Union Station departure board |
| `GET /api/departures/:stopCode` | `Stop/NextService/{StopCode}` | 30s | Next service predictions for any stop |
| `GET /api/trains` | `ServiceataGlance/Trains/All` | 30s | All in-service train trips |
| `GET /api/trains/positions` | `Gtfs/Feed/VehiclePosition` | 15s | Live GPS positions for map |
| `GET /api/alerts/service` | `ServiceUpdate/ServiceAlert/All` | 60s | Service disruptions |
| `GET /api/alerts/info` | `ServiceUpdate/InformationAlert/All` | 60s | Info alerts |
| `GET /api/exceptions` | `ServiceUpdate/Exceptions/All` | 60s | Cancellations, modified trips |
| `GET /api/schedule/lines/:date` | `Schedule/Line/All/{Date}` | 1h | All lines for a date |
| `GET /api/schedule/journey` | `Schedule/Journey/...` | 5m | Journey planner |
| `GET /api/fares/:from/:to` | `Fares/{From}/{To}` | 24h | Fare lookup |
| `GET /api/stops` | `Stop/All` | 24h | All stops/stations |
| `GET /api/stops/:code` | `Stop/Details/{StopCode}` | 24h | Stop details |
| `GET /api/health` | — | — | Health check |

### Tech Stack

- Go 1.22+ with `net/http` stdlib mux (or chi)
- In-memory TTL cache (`sync.Map` or custom)
- Structured logging with `slog`
- Config via environment variables: `METROLINX_API_KEY`, `PORT`, `ALLOWED_ORIGINS`

## SvelteKit Frontend

### Pages

| Route | Description |
|---|---|
| `/` | Station picker hero + active alerts. Saved station redirects to departures. |
| `/departures/:stopCode` | Departure board for any station, auto-refreshes every 30s |
| `/map` | Live train map (Mapbox GL JS) with vehicle positions, 15s polling |
| `/alerts` | All service alerts, info alerts, exceptions — filterable |
| `/schedule` | Schedule explorer — pick date, browse lines, view trip stops |
| `/journey` | Journey planner — from/to station, date/time, results with fares |
| `/stations` | All stations with search, click for departures |

### Key Components

- **DepartureBoard** — real-time table, auto-refreshes via `setInterval` + `invalidate()`
- **TrainMap** — Mapbox GL JS, polls positions every 15s, animated markers
- **AlertBanner** — sticky banner for active service disruptions
- **StationPicker** — searchable dropdown for journey planner and departures
- **FavoriteStations** — localStorage-backed, star icon on station pages

### Homepage Behavior

- First visit: station picker as hero, active alerts below
- User picks a station: saved in localStorage, redirected to `/departures/:stopCode`
- Returning users: auto-redirect to saved station's departures

### Tech Stack

- SvelteKit with Node adapter (for Railway)
- Tailwind CSS
- Mapbox GL JS
- Data fetched via `+page.server.ts` load functions from Go API
- Client-side polling for real-time updates

## Repo Structure

```
gopulse/
├── api/                    # Go API service
│   ├── cmd/server/         # main.go entrypoint
│   ├── internal/
│   │   ├── metrolinx/      # Metrolinx API client
│   │   ├── cache/          # TTL cache
│   │   ├── handlers/       # HTTP handlers
│   │   └── models/         # Data types
│   ├── go.mod
│   ├── railway.toml        # Railpack build config for API service
│   └── .env.example
├── web/                    # SvelteKit frontend
│   ├── src/
│   │   ├── routes/         # Pages
│   │   ├── lib/            # Shared components, stores
│   │   └── app.html
│   ├── package.json
│   ├── svelte.config.js
│   ├── railway.toml        # Railpack build config for web service
│   └── .env.example
├── .github/workflows/
│   ├── api.yml             # Go: test, lint, deploy on api/** changes
│   └── web.yml             # Svelte: check, lint, build, deploy on web/** changes
├── docs/plans/
└── CLAUDE.md
```

## Build & Deploy

**Railpack** handles builds for both services. Each service has its own `railway.toml` in its directory:

**api/railway.toml**:
- Railpack auto-detects Go, builds the binary
- Sets start command, health check path

**web/railway.toml**:
- Railpack auto-detects Node/SvelteKit, runs `npm run build`
- Sets start command for the Node adapter

Each `railway.toml` is configured as a separate Railway service in the same project.

## CI/CD

GitHub Actions with path-filtered workflows:

**api.yml** (triggers on `api/**`):
1. `go test ./...`
2. `golangci-lint run`
3. Railway auto-deploys via Railpack on merge to `main`

**web.yml** (triggers on `web/**`):
1. `npm run check` (svelte-check)
2. `npm run lint`
3. `npm run build`
4. Railway auto-deploys via Railpack on merge to `main`

Railway watches the repo and uses each service's `railway.toml` + Railpack for builds. GitHub Actions gate quality before merge.

## Environment Variables

**api**:
- `METROLINX_API_KEY` — API key from openmetrolinx.com registration
- `PORT` — Railway-assigned port
- `ALLOWED_ORIGINS` — CORS origins (gopulse-web URL)

**web**:
- `API_BASE_URL` — Go API internal URL (`http://gopulse-api.railway.internal:PORT`)
- `PUBLIC_MAPBOX_TOKEN` — Mapbox GL JS access token
- `PORT` — Railway-assigned port

## Error Handling

- **Metrolinx API down**: Serve stale cached data with `X-Cache-Stale: true` header. Frontend shows "data may be outdated" banner.
- **Rate limiting**: Go service controls all Metrolinx requests. Frontend never hits Metrolinx directly.
- **Empty data off-peak**: Show "no active trains" instead of errors.
- **API key rotation**: Environment variable swap on Railway, zero-downtime.

## Personalization

- No user accounts
- Browser localStorage for: favorite stations, default station, theme preference
- Station picker remembers last selection
