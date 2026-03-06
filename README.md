# Six Rail

Real-time GO Transit tracking on a fullscreen interactive map. See live train and bus positions, search stations, check upcoming departures, and view service alerts — all in one place.

## Features

- **Live vehicle tracking** — Train and bus positions updated every 10 seconds
- **Station search** — Autocomplete search overlay to quickly find any GO station
- **Departure board** — Upcoming departures with real-time delay information, auto-refreshes every 30s
- **Service alerts** — Active alerts with badge count indicator
- **Favorites** — Save favorite stations and set a default station (stored locally)
- **Responsive** — Bottom sheet on mobile, side panel on desktop

## Architecture

Monorepo with two services:

```
Browser → SvelteKit (SSR + proxy) → Go API → Metrolinx OpenData API
```

- **`api/`** — Go API server. Downloads GTFS static data at startup, polls GTFS-RT feeds for positions, alerts, and trip updates in background goroutines. Merges static schedule with real-time data for accurate departures.
- **`web/`** — SvelteKit frontend. Server-side rendering with client-side polling. Mapbox GL JS renders the map with GeoJSON layers.

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 18+
- A [Metrolinx OpenData API key](https://www.gotransit.com/en/open-data)
- A [Mapbox access token](https://www.mapbox.com/)

### API

```bash
cd api
export METROLINX_API_KEY=your_key_here
go run ./cmd/server/
```

The API starts on port 8080 by default.

### Web

```bash
cd web
npm install
```

Create a `.env` file in `web/`:

```
API_BASE_URL=http://localhost:8080
PUBLIC_MAPBOX_TOKEN=your_mapbox_token
```

```bash
npm run dev
```

The frontend starts on port 5173.

## Environment Variables

### API

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `METROLINX_API_KEY` | — | Metrolinx OpenData API key (required) |
| `GTFS_STATIC_URL` | Metrolinx default | URL to GTFS static ZIP |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins |

### Web

| Variable | Description |
|---|---|
| `API_BASE_URL` | Go API base URL |
| `PUBLIC_MAPBOX_TOKEN` | Mapbox GL access token |

## API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/api/health` | Health check |
| GET | `/api/stops` | All GO Transit stops |
| GET | `/api/positions` | Live vehicle positions |
| GET | `/api/alerts` | Active service alerts |
| GET | `/api/departures/{stopCode}` | Departures for a station |

## Tech Stack

- **Backend:** Go stdlib (`net/http`, `slog`), GTFS protobuf parsing
- **Frontend:** SvelteKit 2, Svelte 5, Mapbox GL JS, Tailwind CSS 4
- **Deploy:** Railway with Railpack builder
- **CI:** GitHub Actions (Go test/vet, SvelteKit check/lint/build)

## Development

```bash
# API tests
cd api && go test ./... -v

# Web type checking + linting
cd web && npm run check && npm run lint

# Auto-format
cd web && npm run format
```
