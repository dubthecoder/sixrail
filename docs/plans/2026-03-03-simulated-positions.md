# Simulated Vehicle Positions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When `METROLINX_API_KEY` is absent, compute synthetic vehicle positions from the GTFS static schedule and write them to `RealtimeCache` so the map shows active trains moving — zero frontend changes needed.

**Architecture:** `main.go` checks `cfg.MetrolinxAPIKey`; if empty it starts `StartSimulatedPositionPoller` instead of the real GTFS-RT pollers. The simulated poller calls `SimulatePositions(now, static)` every 10s, which walks all active trips, finds each train's current segment between two stops, and linearly interpolates lat/lon. Output is written to `RealtimeCache.SetPositions` — the same field the real poller writes, so no handler or frontend changes are needed.

**Tech Stack:** Go stdlib (`math`, `time`), existing `StaticStore` + `RealtimeCache`, no new dependencies.

---

### Task 1: Add `TripStop` + `tripIndex` to `StaticStore`

**Files:**
- Modify: `api/internal/gtfs/static.go`

The simulation needs to walk a trip's ordered stop sequence with lat/lon and arrival/departure times. Currently `static.go` only stores a `stopIndex` (keyed by stopID for departures). We need a `tripIndex` (keyed by tripID → ordered `[]TripStop`).

**Step 1: Add the `TripStop` type and `tripIndex` field**

In `static.go`, add after the `ScheduledDeparture` type:

```go
// TripStop is one stop in a trip's ordered sequence, used for position simulation.
type TripStop struct {
	StopID        string
	Lat           float64
	Lon           float64
	ArrivalTime   time.Duration // duration from midnight of service day
	DepartureTime time.Duration
}

// SimTrip holds a trip's identity and full stop sequence for position simulation.
type SimTrip struct {
	TripID    string
	RouteID   string
	ServiceID string
	Stops     []TripStop
}
```

And add `tripIndex` to the `StaticStore` struct:

```go
type StaticStore struct {
	mu         sync.RWMutex
	stops      []models.Stop
	routes     map[string]models.Route
	stopIndex  map[string][]ScheduledDeparture
	stopCodes  map[string][]string
	services   map[string]*gtfs.Service
	tripIndex  map[string]SimTrip // tripID → SimTrip
}
```

**Step 2: Build `tripIndex` in `load()`**

Inside `load()`, after building `stopIndex`, add:

```go
// --- Trip index for position simulation ---
tripIndex := make(map[string]SimTrip, len(static.Trips))
for i := range static.Trips {
	trip := &static.Trips[i]
	if trip.Route == nil || trip.Service == nil {
		continue
	}
	stops := make([]TripStop, 0, len(trip.StopTimes))
	for j := range trip.StopTimes {
		st := &trip.StopTimes[j]
		if st.Stop == nil || st.Stop.Latitude == nil || st.Stop.Longitude == nil {
			continue
		}
		stops = append(stops, TripStop{
			StopID:        st.Stop.Id,
			Lat:           *st.Stop.Latitude,
			Lon:           *st.Stop.Longitude,
			ArrivalTime:   st.ArrivalTime,
			DepartureTime: st.DepartureTime,
		})
	}
	if len(stops) < 2 {
		continue // need at least 2 stops to interpolate
	}
	tripIndex[trip.ID] = SimTrip{
		TripID:    trip.ID,
		RouteID:   trip.Route.Id,
		ServiceID: trip.Service.Id,
		Stops:     stops,
	}
}
```

Also add `tripIndex` to the `s.mu.Lock()` block:

```go
s.mu.Lock()
s.stops = stops
s.routes = routes
s.stopIndex = stopIndex
s.stopCodes = stopCodes
s.services = services
s.tripIndex = tripIndex
s.mu.Unlock()
```

**Step 3: Add `ActiveSimTrips(now time.Time) []SimTrip` method**

```go
// ActiveSimTrips returns all trips whose service is active on the given date.
// Used by the position simulator to find trips currently running.
func (s *StaticStore) ActiveSimTrips(now time.Time) []SimTrip {
	loc, _ := time.LoadLocation("America/Toronto")
	nowLocal := now.In(loc)
	today := truncateToDay(nowLocal)

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]SimTrip, 0, 256)
	for _, trip := range s.tripIndex {
		svc, ok := s.services[trip.ServiceID]
		if !ok {
			continue
		}
		if serviceActive(svc, today) {
			out = append(out, trip)
		}
	}
	return out
}
```

**Step 4: Write the failing test in `static_test.go`**

The existing `buildTestZip` has one trip (`T001`) with one stop. Extend it to have two stops so `tripIndex` includes it, then test `ActiveSimTrips`.

Add a new helper `buildSimTestZip` and test:

```go
func buildSimTestZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	files := map[string]string{
		"agency.txt":   "agency_id,agency_name,agency_url,agency_timezone\nMX,Metrolinx,https://metrolinx.com,America/Toronto\n",
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWD,1,1,1,1,1,0,0,20260101,20261231\n",
		"routes.txt":   "route_id,agency_id,route_short_name,route_long_name,route_type,route_color,route_text_color\n01,MX,LW,Lakeshore West,2,098137,FFFFFF\n",
		"stops.txt":    "stop_id,stop_code,stop_name,stop_lat,stop_lon,location_type,parent_station\nUN,UN,Union Station,43.6453,-79.3806,1,\nMI,MI,Mimico,43.6200,-79.4900,1,\n",
		"trips.txt":    "route_id,service_id,trip_id,direction_id\n01,WD,T001,0\n",
		// T001: UN departs 08:00, arrives Mimico 08:20
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT001,08:00:00,08:00:00,UN,1\nT001,08:20:00,08:20:00,MI,2\n",
	}
	for name, content := range files {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestStaticStore_ActiveSimTrips_WeekdayMatch(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildSimTestZip(t))
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-03 is a Tuesday — should match WD (weekday) service
	tuesday := time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	trips := store.ActiveSimTrips(tuesday)
	if len(trips) != 1 {
		t.Fatalf("expected 1 active trip, got %d", len(trips))
	}
	if trips[0].TripID != "T001" {
		t.Fatalf("expected T001, got %s", trips[0].TripID)
	}
	if len(trips[0].Stops) != 2 {
		t.Fatalf("expected 2 stops, got %d", len(trips[0].Stops))
	}
}

func TestStaticStore_ActiveSimTrips_WeekendNoMatch(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildSimTestZip(t))
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-07 is a Saturday — WD service is not active
	saturday := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)
	trips := store.ActiveSimTrips(saturday)
	if len(trips) != 0 {
		t.Fatalf("expected 0 active trips on weekend, got %d", len(trips))
	}
}
```

**Step 5: Run tests to verify they fail**

```bash
cd api && go test ./internal/gtfs/... -run TestStaticStore_ActiveSimTrips -v
```
Expected: compile error — `ActiveSimTrips` not yet defined.

**Step 6: Implement (Steps 1–3 above), run tests**

```bash
cd api && go test ./internal/gtfs/... -run TestStaticStore_ActiveSimTrips -v
```
Expected: both tests PASS.

**Step 7: Run full test suite**

```bash
cd api && go test ./... -v
```
Expected: all existing tests still pass.

**Step 8: Commit**

```bash
git add api/internal/gtfs/static.go api/internal/gtfs/static_test.go
git commit -m "feat(gtfs): add TripStop/SimTrip index and ActiveSimTrips for position simulation"
```

---

### Task 2: Create `simulate.go` — position computation

**Files:**
- Create: `api/internal/gtfs/simulate.go`

**Step 1: Write the failing test in `simulate_test.go`**

Create `api/internal/gtfs/simulate_test.go`:

```go
package gtfs_test

import (
	"archive/zip"
	"bytes"
	"math"
	"testing"
	"time"

	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
)

func buildSimZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	files := map[string]string{
		"agency.txt":   "agency_id,agency_name,agency_url,agency_timezone\nMX,Metrolinx,https://metrolinx.com,America/Toronto\n",
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWD,1,1,1,1,1,0,0,20260101,20261231\n",
		"routes.txt":   "route_id,agency_id,route_short_name,route_long_name,route_type,route_color,route_text_color\n01,MX,LW,Lakeshore West,2,098137,FFFFFF\n",
		// Two stops: UN (43.6453, -79.3806) and MI (43.6200, -79.4900)
		"stops.txt": "stop_id,stop_code,stop_name,stop_lat,stop_lon,location_type,parent_station\nUN,UN,Union Station,43.6453,-79.3806,1,\nMI,MI,Mimico,43.6200,-79.4900,1,\n",
		"trips.txt":  "route_id,service_id,trip_id,direction_id\n01,WD,T001,0\n",
		// T001: departs UN 09:00, arrives Mimico 09:20 (20 min trip)
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT001,09:00:00,09:00:00,UN,1\nT001,09:20:00,09:20:00,MI,2\n",
	}
	for name, content := range files {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestSimulatePositions_Midpoint(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildSimZip(t))
	if err != nil {
		t.Fatal(err)
	}
	// Tuesday 2026-03-03, 09:10 Toronto time = exactly midpoint of the trip
	loc, _ := time.LoadLocation("America/Toronto")
	midpoint := time.Date(2026, 3, 3, 9, 10, 0, 0, loc)

	positions := gtfsstore.SimulatePositions(midpoint, store)

	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	p := positions[0]
	if p.VehicleID != "T001" {
		t.Errorf("expected VehicleID T001, got %s", p.VehicleID)
	}

	// At midpoint (t=0.5): lat = 43.6453 + 0.5*(43.6200-43.6453) = 43.63265
	wantLat := 43.6453 + 0.5*(43.6200-43.6453)
	wantLon := -79.3806 + 0.5*(-79.4900-(-79.3806))
	if math.Abs(p.Lat-wantLat) > 0.0001 {
		t.Errorf("lat: want %.4f, got %.4f", wantLat, p.Lat)
	}
	if math.Abs(p.Lon-wantLon) > 0.0001 {
		t.Errorf("lon: want %.4f, got %.4f", wantLon, p.Lon)
	}
}

func TestSimulatePositions_BeforeTrip(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildSimZip(t))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Toronto")
	// 08:00 — before the 09:00 trip starts
	before := time.Date(2026, 3, 3, 8, 0, 0, 0, loc)
	positions := gtfsstore.SimulatePositions(before, store)
	if len(positions) != 0 {
		t.Errorf("expected 0 positions before trip start, got %d", len(positions))
	}
}

func TestSimulatePositions_AfterTrip(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildSimZip(t))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Toronto")
	// 10:00 — after the 09:20 trip ends
	after := time.Date(2026, 3, 3, 10, 0, 0, 0, loc)
	positions := gtfsstore.SimulatePositions(after, store)
	if len(positions) != 0 {
		t.Errorf("expected 0 positions after trip end, got %d", len(positions))
	}
}
```

**Step 2: Run tests to confirm they fail**

```bash
cd api && go test ./internal/gtfs/... -run TestSimulatePositions -v
```
Expected: compile error — `SimulatePositions` not yet defined.

**Step 3: Create `simulate.go`**

Create `api/internal/gtfs/simulate.go`:

```go
package gtfs

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/teclara/sixrail/api/internal/models"
)

// SimulatePositions computes synthetic vehicle positions for all active trips at time now.
// For each trip, it finds the current segment between two stops and linearly interpolates
// the position. Trips not yet started or already finished are skipped.
func SimulatePositions(now time.Time, static *StaticStore) []models.VehiclePosition {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		loc = time.UTC
	}
	nowLocal := now.In(loc)
	midnight := truncateToDay(nowLocal)
	nowOffset := nowLocal.Sub(midnight) // duration since midnight

	trips := static.ActiveSimTrips(now)
	positions := make([]models.VehiclePosition, 0, len(trips))

	for _, trip := range trips {
		pos, ok := interpolatePosition(trip, nowOffset)
		if !ok {
			continue
		}

		route, _ := static.GetRoute(trip.RouteID)
		positions = append(positions, models.VehiclePosition{
			VehicleID:  trip.TripID,
			TripID:     trip.TripID,
			RouteID:    trip.RouteID,
			RouteName:  route.LongName,
			RouteColor: route.Color,
			Lat:        pos.lat,
			Lon:        pos.lon,
			Bearing:    pos.bearing,
			Timestamp:  now.Unix(),
		})
	}

	return positions
}

type interpResult struct {
	lat     float64
	lon     float64
	bearing float32
}

// interpolatePosition finds the segment the trip is currently on and returns
// the interpolated lat/lon and bearing. Returns false if the trip is not active now.
func interpolatePosition(trip SimTrip, nowOffset time.Duration) (interpResult, bool) {
	stops := trip.Stops

	// Trip hasn't started yet.
	if nowOffset < stops[0].DepartureTime {
		return interpResult{}, false
	}
	// Trip has already finished.
	if nowOffset >= stops[len(stops)-1].ArrivalTime {
		return interpResult{}, false
	}

	// Find the segment: stopA.DepartureTime <= nowOffset < stopB.ArrivalTime
	for i := 0; i < len(stops)-1; i++ {
		a := stops[i]
		b := stops[i+1]
		if nowOffset < a.DepartureTime || nowOffset >= b.ArrivalTime {
			continue
		}
		segDur := b.ArrivalTime - a.DepartureTime
		if segDur <= 0 {
			continue
		}
		t := float64(nowOffset-a.DepartureTime) / float64(segDur)
		t = clamp(t, 0, 1)

		lat := a.Lat + t*(b.Lat-a.Lat)
		lon := a.Lon + t*(b.Lon-a.Lon)
		bearing := bearingDeg(a.Lat, a.Lon, b.Lat, b.Lon)

		return interpResult{lat: lat, lon: lon, bearing: float32(bearing)}, true
	}

	return interpResult{}, false
}

// bearingDeg returns the compass bearing (0–360°) from point a to point b.
func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1R := lat1 * math.Pi / 180
	lat2R := lat2 * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2R)
	x := math.Cos(lat1R)*math.Sin(lat2R) - math.Sin(lat1R)*math.Cos(lat2R)*math.Cos(dLon)
	deg := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(deg+360, 360)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// StartSimulatedPositionPoller launches a background goroutine that computes
// synthetic vehicle positions from the GTFS static schedule every interval.
// Use this when no Metrolinx API key is available.
func StartSimulatedPositionPoller(ctx context.Context, static *StaticStore, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		computeAndCachePositions(static, cache)

		for {
			select {
			case <-ctx.Done():
				slog.Info("simulated position poller stopped")
				return
			case <-ticker.C:
				computeAndCachePositions(static, cache)
			}
		}
	}()
}

func computeAndCachePositions(static *StaticStore, cache *RealtimeCache) {
	positions := SimulatePositions(time.Now(), static)
	cache.SetPositions(positions)
	slog.Info("simulated positions updated", "count", len(positions))
}
```

**Step 4: Run tests**

```bash
cd api && go test ./internal/gtfs/... -run TestSimulatePositions -v
```
Expected: all 3 tests PASS.

**Step 5: Run full test suite**

```bash
cd api && go test ./... -v
```
Expected: all tests pass.

**Step 6: Commit**

```bash
git add api/internal/gtfs/simulate.go api/internal/gtfs/simulate_test.go
git commit -m "feat(gtfs): add SimulatePositions and StartSimulatedPositionPoller"
```

---

### Task 3: Wire the toggle in `main.go`

**Files:**
- Modify: `api/cmd/server/main.go`

**Step 1: Replace the hardcoded pollers with a key-based toggle**

In `main.go`, replace:

```go
// Metrolinx client for GTFS-RT feeds
client := metrolinx.NewClient(cfg.MetrolinxBaseURL, cfg.MetrolinxAPIKey)

// Realtime cache + background pollers
rtCache := gtfsstore.NewRealtimeCache()
ctx := context.Background()
gtfsstore.StartPositionPoller(ctx, client, static, rtCache, 10*time.Second)
gtfsstore.StartAlertPoller(ctx, client, static, rtCache, 30*time.Second)
gtfsstore.StartTripUpdatePoller(ctx, client, rtCache, 30*time.Second)
```

With:

```go
rtCache := gtfsstore.NewRealtimeCache()
ctx := context.Background()

if cfg.MetrolinxAPIKey == "" {
	slog.Info("METROLINX_API_KEY not set — using simulated vehicle positions")
	gtfsstore.StartSimulatedPositionPoller(ctx, static, rtCache, 10*time.Second)
} else {
	client := metrolinx.NewClient(cfg.MetrolinxBaseURL, cfg.MetrolinxAPIKey)
	gtfsstore.StartPositionPoller(ctx, client, static, rtCache, 10*time.Second)
	gtfsstore.StartAlertPoller(ctx, client, static, rtCache, 30*time.Second)
	gtfsstore.StartTripUpdatePoller(ctx, client, rtCache, 30*time.Second)
}
```

Note: the `metrolinx` import is now only used inside the `else` branch. Go will still compile fine as long as the import is used somewhere, but if the compiler complains about an unused import when the else branch is not reachable statically — it won't, because both branches are runtime. The import is used.

**Step 2: Run `go vet`**

```bash
cd api && go vet ./...
```
Expected: no errors.

**Step 3: Run full test suite**

```bash
cd api && go test ./... -v
```
Expected: all tests pass.

**Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "feat(api): toggle simulated vs real position pollers on METROLINX_API_KEY"
```

**Step 5: Push**

```bash
git push
```

---

## Verification

After all tasks:

1. CI passes (`go test ./... -v` + `go vet ./...` in GitHub Actions).
2. On Railway: with `METROLINX_API_KEY` unset, logs show `simulated positions updated count=N` every 10s. Map shows moving vehicles.
3. With `METROLINX_API_KEY` set: logs show `vehicle positions updated` from real GTFS-RT. No simulation.
