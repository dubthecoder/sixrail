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
