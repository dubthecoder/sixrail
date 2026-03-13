# Bookmarkable Commute URLs — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add URL query params (`?from=X&to=Y&dir=toWork`) so commute routes are bookmarkable and shareable, without touching localStorage.

**Architecture:** Server load function validates URL params and resolves stop names. MyCommute component checks for a `urlTrip` prop and uses it as an override for the localStorage-backed commute store. Toggle in URL mode uses `replaceState` to avoid server round-trips.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, TypeScript, Vitest

**Spec:** `docs/superpowers/specs/2026-03-13-bookmarkable-commute-urls-design.md`

---

## Chunk 1: Server-Side URL Param Validation

### Task 1: Add URL param parsing and validation to +page.server.ts

**Files:**
- Modify: `services/web/src/routes/+page.server.ts`
- Modify: `services/web/src/routes/page.server.test.ts`

#### UrlTrip type

The `urlTrip` object passed in page data:

```typescript
interface UrlTrip {
	fromCode: string;
	fromName: string;
	toCode: string;
	toName: string;
	dir: 'toWork' | 'toHome';
}
```

This is NOT exported as a shared type — it lives implicitly in the page data contract. The component will receive it via `$props()` typed inline.

- [ ] **Step 1: Write failing tests for URL param parsing**

Add these test cases to `services/web/src/routes/page.server.test.ts`:

```typescript
it('returns urlTrip when valid from/to/dir params are provided', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' },
		{ code: 'OASH', id: '2', name: 'Oakville' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/?from=UN&to=OASH&dir=toWork');
	const result = await load({ url } as any);
	expect(result.urlTrip).toEqual({
		fromCode: 'UN',
		fromName: 'Union',
		toCode: 'OASH',
		toName: 'Oakville',
		dir: 'toWork'
	});
});

it('returns urlTrip null when from param is missing', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/?to=UN&dir=toWork');
	const result = await load({ url } as any);
	expect(result.urlTrip).toBeNull();
});

it('returns urlTrip null when stop code is invalid', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/?from=FAKE&to=UN&dir=toWork');
	const result = await load({ url } as any);
	expect(result.urlTrip).toBeNull();
});

it('returns urlTrip null when to stop code is invalid', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/?from=UN&to=FAKE&dir=toWork');
	const result = await load({ url } as any);
	expect(result.urlTrip).toBeNull();
});

it('returns urlTrip null when dir is invalid', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' },
		{ code: 'OASH', id: '2', name: 'Oakville' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/?from=UN&to=OASH&dir=invalid');
	const result = await load({ url } as any);
	expect(result.urlTrip).toBeNull();
});

it('returns urlTrip null when no URL params (bare /)', async () => {
	vi.mocked(getAllStops).mockResolvedValue([
		{ code: 'UN', id: '1', name: 'Union' }
	]);
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/');
	const result = await load({ url } as any);
	expect(result.urlTrip).toBeNull();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/web && npx vitest run src/routes/page.server.test.ts`
Expected: FAIL — `load()` does not accept `{ url }` param and does not return `urlTrip`

- [ ] **Step 3: Implement URL param parsing in +page.server.ts**

Update `services/web/src/routes/+page.server.ts`:

```typescript
import { error } from '@sveltejs/kit';

import { getAllStops, getAlerts } from '$lib/api';

export async function load({ url }: { url: URL }) {
	try {
		const [stops, alerts] = await Promise.all([getAllStops(), getAlerts()]);

		if (!Array.isArray(stops) || !Array.isArray(alerts)) {
			throw error(502, 'Invalid response from departures-api');
		}

		// Parse URL trip params
		const from = url.searchParams.get('from');
		const to = url.searchParams.get('to');
		const dir = url.searchParams.get('dir');

		let urlTrip: {
			fromCode: string;
			fromName: string;
			toCode: string;
			toName: string;
			dir: 'toWork' | 'toHome';
		} | null = null;

		if (from && to && (dir === 'toWork' || dir === 'toHome')) {
			const fromStop = stops.find((s) => s.code === from);
			const toStop = stops.find((s) => s.code === to);
			if (fromStop && toStop) {
				urlTrip = {
					fromCode: fromStop.code,
					fromName: fromStop.name,
					toCode: toStop.code,
					toName: toStop.name,
					dir
				};
			}
		}

		return {
			stops,
			alerts,
			urlTrip
		};
	} catch (err) {
		if (err instanceof Object && 'status' in err) throw err;
		console.error('[SSR] Failed to load homepage data:', err);
		throw error(503, 'Unable to load homepage data');
	}
}
```

- [ ] **Step 4: Fix existing test to pass url param**

The existing `load()` call in the first test needs to pass a `url` now:

```typescript
it('returns SSR data when both backend calls succeed', async () => {
	vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
	vi.mocked(getAlerts).mockResolvedValue([{ headline: 'Notice', description: 'Normal service' }]);

	const url = new URL('http://localhost/');
	await expect(load({ url } as any)).resolves.toEqual({
		stops: [{ code: 'UN', id: '1', name: 'Union' }],
		alerts: [{ headline: 'Notice', description: 'Normal service' }],
		urlTrip: null
	});
});

it('throws a 503 when homepage data cannot be loaded', async () => {
	vi.mocked(getAllStops).mockRejectedValue(new Error('backend unavailable'));
	vi.mocked(getAlerts).mockResolvedValue([]);

	const url = new URL('http://localhost/');
	await expect(load({ url } as any)).rejects.toMatchObject({ status: 503 });
});
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd services/web && npx vitest run src/routes/page.server.test.ts`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add services/web/src/routes/+page.server.ts services/web/src/routes/page.server.test.ts
git commit -m "feat(web): parse and validate URL trip params in homepage server load"
```

---

## Chunk 2: MyCommute URL Mode

### Task 2: Wire urlTrip through +page.svelte to MyCommute

**Files:**
- Modify: `services/web/src/routes/+page.svelte`
- Modify: `services/web/src/lib/components/MyCommute.svelte`

- [ ] **Step 1: Pass urlTrip prop in +page.svelte and update meta tags**

Update `services/web/src/routes/+page.svelte`:

```svelte
<script lang="ts">
	import MyCommute from '$lib/components/MyCommute.svelte';
	let { data } = $props();
</script>

<svelte:head>
	{#if data.urlTrip}
		<title>{data.urlTrip.fromName} → {data.urlTrip.toName} — Rail Six</title>
		<meta
			property="og:title"
			content="{data.urlTrip.fromName} → {data.urlTrip.toName} — Rail Six"
		/>
		<meta
			property="og:url"
			content="https://railsix.com/?from={data.urlTrip.fromCode}&to={data.urlTrip.toCode}&dir={data.urlTrip.dir}"
		/>
	{:else}
		<title>Rail Six</title>
		<meta
			property="og:title"
			content="Rail Six — GO Train Schedule & Real-Time Toronto Commute Tracker"
		/>
		<meta property="og:url" content="https://railsix.com/" />
	{/if}
	<meta
		name="description"
		content="Track your GO Train commute in real time. Live departure times, delays, and platform info for all GO Transit stations across the Greater Toronto Area."
	/>
	<meta
		name="keywords"
		content="GO Train, GO Transit, Toronto train schedule, GO Train times, GTA commute tracker, real-time train tracker, GO Transit departures, Union Station GO, Toronto commuter rail"
	/>
	<meta name="author" content="Teclara Technologies Inc" />
	<meta
		property="og:description"
		content="Track your GO Train commute in real time. Live departure times, delays, and platform info for all GO Transit stations across the Greater Toronto Area."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:image" content="https://railsix.com/train.png" />
	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content="Rail Six — GO Train Schedule & Toronto Commute Tracker" />
	<meta
		name="twitter:description"
		content="Live GO Train departure times, delays, and platform info for your Toronto commute."
	/>
	<meta name="twitter:image" content="https://railsix.com/train.png" />
</svelte:head>

<MyCommute stops={data.stops} alerts={data.alerts} urlTrip={data.urlTrip} />
```

- [ ] **Step 2: Update MyCommute to accept and use urlTrip prop**

This is the core change. In `services/web/src/lib/components/MyCommute.svelte`:

**Props change** — add `urlTrip` to the destructured props:

```typescript
let {
	stops,
	alerts: initialAlerts,
	urlTrip: initialUrlTrip
}: {
	stops: Stop[];
	alerts: Alert[];
	urlTrip: { fromCode: string; fromName: string; toCode: string; toName: string; dir: 'toWork' | 'toHome' } | null;
} = $props();
```

**Add URL mode state** — local `$state` that tracks the current URL trip (mutable for toggle):

```typescript
let urlTrip = $state(initialUrlTrip);
let isUrlMode = $derived(urlTrip !== null);
```

**Override activeDirection and activeTrip** — when in URL mode, these derive from `urlTrip` instead of the commute store:

```typescript
let activeDirection = $derived(
	isUrlMode ? urlTrip!.dir : getActiveDirection(directionOverride, commuteState)
);
let activeTrip = $derived.by(() => {
	if (isUrlMode) {
		return {
			originCode: urlTrip!.fromCode,
			originName: urlTrip!.fromName,
			destinationCode: urlTrip!.toCode,
			destinationName: urlTrip!.toName
		};
	}
	return activeDirection === 'toWork' ? commuteState.toWork : commuteState.toHome;
});
```

**Fix rendering gate** — update the `{#if}` check at line 188:

```svelte
{#if !isUrlMode && !commuteState.toWork && !commuteState.toHome}
	<CommuteSetup {stops} />
{:else}
```

**Fix departure loading $effect** — make it react to activeTrip (which now covers both modes):

```typescript
$effect(() => {
	const trip = activeTrip;
	if (browser && mounted) {
		void loadDepartures(trip);
	}
});
```

**Fix toggle buttons** — in URL mode, both are enabled and toggle updates URL:

Import `replaceState` from SvelteKit (keeps SvelteKit's internal router in sync with the URL, unlike raw `history.replaceState`):

```typescript
import { replaceState } from '$app/navigation';
```

Then the toggle buttons:

```svelte
<button
	class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
	class:bg-amber-400={activeDirection === 'toWork'}
	class:text-black={activeDirection === 'toWork'}
	class:text-gray-400={activeDirection !== 'toWork'}
	onclick={() => {
		if (isUrlMode) {
			const next = {
				fromCode: urlTrip!.toCode,
				fromName: urlTrip!.toName,
				toCode: urlTrip!.fromCode,
				toName: urlTrip!.fromName,
				dir: 'toWork' as const
			};
			urlTrip = next;
			const params = new URLSearchParams({
				from: next.fromCode,
				to: next.toCode,
				dir: 'toWork'
			});
			replaceState(`/?${params}`, {});
		} else {
			directionOverride = 'toWork';
		}
		track('direction-toggle', { direction: 'toWork' });
	}}
	disabled={!isUrlMode && !commuteState.toWork}
>
	To Work
</button>
<button
	class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
	class:bg-amber-400={activeDirection === 'toHome'}
	class:text-black={activeDirection === 'toHome'}
	class:text-gray-400={activeDirection !== 'toHome'}
	onclick={() => {
		if (isUrlMode) {
			const next = {
				fromCode: urlTrip!.toCode,
				fromName: urlTrip!.toName,
				toCode: urlTrip!.fromCode,
				toName: urlTrip!.fromName,
				dir: 'toHome' as const
			};
			urlTrip = next;
			const params = new URLSearchParams({
				from: next.fromCode,
				to: next.toCode,
				dir: 'toHome'
			});
			replaceState(`/?${params}`, {});
		} else {
			directionOverride = 'toHome';
		}
		track('direction-toggle', { direction: 'toHome' });
	}}
	disabled={!isUrlMode && !commuteState.toHome}
>
	To Home
</button>
```

- [ ] **Step 3: Run svelte-check to verify no type errors**

Run: `cd services/web && npm run check`
Expected: PASS — no type errors

- [ ] **Step 4: Run all existing tests to verify no regressions**

Run: `cd services/web && npx vitest run`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add services/web/src/routes/+page.svelte services/web/src/lib/components/MyCommute.svelte
git commit -m "feat(web): bookmarkable commute URLs with toggle and meta tags"
```

---

## Chunk 3: Validation & Lint

### Task 3: Final validation

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `cd services/web && npm run test:ci`
Expected: All tests PASS

- [ ] **Step 2: Run type checking**

Run: `cd services/web && npm run check`
Expected: PASS

- [ ] **Step 3: Run lint and format**

Run: `cd services/web && npm run format && npm run lint`
Expected: PASS (format may modify files — stage and commit if so)

- [ ] **Step 4: Run build**

Run: `cd services/web && npm run build`
Expected: PASS — production build succeeds

- [ ] **Step 5: Commit formatting changes (if any)**

```bash
git add -A services/web/
git commit -m "style(web): format bookmarkable commute URLs code"
```
