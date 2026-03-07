# Board Enhancements Design

## Features

1. **Cancellations** — Flag cancelled trips on the departure board
2. **Occupancy** — Show how full each train is (3-level indicator)
3. **Fares** — Show PRESTO fare when origin+destination are set
4. **Car count** — Number of coaches per train
5. **Network health bar** — Active trains per line at top of board

## Data Sources

### ServiceataGlance/Trains/All (polled 30s)
Single endpoint providing occupancy, car count, and network health per active trip:
- `TripNumber`, `LineCode`, `Cars`, `OccupancyPercentage`
- `Display` (line display name), `IsInMotion`, `DelaySeconds`

### ServiceUpdate/Exceptions/All (polled 60s)
Cancelled trips and stops:
- `Trip[].TripNumber`, `Trip[].IsCancelled`
- `Trip[].Stop[].Code`, `Trip[].Stop[].IsCancelled`

### Fares/{FromStopCode}/{ToStopCode} (on-demand)
Fare categories and amounts:
- `AllFares.FareCategory[].Type` (e.g., "Adult")
- `AllFares.FareCategory[].Tickets[].Fares[].Amount`

## Architecture

### Go API Changes

**New models** (`models.go`):
- Add `Occupancy int`, `Cars string`, `IsCancelled bool` fields to `Departure`
- `NetworkLine` struct: `LineCode`, `LineName`, `ActiveTrips int`
- `FareInfo` struct: `Category`, `TicketType`, `FareType`, `Amount float64`

**New metrolinx methods** (`responses.go`):
- `GetServiceGlance()` — parse ServiceataGlance/Trains/All
- `GetExceptions()` — parse ServiceUpdate/Exceptions/All
- `GetFares(from, to)` — parse Fares/{from}/{to}

**New caches** (`realtime.go`):
- `ServiceGlanceEntry`: TripNumber, LineCode, LineName, Cars, Occupancy, DelaySeconds
- `serviceGlance map[string]ServiceGlanceEntry` in RealtimeCache (keyed by trip number)
- `cancelledTrips map[string]bool` in RealtimeCache
- Two new pollers: `StartServiceGlancePoller` (30s), `StartExceptionsPoller` (60s)

**Departure enrichment** (`handlers.go`):
- After building departures, cross-reference trip numbers against serviceGlance cache for occupancy + cars
- Cross-reference against cancelledTrips for cancellation flag

**New handlers** (`handlers.go`):
- `NetworkHealth()` — aggregate serviceGlance by lineCode, return []NetworkLine
- `Fares(from, to)` — proxy to metrolinx, return []FareInfo

**New routes** (`main.go`):
- `GET /api/network-health`
- `GET /api/fares/{from}/{to}`

### Frontend Changes

**Types** (`api-client.ts`):
- Add `occupancy?: number`, `cars?: string`, `isCancelled?: boolean` to Departure
- New `NetworkLine` and `FareInfo` types
- New `fetchNetworkHealth()` and `fetchFares(from, to)` functions

**Board** (`SplitFlapBoard.svelte`):
- Add occupancy indicator column (3-level: green/amber/red icon)
- Add car count display next to route name
- Strikethrough + red styling for cancelled departures

**Dashboard** (`CommuteDashboard.svelte`):
- Network health bar at top of board (compact line pills with active train counts)
- Fare display below the board when both origin and destination are set
- Poll network health every 30s alongside departures

**Proxy routes** (SvelteKit):
- `src/routes/api/network-health/+server.ts`
- `src/routes/api/fares/[from]/[to]/+server.ts`

## Implementation Order

1. API: models + metrolinx client methods
2. API: caches + pollers
3. API: departure enrichment
4. API: new handlers + routes
5. Frontend: types + fetch functions + proxy routes
6. Frontend: board UI (occupancy, cars, cancellations)
7. Frontend: network health bar
8. Frontend: fare display
