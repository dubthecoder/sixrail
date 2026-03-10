# Production Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all critical, high, and medium issues from the production readiness review — timezone safety, error visibility, cache staleness, lock correctness, and operational improvements.

**Architecture:** Three workstreams: (1) Go API backend fixes for safety, correctness, and observability, (2) SvelteKit frontend error propagation so users see errors instead of empty screens, (3) operational/CI improvements. Each task is independently testable and committable.

**Tech Stack:** Go stdlib, SvelteKit 2 / Svelte 5, Tailwind CSS 4

---

## Workstream 1: Go API Safety & Correctness

### Task 1: Make timezone loading fatal instead of silent UTC fallback

**Files:**
- Modify: `api/internal/gtfs/departures.go:22-25`
- Test: `api/internal/gtfs/departures_test.go`

**Step 1: Write the failing test**

In `api/internal/gtfs/departures_test.go`, add a test that verifies `GetDepartures` uses Toronto timezone correctly. The existing tests should already cover this — verify they exist and pass.

Run: `cd api && go test ./internal/gtfs/ -run TestGetDepartures -v`

**Step 2: Replace UTC fallback with panic**

In `api/internal/gtfs/departures.go`, replace lines 22-25:

```go
loc, err := time.LoadLocation(torontoTZ)
if err != nil {
    loc = time.UTC
}
```

With:

```go
loc, err := time.LoadLocation(torontoTZ)
if err != nil {
    // This should never happen: main.go imports _ "time/tzdata" which embeds
    // the timezone database. If this fails, departure times will be wrong by
    // 4-5 hours — panic rather than serve silently wrong data.
    panic("failed to load America/Toronto timezone: " + err.Error())
}
```

**Step 3: Run tests to verify nothing breaks**

Run: `cd api && go test ./... -v -race`
Expected: All PASS (tzdata is embedded via `_ "time/tzdata"` in main.go and tests should have it available)

**Step 4: Commit**

```bash
git add api/internal/gtfs/departures.go
git commit -m "fix: panic on timezone load failure instead of silent UTC fallback"
```

---

### Task 2: Add staleness tracking to RealtimeCache

**Files:**
- Modify: `api/internal/gtfs/realtime.go:141-160` (RealtimeCache struct + constructor)
- Modify: `api/internal/gtfs/realtime.go:162-240` (Set/Get methods)
- Modify: `api/internal/gtfs/realtime.go:503-620` (poller fetch functions)
- Test: `api/internal/gtfs/realtime_test.go`

**Step 1: Add staleness fields to RealtimeCache**

Add to the `RealtimeCache` struct (after line 149):

```go
// Staleness tracking — each cache type tracks when it was last successfully updated.
alertsUpdatedAt       time.Time
tripUpdatesUpdatedAt  time.Time
serviceGlanceUpdatedAt time.Time
cancelledTripsUpdatedAt time.Time
unionDepsUpdatedAt    time.Time
```

**Step 2: Update Set methods to record timestamps**

In `SetAlerts` (line 162), add after `rc.alerts = alerts`:
```go
rc.alertsUpdatedAt = time.Now()
```

Do the same for:
- `SetTripUpdates`: add `rc.tripUpdatesUpdatedAt = time.Now()`
- `SetServiceGlance`: add `rc.serviceGlanceUpdatedAt = time.Now()`
- `SetCancelledTrips`: add `rc.cancelledTripsUpdatedAt = time.Now()`
- `SetUnionDepartures`: add `rc.unionDepsUpdatedAt = time.Now()`

**Step 3: Add public staleness check method**

```go
const maxCacheAge = 5 * time.Minute

// CacheStatus returns the age of each cache type. A zero time means never updated.
type CacheStatus struct {
    AlertsAge       time.Duration `json:"alertsAge"`
    TripUpdatesAge  time.Duration `json:"tripUpdatesAge"`
    ServiceGlanceAge time.Duration `json:"serviceGlanceAge"`
    UnionDepsAge    time.Duration `json:"unionDepsAge"`
    Stale           bool          `json:"stale"`
}

func (rc *RealtimeCache) Status() CacheStatus {
    rc.mu.RLock()
    defer rc.mu.RUnlock()
    now := time.Now()
    s := CacheStatus{}
    if !rc.alertsUpdatedAt.IsZero() {
        s.AlertsAge = now.Sub(rc.alertsUpdatedAt)
    }
    if !rc.tripUpdatesUpdatedAt.IsZero() {
        s.TripUpdatesAge = now.Sub(rc.tripUpdatesUpdatedAt)
    }
    if !rc.serviceGlanceUpdatedAt.IsZero() {
        s.ServiceGlanceAge = now.Sub(rc.serviceGlanceUpdatedAt)
    }
    if !rc.unionDepsUpdatedAt.IsZero() {
        s.UnionDepsAge = now.Sub(rc.unionDepsUpdatedAt)
    }
    // Stale if any cache that has been populated is older than maxCacheAge
    s.Stale = (s.AlertsAge > maxCacheAge && !rc.alertsUpdatedAt.IsZero()) ||
        (s.TripUpdatesAge > maxCacheAge && !rc.tripUpdatesUpdatedAt.IsZero()) ||
        (s.ServiceGlanceAge > maxCacheAge && !rc.serviceGlanceUpdatedAt.IsZero()) ||
        (s.UnionDepsAge > maxCacheAge && !rc.unionDepsUpdatedAt.IsZero())
    return s
}
```

**Step 4: Clear stale data in pollers after consecutive failures**

In each `fetchAndCache*` function, when an error occurs, check if the cache is beyond `maxCacheAge` and clear it. For example in `fetchAndCacheAlerts`:

```go
func fetchAndCacheAlerts(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache) {
    data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/Alerts")
    if err != nil {
        slog.Error("fetching alerts", "error", err)
        cache.clearStaleAlerts()
        return
    }
    // ... rest unchanged
}
```

Add to RealtimeCache:

```go
func (rc *RealtimeCache) clearStaleAlerts() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    if !rc.alertsUpdatedAt.IsZero() && time.Since(rc.alertsUpdatedAt) > maxCacheAge {
        slog.Warn("clearing stale alerts cache", "age", time.Since(rc.alertsUpdatedAt))
        rc.alerts = nil
        rc.alertsUpdatedAt = time.Time{}
    }
}

func (rc *RealtimeCache) clearStaleTripUpdates() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    if !rc.tripUpdatesUpdatedAt.IsZero() && time.Since(rc.tripUpdatesUpdatedAt) > maxCacheAge {
        slog.Warn("clearing stale trip updates cache", "age", time.Since(rc.tripUpdatesUpdatedAt))
        rc.tripUpdates = make(map[string]RawTripUpdate)
        rc.tripUpdatesUpdatedAt = time.Time{}
    }
}

func (rc *RealtimeCache) clearStaleServiceGlance() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    if !rc.serviceGlanceUpdatedAt.IsZero() && time.Since(rc.serviceGlanceUpdatedAt) > maxCacheAge {
        slog.Warn("clearing stale service glance cache", "age", time.Since(rc.serviceGlanceUpdatedAt))
        rc.serviceGlance = make(map[string]models.ServiceGlanceEntry)
        rc.serviceGlanceUpdatedAt = time.Time{}
    }
}

func (rc *RealtimeCache) clearStaleUnionDepartures() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    if !rc.unionDepsUpdatedAt.IsZero() && time.Since(rc.unionDepsUpdatedAt) > maxCacheAge {
        slog.Warn("clearing stale union departures cache", "age", time.Since(rc.unionDepsUpdatedAt))
        rc.unionDepartures = nil
        rc.unionDepsUpdatedAt = time.Time{}
    }
}

func (rc *RealtimeCache) clearStaleExceptions() {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    if !rc.cancelledTripsUpdatedAt.IsZero() && time.Since(rc.cancelledTripsUpdatedAt) > maxCacheAge {
        slog.Warn("clearing stale exceptions cache", "age", time.Since(rc.cancelledTripsUpdatedAt))
        rc.cancelledTrips = make(map[string]bool)
        rc.cancelledTripsUpdatedAt = time.Time{}
    }
}
```

Call the corresponding `clearStale*` in each poller's error path.

**Step 5: Run tests**

Run: `cd api && go test ./... -v -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add api/internal/gtfs/realtime.go
git commit -m "feat: add staleness tracking to RealtimeCache, clear stale data after 5min"
```

---

### Task 3: Fix Health endpoint to return 503 during startup

**Files:**
- Modify: `api/internal/handlers/handlers.go:28-35`

**Step 1: Fix Health handler**

Replace the `Health` function:

```go
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if !h.static.Ready() {
        writeJSON(w, http.StatusServiceUnavailable,
            []byte(`{"status":"starting","reason":"GTFS static data loading"}`))
        return
    }
    writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
}
```

Also fix `Ready` handler line 43 — `w.Write` should use `writeJSON` for consistency:

```go
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if !h.static.Ready() {
        writeJSON(w, http.StatusServiceUnavailable,
            []byte(`{"status":"starting","reason":"GTFS static data loading"}`))
        return
    }
    writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
}
```

**Step 2: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/handlers/handlers.go
git commit -m "fix: Health endpoint returns 503 during startup instead of 200"
```

---

### Task 4: Fix Fares handler to return proper error status codes

**Files:**
- Modify: `api/internal/handlers/handlers.go:344-358`

**Step 1: Return 503 when mx is nil, 502 when upstream fails**

Replace lines 344-358:

```go
if h.mx == nil {
    jsonError(w, "fare data unavailable", http.StatusServiceUnavailable)
    return
}

var fares []models.FareInfo
if cached, ok := h.rt.GetFares(fromCode, toCode); ok {
    fares = cached
} else {
    fetched, err := h.mx.GetFares(r.Context(), fromCode, toCode)
    if err != nil {
        slog.Warn("fares fetch failed", "from", fromCode, "to", toCode, "error", err)
        jsonError(w, "unable to fetch fares", http.StatusBadGateway)
        return
    }
    fares = fetched
    h.rt.SetFares(fromCode, toCode, fetched)
}
```

**Step 2: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/handlers/handlers.go
git commit -m "fix: Fares handler returns proper error status codes instead of 200 with empty array"
```

---

### Task 5: Log NextService fetch errors

**Files:**
- Modify: `api/internal/handlers/handlers.go:193-197`

**Step 1: Add error logging**

Replace lines 193-197:

```go
if !ok {
    if fetched, err := h.mx.GetNextService(r.Context(), stopCode); err == nil {
        nsLines = fetched
        h.rt.SetNextService(stopCode, fetched)
    } else {
        slog.Warn("NextService fetch failed", "stopCode", stopCode, "error", err)
    }
}
```

**Step 2: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/handlers/handlers.go
git commit -m "fix: log NextService fetch errors instead of silently swallowing"
```

---

### Task 6: Fix RemainingStopNames lock scope

**Files:**
- Modify: `api/internal/gtfs/static.go:258-284`

**Step 1: Hold single RLock for entire function**

Replace the `RemainingStopNames` function:

```go
func (s *StaticStore) RemainingStopNames(tripID string, departureStopIDs []string) []string {
    s.mu.RLock()
    defer s.mu.RUnlock()

    trip, ok := s.tripIndex[tripID]
    if !ok {
        return nil
    }

    depSet := make(map[string]bool, len(departureStopIDs))
    for _, id := range departureStopIDs {
        depSet[id] = true
    }

    found := false
    var names []string
    for _, ts := range trip.Stops {
        if !found {
            if depSet[ts.StopID] {
                found = true
            }
            continue
        }
        name := s.stopNames[ts.StopID]
        if name != "" {
            names = append(names, name)
        }
    }
    return names
}
```

**Step 2: Run tests with race detector**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/gtfs/static.go
git commit -m "fix: hold single RLock in RemainingStopNames to prevent data race window"
```

---

### Task 7: Wire context into downloadURL

**Files:**
- Modify: `api/cmd/server/main.go:110-130` (downloadURL signature and body)
- Modify: `api/cmd/server/main.go:162` (call site in loadGTFSIntoStore)
- Modify: `api/cmd/server/main.go:217` (call site in refreshLoop)

**Step 1: Add context parameter and use NewRequestWithContext**

Update `downloadURL`:

```go
func downloadURL(ctx context.Context, rawURL string) ([]byte, error) {
    parsed, err := neturl.Parse(rawURL)
    if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
        return nil, fmt.Errorf("invalid or non-HTTP(S) URL: %s", rawURL)
    }
    if !allowedGTFSHosts[parsed.Hostname()] {
        return nil, fmt.Errorf("host %q not in GTFS allowlist", parsed.Hostname())
    }
    client := &http.Client{Timeout: 60 * time.Second}
    cleanURL := parsed.String()
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURL, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request for %s: %w", rawURL, err)
    }
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("downloading %s: %w", rawURL, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, rawURL)
    }
    const maxBytes = 50 * 1024 * 1024 // 50 MB
    data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
    if err != nil {
        return nil, fmt.Errorf("reading response from %s: %w", rawURL, err)
    }
    if int64(len(data)) >= maxBytes {
        return nil, fmt.Errorf("response from %s exceeds %d byte limit", rawURL, maxBytes)
    }
    return data, nil
}
```

**Step 2: Update call sites**

In `loadGTFSIntoStore` (line 162): change `downloadURL(url)` to `downloadURL(ctx, url)`
In `refreshLoop` (line 217): change `downloadURL(url)` to `downloadURL(ctx, url)` — need to pass `ctx` into `refreshLoop` (already available via parameter).

Note: `refreshLoop` already receives `ctx` via `manageGTFS`. Just thread it through to `downloadURL`.

**Step 3: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "fix: wire context into downloadURL for graceful shutdown, add size limit check"
```

---

### Task 8: Check scanner error in config.go

**Files:**
- Modify: `api/internal/config/config.go:61-93`

**Step 1: Add scanner.Err() check**

After the `for scanner.Scan()` loop (after line 92), add:

```go
if err := scanner.Err(); err != nil {
    // Partial .env load — log but don't fail since env vars may also come from OS
    // Using fmt since slog may not be configured yet during config loading
    fmt.Fprintf(os.Stderr, "warning: error reading %s: %v\n", path, err)
}
```

**Step 2: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/config/config.go
git commit -m "fix: check scanner.Err() after reading .env files"
```

---

## Workstream 2: Frontend Error Propagation

### Task 9: Add error logging to SvelteKit proxy routes

**Files:**
- Modify: `web/src/routes/api/alerts/+server.ts`
- Modify: `web/src/routes/api/stops/+server.ts`
- Modify: `web/src/routes/api/union-departures/+server.ts`
- Modify: `web/src/routes/api/network-health/+server.ts`
- Modify: `web/src/routes/api/departures/[stopCode]/+server.ts`
- Modify: `web/src/routes/api/fares/[from]/[to]/+server.ts`

**Step 1: Add error logging and error payload to all proxy routes**

Update each proxy route to bind the error and log it. Pattern for simple routes (alerts, stops, union-departures, network-health):

```typescript
// Example: alerts/+server.ts
import { json } from '@sveltejs/kit';
import { getAlerts } from '$lib/api';

export async function GET() {
    try {
        const alerts = await getAlerts();
        return json(alerts);
    } catch (err) {
        console.error('[proxy] /api/alerts failed:', err);
        return json({ error: 'upstream unavailable' }, { status: 502 });
    }
}
```

Apply the same pattern to all 6 proxy routes:
- `stops/+server.ts`: `console.error('[proxy] /api/stops failed:', err);`
- `union-departures/+server.ts`: `console.error('[proxy] /api/union-departures failed:', err);`
- `network-health/+server.ts`: `console.error('[proxy] /api/network-health failed:', err);`
- `departures/[stopCode]/+server.ts`: `console.error('[proxy] /api/departures failed:', err);`
- `fares/[from]/[to]/+server.ts`: `console.error('[proxy] /api/fares failed:', err);`

**Important:** These now return `{ error: '...' }` (an object) instead of `[]` (an array) on error. The client-side code in `api-client.ts` (Task 10) will need to handle this.

**Step 2: Add dest validation to departures proxy**

In `departures/[stopCode]/+server.ts`, add before the try block:

```typescript
const dest = url.searchParams.get('dest') ?? undefined;
if (dest !== undefined && !stopCodeRe.test(dest)) {
    return json({ error: 'invalid dest code' }, { status: 400 });
}
```

**Step 3: Run checks**

Run: `cd web && npm run check`

**Step 4: Commit**

```bash
git add web/src/routes/api/
git commit -m "fix: log errors and return error payloads in SvelteKit proxy routes"
```

---

### Task 10: Make api-client.ts propagate errors instead of swallowing them

**Files:**
- Modify: `web/src/lib/api-client.ts`

**Step 1: Replace silent `return []` with thrown errors**

```typescript
import type { Alert } from './api';

export class ApiError extends Error {
    constructor(
        public status: number,
        message: string
    ) {
        super(message);
        this.name = 'ApiError';
    }
}

export async function fetchAlerts(): Promise<Alert[]> {
    const res = await fetch('/api/alerts', { signal: AbortSignal.timeout(10000) });
    if (!res.ok) throw new ApiError(res.status, `alerts: ${res.status}`);
    return res.json();
}

export type Departure = {
    line: string;
    lineName?: string;
    scheduledTime: string;
    actualTime?: string;
    arrivalTime?: string;
    status: string;
    platform?: string;
    delayMinutes?: number;
    stops?: string[];
    cars?: string;
    isInMotion?: boolean;
    isCancelled?: boolean;
    isExpress?: boolean;
    alert?: string;
    routeType?: number;
};

export async function fetchDepartures(stopCode: string, destCode?: string): Promise<Departure[]> {
    const url = destCode
        ? `/api/departures/${encodeURIComponent(stopCode)}?dest=${encodeURIComponent(destCode)}`
        : `/api/departures/${encodeURIComponent(stopCode)}`;
    const res = await fetch(url, { signal: AbortSignal.timeout(10000) });
    if (!res.ok) throw new ApiError(res.status, `departures: ${res.status}`);
    return res.json();
}

export type UnionDeparture = {
    service: string;
    platform: string;
    time: string;
    info: string;
    stops: string[];
    cars?: string;
    isInMotion?: boolean;
    isCancelled?: boolean;
    alert?: string;
};

export async function fetchUnionDepartures(): Promise<UnionDeparture[]> {
    const res = await fetch('/api/union-departures', { signal: AbortSignal.timeout(10000) });
    if (!res.ok) throw new ApiError(res.status, `union-departures: ${res.status}`);
    return res.json();
}

export type NetworkLine = {
    lineCode: string;
    lineName: string;
    activeTrips: number;
};

export async function fetchNetworkHealth(): Promise<NetworkLine[]> {
    const res = await fetch('/api/network-health', { signal: AbortSignal.timeout(10000) });
    if (!res.ok) throw new ApiError(res.status, `network-health: ${res.status}`);
    return res.json();
}

export type FareInfo = {
    category: string;
    fareType: string;
    amount: number;
};

export async function fetchFares(from: string, to: string): Promise<FareInfo[]> {
    const res = await fetch(
        `/api/fares/${encodeURIComponent(from)}/${encodeURIComponent(to)}`,
        { signal: AbortSignal.timeout(10000) }
    );
    if (!res.ok) throw new ApiError(res.status, `fares: ${res.status}`);
    return res.json();
}
```

**Step 2: Run checks**

Run: `cd web && npm run check`
Expected: Type errors in components that call these functions — those are fixed in Tasks 11 and 12.

**Step 3: Commit**

```bash
git add web/src/lib/api-client.ts
git commit -m "fix: api-client throws ApiError instead of silently returning empty arrays"
```

---

### Task 11: Handle errors gracefully in MyCommute component

**Files:**
- Modify: `web/src/lib/components/MyCommute.svelte:36-84`

**Step 1: Add error state and update error handling**

Add after line 38 (`let showSettings = $state(false);`):

```typescript
let fetchError = $state(false);
```

Update `loadDepartures` (lines 66-76):

```typescript
async function loadDepartures(trip = activeTrip) {
    if (!trip) {
        departures = [];
        fetchError = false;
        return;
    }
    try {
        departures = await fetchDepartures(trip.originCode, trip.destinationCode);
        fetchError = false;
    } catch (err) {
        // Keep existing departures on error — don't wipe the screen
        fetchError = true;
        console.error('Failed to load departures:', err);
    }
}
```

Update `loadAlerts` (lines 78-84):

```typescript
async function loadAlerts() {
    try {
        alerts = await fetchAlerts();
    } catch (err) {
        console.error('Failed to load alerts:', err);
        // keep existing alerts on error
    }
}
```

**Step 2: Add error indicator to the template**

After the `<AlertBanner>` component (line 196), add:

```svelte
{#if fetchError}
    <div class="text-amber-400/70 text-xs text-center py-1 tracking-wider uppercase">
        Unable to refresh — showing last known data
    </div>
{/if}
```

**Step 3: Run checks**

Run: `cd web && npm run check`

**Step 4: Commit**

```bash
git add web/src/lib/components/MyCommute.svelte
git commit -m "fix: MyCommute keeps existing data on error, shows stale-data indicator"
```

---

### Task 12: Handle errors gracefully in departure board

**Files:**
- Modify: `web/src/routes/departures/+page.svelte:69-90`

**Step 1: Add error state**

After line 69 (`let allGtfsDepartures = $state<Departure[]>([]);`), add:

```typescript
let fetchError = $state(false);
```

**Step 2: Update loadDepartures**

Replace the catch block (lines 87-89):

```typescript
async function loadDepartures() {
    if (loadController) loadController.abort();
    const controller = new AbortController();
    loadController = controller;

    const stopCode = selectedStation || 'UN';
    try {
        const deps = await fetchDepartures(stopCode);
        if (controller.signal.aborted) return;
        allGtfsDepartures = sortByScheduledTime(deps);
        fetchError = false;
    } catch (err) {
        if (controller.signal.aborted) return;
        // Keep existing departures on error
        fetchError = true;
        console.error('Failed to load departures:', err);
    }
}
```

**Step 3: Update loadNetworkHealth similarly**

```typescript
async function loadNetworkHealth() {
    try {
        networkHealth = await fetchNetworkHealth();
    } catch (err) {
        console.error('Failed to load network health:', err);
        // keep existing data on error
    }
}
```

**Step 4: Add error indicator to template**

After the `<div class="col-headers ...">` section (around line 349), add:

```svelte
{#if fetchError}
    <div class="text-amber-400/70 text-center py-1 tracking-wider uppercase" style="font-size: 0.55em;">
        Unable to refresh — showing last known data
    </div>
{/if}
```

**Step 5: Run checks**

Run: `cd web && npm run check`

**Step 6: Commit**

```bash
git add web/src/routes/departures/+page.svelte
git commit -m "fix: departure board keeps data on fetch error, shows stale-data indicator"
```

---

### Task 13: Add error logging to SSR page loads

**Files:**
- Modify: `web/src/routes/+page.server.ts`
- Modify: `web/src/routes/departures/+page.server.ts`

**Step 1: Log errors in SSR catch blocks**

Update `web/src/routes/+page.server.ts`:

```typescript
import { getAllStops, getAlerts } from '$lib/api';

export async function load() {
    const [stops, alerts] = await Promise.all([
        getAllStops().catch((err) => {
            console.error('[SSR] Failed to load stops:', err);
            return [];
        }),
        getAlerts().catch((err) => {
            console.error('[SSR] Failed to load alerts:', err);
            return [];
        })
    ]);
    return {
        stops: Array.isArray(stops) ? stops : [],
        alerts: Array.isArray(alerts) ? alerts : []
    };
}
```

Update `web/src/routes/departures/+page.server.ts`:

```typescript
import { getAllStops } from '$lib/api';

export async function load() {
    const stops = await getAllStops().catch((err) => {
        console.error('[SSR] Failed to load stops:', err);
        return [];
    });
    return {
        stops: Array.isArray(stops) ? stops : []
    };
}
```

**Step 2: Run checks**

Run: `cd web && npm run check`

**Step 3: Commit**

```bash
git add web/src/routes/+page.server.ts web/src/routes/departures/+page.server.ts
git commit -m "fix: log SSR page load errors instead of silently swallowing"
```

---

## Workstream 3: Operational Improvements

### Task 14: Add cache status to health endpoint

**Files:**
- Modify: `api/internal/handlers/handlers.go:28-44`

**Step 1: Expose cache status in health response**

Update the `Health` handler to include cache status when the service is ready:

```go
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if !h.static.Ready() {
        writeJSON(w, http.StatusServiceUnavailable,
            []byte(`{"status":"starting","reason":"GTFS static data loading"}`))
        return
    }
    status := h.rt.Status()
    resp := struct {
        Status string                     `json:"status"`
        Cache  gtfsstore.CacheStatus      `json:"cache"`
    }{
        Status: "ok",
        Cache:  status,
    }
    if status.Stale {
        resp.Status = "degraded"
    }
    data, err := json.Marshal(resp)
    if err != nil {
        writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
        return
    }
    writeJSON(w, http.StatusOK, data)
}
```

**Step 2: Run tests**

Run: `cd api && go test ./... -v -race`

**Step 3: Commit**

```bash
git add api/internal/handlers/handlers.go api/internal/gtfs/realtime.go
git commit -m "feat: health endpoint reports cache staleness and degraded status"
```

---

### Task 15: Add web health endpoint decoupled from API

**Files:**
- Create: `web/src/routes/health/+server.ts`

**Step 1: Create a simple health endpoint**

```typescript
import { json } from '@sveltejs/kit';

export function GET() {
    return json({ status: 'ok' });
}
```

**Step 2: Update `web/railway.toml`**

Change `healthcheckPath = "/"` to `healthcheckPath = "/health"` so web deploys don't depend on the API being up.

**Step 3: Run checks**

Run: `cd web && npm run check`

**Step 4: Commit**

```bash
git add web/src/routes/health/+server.ts web/railway.toml
git commit -m "feat: add dedicated /health endpoint for web, decouple from API availability"
```

---

### Task 16: Format, lint, and final verification

**Step 1: Format web code**

Run: `cd web && npm run format`

**Step 2: Run all web checks**

Run: `cd web && npm run check && npm run lint`

**Step 3: Run all Go tests**

Run: `cd api && go test ./... -v -race && go vet ./...`

**Step 4: Fix any issues found**

Address any formatting, lint, or test failures.

**Step 5: Commit any formatting changes**

```bash
git add -A
git commit -m "chore: format and lint fixes"
```

---

## Summary of Changes

| Task | Severity Fixed | Area |
|------|---------------|------|
| 1 | CRITICAL | Timezone panic vs silent UTC |
| 2 | CRITICAL/HIGH | Cache staleness tracking + auto-clear |
| 3 | HIGH | Health endpoint 503 during startup |
| 4 | HIGH | Fares proper error status codes |
| 5 | HIGH | NextService error logging |
| 6 | MEDIUM | RemainingStopNames lock scope |
| 7 | MEDIUM | downloadURL context propagation + size check |
| 8 | MEDIUM | Scanner error check in config |
| 9 | CRITICAL | Proxy route error logging + payloads |
| 10 | CRITICAL | api-client throws instead of swallowing |
| 11 | HIGH | MyCommute keeps data on error + indicator |
| 12 | HIGH | Departure board keeps data on error + indicator |
| 13 | CRITICAL | SSR page load error logging |
| 14 | HIGH | Health endpoint cache status |
| 15 | MEDIUM | Web health endpoint decoupled from API |
| 16 | — | Format and lint |
