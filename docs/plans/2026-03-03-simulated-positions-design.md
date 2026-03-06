# Simulated Vehicle Positions Design

**Date:** 2026-03-03
**Status:** Completed

## Problem

The Metrolinx OpenData API key can take up to 10 days to provision. Without it, the map shows no vehicle positions. Departures already work from GTFS static data, but the map is empty.

## Goal

When `METROLINX_API_KEY` is absent, simulate vehicle positions from the GTFS static schedule so the map shows active trains moving in real time. Zero frontend changes required. When the key arrives, set the env var and redeploy — real data takes over automatically.

## Architecture

```
METROLINX_API_KEY set?
  yes → real GTFS-RT pollers (existing, unchanged)
  no  → StartSimulatedPositionPoller (new)
          → SimulatePositions(now, static)
          → RealtimeCache.SetPositions(...)
```

The simulated poller writes to the same `RealtimeCache.positions` field as the real poller. Handlers and frontend are untouched.

## Algorithm: SimulatePositions

For each trip active at time `now`:

1. Check that the trip's service is active today via the existing `serviceActive()` logic.
2. Walk the trip's stop sequence to find the current segment — the pair `(stopA, stopB)` where `stopA.DepartureTime ≤ nowOffset < stopB.ArrivalTime` (where `nowOffset` is the duration since midnight of the service day).
3. Compute interpolation fraction: `t = (nowOffset - stopA.DepartureTime) / (stopB.ArrivalTime - stopA.DepartureTime)`, clamped to [0, 1].
4. Interpolate position: `lat = stopA.Lat + t*(stopB.Lat - stopA.Lat)`, same for lon.
5. Compute bearing from the vector `(stopA→stopB)` using `math.Atan2`.
6. Emit a `VehiclePosition` with `VehicleID = tripID`, route name and color from the route lookup.

Trips that haven't started or have already terminated are skipped. Trips at or past their last stop are also skipped.

## Static Store Additions

`static.go` currently stores only a `stopIndex` (stopID → departures). Simulation needs the full stop sequence per trip with lat/lon at each stop.

**New type:**
```go
type TripStop struct {
    StopID        string
    Lat           float64
    Lon           float64
    ArrivalTime   time.Duration
    DepartureTime time.Duration
}
```

**New field on `StaticStore`:**
```go
tripIndex map[string][]TripStop // tripID → ordered stop sequence
```

Built in `load()` alongside the existing `stopIndex`. No re-parsing needed.

**New exported method:**
```go
func (s *StaticStore) TripStops(tripID string) []TripStop
func (s *StaticStore) AllTripIDs() []string  // or iterate via ActiveTrips
```

Actually cleaner: one method that returns everything simulation needs:
```go
type SimTrip struct {
    TripID    string
    RouteID   string
    ServiceID string
    Stops     []TripStop
}

func (s *StaticStore) ActiveSimTrips(now time.Time) []SimTrip
```

This keeps the simulation logic self-contained and the static store locked only once per call.

## Files Changed

| File | Change |
|------|--------|
| `api/internal/gtfs/static.go` | Add `TripStop`, `SimTrip` types; build `tripIndex` in `load()`; expose `ActiveSimTrips(now)` |
| `api/internal/gtfs/simulate.go` | **New** — `SimulatePositions()` + `StartSimulatedPositionPoller()` |
| `api/cmd/server/main.go` | Toggle real vs simulated pollers based on `cfg.MetrolinxAPIKey` |

## Behavior

- **No API key:** simulated positions update every 10s, trains appear to move between scheduled stops.
- **API key present:** existing GTFS-RT position poller runs, alerts and trip updates also start. Simulation is never started.
- **Switching:** add `METROLINX_API_KEY` env var on Railway and redeploy — no code changes needed.

## Out of Scope

- Shape-based interpolation (straight-line between stops is sufficient)
- Simulated alerts or trip updates (empty is fine while waiting for key)
- Any frontend indicator distinguishing simulated from real positions
