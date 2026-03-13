# Bookmarkable Commute URLs

## Problem

The commute dashboard at `/` stores route configuration in localStorage only. Users cannot bookmark specific routes, share links, or save separate morning/evening bookmarks.

## URL Scheme

```
/?from={stopCode}&to={stopCode}&dir={toWork|toHome}
```

All three params required for URL mode to activate. Missing or invalid params fall back to localStorage behavior.

**Examples:**
- Morning commute: `/?from=UN&to=OASH&dir=toWork`
- Evening commute: `/?from=OASH&to=UN&dir=toHome`

## Behavior

### URL Params Present

- URL params drive the view — origin, destination, and direction toggle state
- localStorage is NOT read or modified — URL is a temporary view layer
- Page title updates: "Union → Oakville — Rail Six"
- OG meta tags (`og:title`, `og:url`) update to match the URL route for sharing
- Settings gear remains visible — users can still configure their own localStorage commute independently

### Bare `/` (No Params)

- Current behavior unchanged: localStorage + time-of-day auto-switch
- Page title: "Rail Six"

### Toggle Behavior (URL Mode)

When user clicks the inactive direction toggle:
1. Swap `from` and `to` values
2. Flip `dir` (`toWork` ↔ `toHome`)
3. Update URL via client-side `replaceState` (NOT `goto()` — avoids re-running server load)
4. Fetch departures for the new origin/destination pair

Both toggle buttons are always enabled in URL mode since the reverse trip is always derivable by swapping `from`/`to`.

### Validation

- `from` and `to` validated against `stop.code` field (matching how `CommuteTrip.originCode` / `destinationCode` work)
- If either code is invalid or any param is missing → ignore all URL params, fall back to localStorage
- `dir` must be `toWork` or `toHome` — invalid value → fall back

## Implementation

### Files to Modify

1. **`src/routes/+page.server.ts`**
   - Read `from`, `to`, `dir` from `url.searchParams`
   - Validate stop codes against `stop.code` in the stops list
   - Resolve stop names from matched stops
   - Pass validated `urlTrip: { fromCode, fromName, toCode, toName, dir } | null` in page data

2. **`src/lib/components/MyCommute.svelte`**
   - Accept optional `urlTrip` prop
   - **Rendering gate fix:** The `{#if !commuteState.toWork && !commuteState.toHome}` check must also pass through when `urlTrip` is present — otherwise URL mode shows `CommuteSetup` for users with no localStorage
   - If `urlTrip` is present: build a synthetic `activeTrip` from it (with names) instead of reading from commute store
   - **Departure loading fix:** The `$effect` that loads departures must react to the URL-derived trip, not just `commuteState`
   - Toggle in URL mode: use `history.replaceState` + update local `$state` holding the URL trip (no server round-trip)
   - Toggle `track()` call preserved, payload unchanged

3. **`src/routes/+page.svelte`**
   - Pass `urlTrip` from page data to `MyCommute`
   - Update `<title>` and `og:title`/`og:url` meta tags reactively when `urlTrip` is present

### Files NOT Modified

- `src/lib/stores/commute.ts` — URL params bypass the store entirely
- `src/lib/api.ts` / `src/lib/api-client.ts` — no API changes needed
- Backend services — no changes

## Edge Cases

- User has localStorage commute AND opens a URL → URL wins for that view, localStorage preserved
- User opens URL then navigates to bare `/` → back to their localStorage commute
- User with no localStorage opens a URL → works fine, shows the URL route (rendering gate updated)
- User with no localStorage visits bare `/` → shows CommuteSetup as before
- User opens URL and clicks settings gear → can configure their own localStorage commute independently
- Toggle in URL mode → `replaceState` only, no server re-fetch of stops/alerts
