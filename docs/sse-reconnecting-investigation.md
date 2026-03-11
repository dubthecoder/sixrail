# SSE "LIVE DATA RECONNECTING" Banner Investigation

## Problem
User sees "LIVE DATA RECONNECTING..." banner persistently in the web app.

## Changes Made (all committed & pushed to main)

### 1. Client-side reconnection (`services/web/src/lib/sse.ts`)
- Added manual reconnection when EventSource enters CLOSED state (server returns non-200)
- Retries every 5 seconds via `scheduleReconnect()`
- `disconnectSSE()` properly cancels pending reconnect timers

### 2. SSE proxy route (`services/web/src/routes/api/sse/+server.ts`)
- Removed `Connection: keep-alive` header (invalid in HTTP/2, may cause Railway proxy issues)
- Added `X-Accel-Buffering: no` header (tells reverse proxies not to buffer SSE stream)
- Added try-catch around upstream fetch for cleaner 502 responses
- **IMPORTANT:** Kept simple `upstream.body` passthrough — custom ReadableStream and `request.signal` forwarding both broke SSE entirely (no connections reached sse-push)

### 3. Rate limiter fix (`services/web/src/hooks.server.ts` + `services/web/src/lib/server/rate-limit.ts`)
- `SSE_MAX_PER_IP`: 3 → 5 (handles race condition: browser reconnects before old connection's `request.signal` fires abort to call `closeSSE`, temporarily double-counting)
- `SSE_TTL_SECONDS`: 3600 → 120 (leaked Redis counters self-heal in 2 min instead of 1 hour)

### 4. CLAUDE.md updates
- Added pre-PR checklist, healthcheck paths, commit conventions, missing env vars, rate-limit/health infrastructure docs

## Key Findings

### All backend services healthy
- realtime-poller: polling every 30s, getting data
- sse-push: connected to NATS, subscribed to all 5 subjects, sends keepalives every 15s
- departures-api: serving requests fine
- gtfs-static: loaded GTFS data
- NATS & Redis: both running

### curl test confirmed SSE proxy works
- `curl -s -N -H "Origin: https://railsix.com" -H "Referer: https://railsix.com/" -H "sec-fetch-site: same-origin" "https://railsix.com/api/sse" --max-time 45`
- Keepalives and full SSE events (alerts, trip-updates, union-departures) flow through
- Connection survived full 45 seconds without dropping

### Things that BREAK SSE (do not attempt again)
- Passing `request.signal` directly to upstream `fetch()` — aborts connection before it establishes
- Using `AbortController` + `request.signal.addEventListener('abort', ...)` — same result
- Wrapping upstream body in custom `ReadableStream` with pull-based reading — no connections reach sse-push
- TransformStream wrapping (per comment in hooks.server.ts) — breaks SSE stream

### Rate limiter race condition was blocking reconnections
- Railway proxy kills SSE after ~60s
- Browser auto-reconnects in ~3s, calling `openSSE(ip)` (+1 counter)
- Old connection's `request.signal` abort hasn't fired yet, so `closeSSE` hasn't run (-1)
- Counter temporarily doubles → exceeds old max of 3 → 429 rejection → permanent lockout
- Fix: higher max (5) + shorter TTL (120s)

## Still Unresolved

### Connections drop every ~25-77 seconds in sse-push logs
- Need a **120-second curl test** to determine if this is Railway's proxy timeout or user page navigations
- `data-sveltekit-reload` on nav links causes full page reloads, killing EventSource each time — this could explain the drops
- If curl survives 120s, drops are just page navigations and problem is likely solved
- If curl drops at ~60s, Railway's proxy idle timeout is the cause and needs Railway-side config

### Next step
Run: `curl -s -N -H "Origin: https://railsix.com" -H "Referer: https://railsix.com/" -H "sec-fetch-site: same-origin" "https://railsix.com/api/sse" --max-time 120 2>&1 | grep -c keepalive`
- If ~7-8 keepalives (one every 15s for 120s): connection is stable, problem solved
- If fewer / curl exits early: Railway proxy is killing it

### If Railway proxy IS the problem
- Check if Railway supports configurable proxy timeout in `railway.toml` (e.g., `deploy.requestTimeout`)
- Or expose sse-push directly with CORS instead of proxying through SvelteKit
- The sse-push service sends ALL 5 event types but the client only uses `alerts` and `union-departures` — filtering at the proxy or adding a query param to sse-push could reduce bandwidth
