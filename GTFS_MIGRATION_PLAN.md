# GTFS Migration Plan — Replace Metrolinx REST API with GTFS Feeds

## Goal

Remove the proprietary Metrolinx REST API dependency for departures and move entirely to GTFS feeds (static schedule + GTFS-RT TripUpdates). Positions and alerts already use GTFS-RT feeds — only the departures system needs to change.

## Research Findings

### No Public GTFS-RT URLs Exist

The Metrolinx OpenData API (`api.openmetrolinx.com`) with an API key is the **only source** for GO Transit real-time data. There are no unauthenticated public GTFS-RT feed URLs. The API key is still required — we cannot fully eliminate it. However, we can eliminate the non-standard REST endpoints.

Checked: Transitland, TransitFeeds/OpenMobilityData, Mobility Database, metrolinx.tmix.se (municipal only). None provide GO Transit GTFS-RT feeds.

### Available Metrolinx GTFS-RT Endpoints

| Feed | Path | Currently Used |
|------|------|----------------|
| Vehicle Positions | `/Gtfs/Feed/VehiclePosition` | Yes (10s poll) |
| Alerts | `/Gtfs/Feed/Alerts` | Yes (30s poll) |
| **Trip Updates** | `/Gtfs/Feed/TripUpdates` | **No — this is the key addition** |
| UP Express (3 feeds) | `/UP/Gtfs/Feed/*` | No |
| Fleet Occupancy (3 feeds) | `/Fleet/Occupancy/GtfsRT/Feed/*` | No |

### What Changes

| Component | Current | Target |
|-----------|---------|--------|
| Positions | GTFS-RT via Metrolinx API | No change |
| Alerts | GTFS-RT via Metrolinx API | No change |
| **Departures** | REST `/Stop/NextService/{stopCode}` (JSON) | **GTFS static schedule + GTFS-RT TripUpdates** |

## Current Architecture (Departures Only)

```
Browser → SvelteKit proxy (/api/departures/[stopCode])
  → Go handler StopDepartures()
    → cachedProxy() with 30s TTL
      → metrolinx.Client.Fetch("/Stop/NextService/{stopCode}")
        → Metrolinx REST API (proprietary JSON format)
```

The REST API returns Metrolinx-specific JSON with fields like `LineName`, `Destination`, `ScheduledTime`, `Status`, `Platform`, `Late`, `Delayed`. The frontend `DepartureBoard.svelte` renders these directly (untyped `any[]`).

## Target Architecture (Departures)

```
Startup:
  1. Download GTFS ZIP (already done)
  2. Parse stops, routes (already done)
  3. NEW: Parse trips, stop_times, calendar, calendar_dates → build schedule index
  4. NEW: Start TripUpdates poller (every 30s)

Request for /api/departures/{stopCode}:
  1. Look up stop by code
  2. Get today's active service IDs from calendar + calendar_dates
  3. Find upcoming stop_times for this stop, filtered by active services
  4. Overlay real-time updates from cached TripUpdates (delays, cancellations)
  5. Return merged departures sorted by departure time
```

## Implementation Steps

### Step 1: Expand GTFS Static Parsing (`api/internal/gtfs/static.go`)

Currently parses: stops, routes.
Need to add: **trips, stop_times, calendar, calendar_dates**.

Build an index structure for efficient lookups:
- `stopIndex`: map[stopID] → []scheduledDeparture (sorted by departure time)
- `tripIndex`: map[tripID] → trip (route, headsign, service, direction)
- `serviceCalendar`: map[serviceID] → {weekday booleans, start/end date}
- `serviceExceptions`: map[serviceID] → map[date] → added/removed

The `jamespfennell/gtfs` library (`v0.1.24`) parses the full GTFS ZIP. The `gtfs.ParseStatic()` result has `Trips`, `StopTimes`, `Services` (calendar), etc. — currently unused.

Key considerations:
- **Timezone**: GO Transit operates in `America/Toronto`. Stop times in GTFS are local time strings (e.g., "08:30:00"). Times can exceed 24:00:00 for trips past midnight.
- **Memory**: stop_times.txt can be large. Index by stop_id for O(1) per-stop lookups.
- **Thread safety**: Wrap in existing `sync.RWMutex` pattern.

### Step 2: Add TripUpdates to Realtime Layer (`api/internal/gtfs/realtime.go`)

Follow the existing pattern (same as positions/alerts):

1. Add `RawTripUpdate` struct with: tripID, routeID, stopTimeUpdates (stopID, arrival delay, departure delay, schedule relationship)
2. Add `ParseTripUpdates(data []byte)` function — parse GTFS-RT protobuf
3. Add to `RealtimeCache`: `tripUpdates` field with `Set`/`Get` methods
4. Add `StartTripUpdatePoller()` — poll `/Gtfs/Feed/TripUpdates` every 30s

Build a lookup: `map[tripID][]StopTimeUpdate` for fast merging with static schedule.

### Step 3: Add Departure Model (`api/internal/models/models.go`)

New `Departure` struct:
```go
type Departure struct {
    Line          string `json:"line"`
    Destination   string `json:"destination"`
    ScheduledTime string `json:"scheduledTime"` // "HH:MM" local time
    Status        string `json:"status"`         // "On Time", "Delayed +5m", "Cancelled"
    Platform      string `json:"platform"`       // from stop_times if available
    RouteColor    string `json:"routeColor"`
    DelayMinutes  int    `json:"delayMinutes,omitempty"`
}
```

### Step 4: Build Departure Query Logic

New file or function in `api/internal/gtfs/` — `departures.go`:

```
GetDepartures(stopCode string, now time.Time) → []Departure
```

Algorithm:
1. Resolve stop code → stop ID(s) (a station may have child stops)
2. Determine active service IDs for `now` using calendar + exceptions
3. Filter stop_times for this stop where service is active and departure ≥ now
4. For each match, look up trip → get route name, headsign (destination), direction
5. Check TripUpdates cache for real-time adjustments (delay seconds, cancellations)
6. Compute status: "On Time" if no update or delay < 60s, "Delayed +Xm" if delay, "Cancelled" if schedule relationship = CANCELED
7. Sort by adjusted departure time, return first N (e.g., 20)

Handle midnight rollover: trips with departure times ≥ 24:00:00 belong to the previous service day.

### Step 5: Update Handler (`api/internal/handlers/handlers.go`)

- Replace `StopDepartures` to call the new GTFS departure query instead of `cachedProxy()`
- Remove the `Fetcher` interface and `cachedProxy()` method (no longer needed for departures)
- Remove `cache *cache.Cache` from `Handlers` struct (TTL cache was only for departures)
- Keep `Fetcher` if still needed by realtime pollers (but pollers use their own `Fetcher` interface in `realtime.go`)

Actually, the `Fetcher` interface in `handlers.go` is **only** used by `cachedProxy()`. The realtime pollers use a separate `Fetcher` interface defined in `realtime.go`. So the handler's `Fetcher` and `cache.Cache` can both be removed.

### Step 6: Update Main (`api/cmd/server/main.go`)

- Start TripUpdates poller alongside position/alert pollers
- Remove `cache.New()` (no longer needed)
- Update `handlers.New()` — pass schedule store instead of fetcher + cache
- Keep `metrolinx.NewClient()` — still needed for GTFS-RT pollers

### Step 7: Update Frontend Types (`web/src/lib/components/DepartureBoard.svelte`)

The new Go API will return typed `Departure` objects instead of Metrolinx's proprietary JSON. Update the Svelte component to use the new field names (`line`, `destination`, `scheduledTime`, `status`, `platform`).

Also add a TypeScript `Departure` type in `api-client.ts` or `api.ts` to replace the current `any[]`.

### Step 8: Remove Dead Code

- Delete `api/internal/cache/` package (TTL cache only served departures proxy)
- Clean up `cachedProxy()` from handlers
- Remove `Fetcher` interface from handlers (keep the one in `realtime.go`)
- Remove `METROLINX_BASE_URL` from config if desired (but still needed for GTFS-RT — keep it)

### Step 9: Update CLAUDE.md

Reflect the new architecture: no more REST proxy for departures, document the TripUpdates poller, remove references to `cache/` package.

## Files to Modify

| File | Change |
|------|--------|
| `api/internal/gtfs/static.go` | Add trips, stop_times, calendar parsing + schedule index |
| `api/internal/gtfs/realtime.go` | Add TripUpdates parsing, cache, poller |
| `api/internal/models/models.go` | Add `Departure` struct |
| `api/internal/gtfs/departures.go` | **New** — departure query logic |
| `api/internal/handlers/handlers.go` | Replace `StopDepartures`, remove `cachedProxy`, `Fetcher`, `cache` |
| `api/cmd/server/main.go` | Add TripUpdates poller, remove cache, update handler init |
| `api/internal/config/config.go` | No change needed (keep API key + base URL for GTFS-RT) |
| `api/internal/cache/cache.go` | **Delete** (no longer needed) |
| `api/internal/cache/cache_test.go` | **Delete** |
| `web/src/lib/components/DepartureBoard.svelte` | Update field names to match new Departure type |
| `web/src/lib/api-client.ts` | Add `Departure` type |
| `CLAUDE.md` | Update architecture docs |

## Key Design Decisions

1. **Schedule index in memory**: Build a `map[stopID][]scheduledDeparture` at startup from stop_times.txt. This avoids scanning the full stop_times list on every request.

2. **TripUpdates polling at 30s**: Matches the alert poller interval. The TripUpdates feed is smaller than positions and changes less frequently.

3. **Departure response format**: Use clean lowercase JSON field names (`line`, `destination`, `scheduledTime`) instead of matching Metrolinx's PascalCase. Update the frontend accordingly.

4. **Parent station handling**: A stop code might map to a parent station with multiple child stops (platforms). Query all child stop IDs when looking up departures.

5. **Stale data fallback**: If TripUpdates fetch fails, return static schedule data without real-time updates (graceful degradation — same pattern as current positions/alerts).

## Verification

1. `cd api && go test ./... -v` — all existing tests pass + new tests for schedule parsing and departure queries
2. `cd api && go vet ./...` — no static analysis issues
3. `cd api && go run ./cmd/server/` — server starts, `/api/departures/UN` returns GTFS-based departures
4. `cd web && npm run check` — TypeScript checks pass
5. `cd web && npm run dev` — DeparturesPanel shows departures with correct fields
6. Compare output against current Metrolinx REST response to verify data accuracy
