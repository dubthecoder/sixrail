# World-Class Commuter Upgrade Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform Six Rail from a map-first tracker into a world-class commuter PWA with a split-flap departure dashboard, push notifications, and zero-friction commute setup.

**Architecture:** Frontend-only changes. New `/` route becomes the dashboard; existing map page moves to `/map`. New localStorage stores for commute trips and notification preferences. A service worker handles PWA install and local push notifications. No Go API changes required.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, Tailwind CSS 4, Web Push API (browser-native), Service Worker API, Web App Manifest. Existing: `fetchDepartures`, `fetchAlerts` from `api-client.ts`.

---

## Task 1: Move map to `/map` route

**Files:**
- Create: `web/src/routes/map/+page.svelte` (move from `web/src/routes/+page.svelte`)
- Create: `web/src/routes/map/+page.server.ts` (move from `web/src/routes/+page.server.ts`)
- Modify: `web/src/routes/+page.svelte` (replace with dashboard stub)
- Modify: `web/src/routes/+page.server.ts` (replace with dashboard load)

**Step 1: Copy the existing map page to `/map`**

```bash
cp web/src/routes/+page.svelte web/src/routes/map/+page.svelte
cp web/src/routes/+page.server.ts web/src/routes/map/+page.server.ts
```

Note: You'll need to `mkdir -p web/src/routes/map` first.

**Step 2: Replace `web/src/routes/+page.svelte` with a dashboard stub**

Replace the entire file content with:

```svelte
<script lang="ts">
  let { data } = $props();
</script>

<div class="min-h-screen bg-[#111] text-white flex items-center justify-center">
  <p class="text-amber-400 font-mono">Dashboard coming soon</p>
</div>
```

**Step 3: Replace `web/src/routes/+page.server.ts` with a minimal load**

```typescript
export async function load() {
  return {};
}
```

**Step 4: Verify the app still runs**

```bash
cd web && npm run dev
```

Navigate to `http://localhost:5173` — should show stub. Navigate to `http://localhost:5173/map` — should show the full map.

**Step 5: Run type check**

```bash
cd web && npm run check
```

Expected: no errors.

**Step 6: Commit**

```bash
git add web/src/routes/
git commit -m "refactor(web): move map to /map route, stub dashboard at /"
```

---

## Task 2: Add commute and notification stores

**Files:**
- Create: `web/src/lib/stores/commute.ts`

**Step 1: Create the store**

```typescript
// web/src/lib/stores/commute.ts
import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export interface CommuteTrip {
  originCode: string;
  originName: string;
  destinationCode: string;
  destinationName: string;
}

export interface CommuteStore {
  toWork: CommuteTrip | null;
  toHome: CommuteTrip | null;
}

export interface NotificationPrefs {
  enabled: boolean;
  thresholdMinutes: 5 | 10 | 15;
}

function createCommuteStore() {
  const initial: CommuteStore = browser
    ? JSON.parse(localStorage.getItem('commute') || 'null') ?? { toWork: null, toHome: null }
    : { toWork: null, toHome: null };

  const { subscribe, set, update } = writable<CommuteStore>(initial);

  return {
    subscribe,
    setTrip(direction: 'toWork' | 'toHome', trip: CommuteTrip) {
      update((s) => {
        const next = { ...s, [direction]: trip };
        if (browser) localStorage.setItem('commute', JSON.stringify(next));
        return next;
      });
    },
    clear() {
      const empty = { toWork: null, toHome: null };
      if (browser) localStorage.removeItem('commute');
      set(empty);
    }
  };
}

function createNotificationStore() {
  const initial: NotificationPrefs = browser
    ? JSON.parse(localStorage.getItem('notificationPrefs') || 'null') ?? {
        enabled: false,
        thresholdMinutes: 5
      }
    : { enabled: false, thresholdMinutes: 5 };

  const { subscribe, set, update } = writable<NotificationPrefs>(initial);

  return {
    subscribe,
    setEnabled(enabled: boolean) {
      update((s) => {
        const next = { ...s, enabled };
        if (browser) localStorage.setItem('notificationPrefs', JSON.stringify(next));
        return next;
      });
    },
    setThreshold(thresholdMinutes: 5 | 10 | 15) {
      update((s) => {
        const next = { ...s, thresholdMinutes };
        if (browser) localStorage.setItem('notificationPrefs', JSON.stringify(next));
        return next;
      });
    }
  };
}

export const commute = createCommuteStore();
export const notificationPrefs = createNotificationStore();

/** Returns which direction to show based on time of day, respecting manual override */
export function getActiveDirection(override: 'toWork' | 'toHome' | null): 'toWork' | 'toHome' {
  if (override) return override;
  return new Date().getHours() < 12 ? 'toWork' : 'toHome';
}
```

**Step 2: Run type check**

```bash
cd web && npm run check
```

Expected: no errors.

**Step 3: Commit**

```bash
git add web/src/lib/stores/commute.ts
git commit -m "feat(web): add commute and notification preference stores"
```

---

## Task 3: Build `SplitFlapChar` component

This is the core animation primitive — a single character cell that flips when its value changes.

**Files:**
- Create: `web/src/lib/components/SplitFlapChar.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/SplitFlapChar.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';

  let { value = ' ', delay = 0 }: { value: string; delay?: number } = $props();

  let displayValue = $state(value);
  let isFlipping = $state(false);
  let topValue = $state(value);
  let bottomValue = $state(value);

  const CHARS = ' ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789:+-.';

  function getNextChar(current: string): string {
    const idx = CHARS.indexOf(current.toUpperCase());
    return CHARS[(idx + 1) % CHARS.length];
  }

  async function flipTo(target: string) {
    const targetUpper = target.toUpperCase();
    if (displayValue.toUpperCase() === targetUpper) return;

    await new Promise((r) => setTimeout(r, delay));

    let current = displayValue.toUpperCase();
    while (current !== targetUpper) {
      current = getNextChar(current);
      topValue = current;
      isFlipping = true;
      await new Promise((r) => setTimeout(r, 40));
      isFlipping = false;
      bottomValue = current;
      displayValue = current;
      await new Promise((r) => setTimeout(r, 10));
    }
  }

  $effect(() => {
    flipTo(value);
  });
</script>

<span class="split-flap-char" style="--flip-delay: {delay}ms">
  <span class="tile top">{topValue}</span>
  <span class="tile bottom">{bottomValue}</span>
  {#if isFlipping}
    <span class="tile flipping">{topValue}</span>
  {/if}
</span>

<style>
  .split-flap-char {
    position: relative;
    display: inline-flex;
    flex-direction: column;
    width: 1ch;
    height: 1.4em;
    background: #1e1e1e;
    border-radius: 2px;
    overflow: hidden;
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.5);
    margin: 0 1px;
    font-variant-numeric: tabular-nums;
  }

  .tile {
    position: absolute;
    width: 100%;
    height: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    line-height: 2;
  }

  .tile.top {
    top: 0;
    align-items: flex-end;
    background: #1e1e1e;
    border-bottom: 1px solid #000;
  }

  .tile.bottom {
    bottom: 0;
    align-items: flex-start;
    background: #181818;
  }

  .tile.flipping {
    top: 0;
    height: 100%;
    animation: flip 80ms linear forwards;
    transform-origin: center;
    background: #1e1e1e;
    z-index: 2;
  }

  @keyframes flip {
    0% { transform: rotateX(0deg); }
    50% { transform: rotateX(-90deg); }
    100% { transform: rotateX(0deg); opacity: 0; }
  }
</style>
```

**Step 2: Run type check**

```bash
cd web && npm run check
```

Expected: no errors.

**Step 3: Commit**

```bash
git add web/src/lib/components/SplitFlapChar.svelte
git commit -m "feat(web): add SplitFlapChar component with flip animation"
```

---

## Task 4: Build `SplitFlapBoard` component

Renders a full departure board row using `SplitFlapChar` for each character.

**Files:**
- Create: `web/src/lib/components/SplitFlapBoard.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/SplitFlapBoard.svelte -->
<script lang="ts">
  import SplitFlapChar from './SplitFlapChar.svelte';
  import type { Departure } from '$lib/api-client';

  let {
    departures = [],
    maxRows = 3
  }: {
    departures: Departure[];
    maxRows?: number;
  } = $props();

  function padRight(str: string, len: number): string {
    return str.toUpperCase().padEnd(len, ' ').slice(0, len);
  }

  function formatTime(t: string): string {
    // t is like "08:15" already
    return t.slice(0, 5);
  }

  function statusText(d: Departure): string {
    if (d.status === 'CANCELLED') return 'CANCELLED  ';
    if (d.delayMinutes && d.delayMinutes > 0) return `DELAYED +${d.delayMinutes}MIN`;
    return 'ON TIME    ';
  }

  function statusClass(d: Departure): string {
    if (d.status === 'CANCELLED') return 'text-red-500';
    if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
    return 'text-green-400';
  }

  let rows = $derived(departures.slice(0, maxRows));
</script>

<div class="split-flap-board font-mono select-none" role="region" aria-label="Departure board">
  <!-- Header -->
  <div class="board-header">
    <span class="col-time text-gray-500 text-xs uppercase tracking-widest">Time</span>
    <span class="col-route text-gray-500 text-xs uppercase tracking-widest">Route</span>
    <span class="col-platform text-gray-500 text-xs uppercase tracking-widest">Plat</span>
    <span class="col-status text-gray-500 text-xs uppercase tracking-widest">Status</span>
  </div>

  <!-- Rows -->
  {#each rows as dep, i}
    <div class="board-row" class:next-train={i === 0}>
      <!-- Time -->
      <span class="col-time text-amber-400">
        {#each formatTime(dep.scheduledTime).split('') as char, j}
          <SplitFlapChar value={char} delay={j * 30} />
        {/each}
      </span>

      <!-- Route -->
      <span class="col-route text-white">
        {#each padRight(dep.line, 14).split('') as char, j}
          <SplitFlapChar value={char} delay={50 + j * 20} />
        {/each}
      </span>

      <!-- Platform -->
      <span class="col-platform text-white">
        {#each padRight(dep.platform ?? '--', 4).split('') as char, j}
          <SplitFlapChar value={char} delay={100 + j * 20} />
        {/each}
      </span>

      <!-- Status -->
      <span class="col-status {statusClass(dep)}">
        {#each padRight(statusText(dep), 14).split('') as char, j}
          <SplitFlapChar value={char} delay={120 + j * 15} />
        {/each}
      </span>
    </div>
  {/each}

  {#if rows.length === 0}
    <div class="board-empty text-gray-600 font-mono text-sm py-8 text-center">
      NO DEPARTURES FOUND
    </div>
  {/if}
</div>

<style>
  .split-flap-board {
    background: #111;
    border-radius: 8px;
    padding: 12px;
    width: 100%;
    overflow: hidden;
  }

  .board-header,
  .board-row {
    display: grid;
    grid-template-columns: 5ch 15ch 5ch 15ch;
    gap: 8px;
    align-items: center;
    padding: 4px 0;
  }

  .board-header {
    border-bottom: 1px solid #222;
    margin-bottom: 8px;
    padding-bottom: 8px;
  }

  .board-row {
    border-bottom: 1px solid #1a1a1a;
    padding: 6px 0;
    transition: background 0.2s;
  }

  .board-row.next-train {
    background: #1a1600;
    border-radius: 4px;
    padding: 8px 4px;
    font-size: 1.1em;
  }

  .col-time { font-size: 0.95em; }
  .col-route { font-size: 0.85em; }
  .col-platform { font-size: 0.85em; }
  .col-status { font-size: 0.8em; letter-spacing: 0.05em; }

  @media (max-width: 480px) {
    .board-header,
    .board-row {
      grid-template-columns: 5ch 12ch 4ch 12ch;
      gap: 4px;
    }
  }
</style>
```

**Step 2: Run type check**

```bash
cd web && npm run check
```

Expected: no errors.

**Step 3: Commit**

```bash
git add web/src/lib/components/SplitFlapBoard.svelte
git commit -m "feat(web): add SplitFlapBoard departure display component"
```

---

## Task 5: Build `CountdownTimer` component

**Files:**
- Create: `web/src/lib/components/CountdownTimer.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/CountdownTimer.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let { scheduledTime }: { scheduledTime: string } = $props();

  let display = $state('--:--');
  let interval: ReturnType<typeof setInterval>;

  function computeCountdown(scheduled: string): string {
    const now = new Date();
    const [h, m] = scheduled.split(':').map(Number);
    const target = new Date(now);
    target.setHours(h, m, 0, 0);
    if (target < now) target.setDate(target.getDate() + 1); // next day
    const diffMs = target.getTime() - now.getTime();
    if (diffMs < 0) return '00:00';
    const mins = Math.floor(diffMs / 60000);
    const secs = Math.floor((diffMs % 60000) / 1000);
    return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }

  onMount(() => {
    display = computeCountdown(scheduledTime);
    interval = setInterval(() => {
      display = computeCountdown(scheduledTime);
    }, 1000);
  });

  onDestroy(() => clearInterval(interval));

  $effect(() => {
    display = computeCountdown(scheduledTime);
  });
</script>

<div class="countdown" role="timer" aria-label="Time until next departure">
  <span class="label text-gray-500 text-xs uppercase tracking-widest">Next train in</span>
  <span class="time font-mono text-amber-400 tabular-nums">{display}</span>
</div>

<style>
  .countdown {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    background: #1a1a1a;
    border-radius: 8px;
    padding: 12px 24px;
    min-width: 160px;
  }

  .time {
    font-size: 2rem;
    letter-spacing: 0.1em;
  }
</style>
```

**Step 2: Run type check**

```bash
cd web && npm run check
```

**Step 3: Commit**

```bash
git add web/src/lib/components/CountdownTimer.svelte
git commit -m "feat(web): add CountdownTimer component"
```

---

## Task 6: Build `AlertBanner` component

**Files:**
- Create: `web/src/lib/components/AlertBanner.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/AlertBanner.svelte -->
<script lang="ts">
  import type { Alert } from '$lib/api';

  let {
    alerts = [],
    routeNames = []
  }: {
    alerts: Alert[];
    routeNames: string[];
  } = $props();

  let relevant = $derived(
    alerts.filter(
      (a) =>
        !a.routeNames ||
        a.routeNames.length === 0 ||
        a.routeNames.some((r) => routeNames.includes(r))
    )
  );

  let expanded = $state(false);
</script>

{#if relevant.length > 0}
  <button
    class="alert-banner w-full text-left"
    onclick={() => (expanded = !expanded)}
    aria-expanded={expanded}
  >
    <div class="banner-bar">
      <span class="icon">⚠</span>
      <span class="text text-xs font-mono uppercase tracking-wide">
        {relevant[0].headline}
        {#if relevant.length > 1}(+{relevant.length - 1} more){/if}
      </span>
      <span class="chevron text-xs">{expanded ? '▲' : '▼'}</span>
    </div>

    {#if expanded}
      <div class="banner-details">
        {#each relevant as alert}
          <div class="alert-item">
            <p class="font-mono text-xs text-amber-200">{alert.headline}</p>
            {#if alert.description}
              <p class="text-xs text-gray-400 mt-1">{alert.description}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </button>
{/if}

<style>
  .alert-banner {
    background: #1a1200;
    border: 1px solid #5a3e00;
    border-radius: 6px;
    overflow: hidden;
    margin-bottom: 12px;
  }

  .banner-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    color: #f5a623;
  }

  .icon { font-size: 0.9em; }
  .text { flex: 1; }

  .banner-details {
    border-top: 1px solid #3a2800;
    padding: 8px 12px;
  }

  .alert-item + .alert-item {
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid #2a1e00;
  }
</style>
```

**Step 2: Commit**

```bash
git add web/src/lib/components/AlertBanner.svelte
git commit -m "feat(web): add AlertBanner component for commute route alerts"
```

---

## Task 7: Build `CommuteSetup` component (onboarding)

**Files:**
- Create: `web/src/lib/components/CommuteSetup.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/CommuteSetup.svelte -->
<script lang="ts">
  import type { Stop } from '$lib/api';
  import { commute, type CommuteTrip } from '$lib/stores/commute';

  let { stops }: { stops: Stop[] } = $props();

  let step = $state<1 | 2>(1);
  let workOrigin = $state<Stop | null>(null);
  let workDest = $state<Stop | null>(null);
  let homeOrigin = $state<Stop | null>(null);
  let homeDest = $state<Stop | null>(null);

  let workOriginQuery = $state('');
  let workDestQuery = $state('');
  let homeOriginQuery = $state('');
  let homeDestQuery = $state('');

  function filterStops(query: string): Stop[] {
    if (query.length < 2) return [];
    const q = query.toLowerCase();
    return stops.filter((s) => s.name.toLowerCase().includes(q)).slice(0, 6);
  }

  function goToStep2() {
    if (!workOrigin || !workDest) return;
    // Pre-fill home as reverse
    homeOrigin = workDest;
    homeDest = workOrigin;
    homeOriginQuery = workDest.name;
    homeDestQuery = workOrigin.name;
    step = 2;
  }

  function save() {
    if (!workOrigin || !workDest || !homeOrigin || !homeDest) return;
    commute.setTrip('toWork', {
      originCode: workOrigin.code,
      originName: workOrigin.name,
      destinationCode: workDest.code,
      destinationName: workDest.name
    });
    commute.setTrip('toHome', {
      originCode: homeOrigin.code,
      originName: homeOrigin.name,
      destinationCode: homeDest.code,
      destinationName: homeDest.name
    });
  }

  function StopPicker(
    query: string,
    onSelect: (s: Stop) => void,
    onQueryChange: (q: string) => void
  ) {
    const results = filterStops(query);
    return { query, results, onSelect, onQueryChange };
  }
</script>

<div class="setup-container font-mono">
  <div class="setup-header">
    <h1 class="text-amber-400 text-lg font-bold tracking-widest uppercase">Six Rail</h1>
    <p class="text-gray-400 text-sm mt-1">Set up your commute to get started</p>
  </div>

  <div class="steps">
    <div class="step-indicator">
      <span class:active={step === 1}>1. To Work</span>
      <span class="sep text-gray-600">→</span>
      <span class:active={step === 2}>2. To Home</span>
    </div>
  </div>

  {#if step === 1}
    <div class="step-content">
      <p class="label">From</p>
      <StationSearch
        bind:query={workOriginQuery}
        {stops}
        onSelect={(s) => { workOrigin = s; workOriginQuery = s.name; }}
      />
      <p class="label mt-4">To</p>
      <StationSearch
        bind:query={workDestQuery}
        {stops}
        onSelect={(s) => { workDest = s; workDestQuery = s.name; }}
      />
      <button
        class="btn-primary mt-6"
        disabled={!workOrigin || !workDest}
        onclick={goToStep2}
      >
        Next →
      </button>
    </div>
  {:else}
    <div class="step-content">
      <p class="text-gray-400 text-xs mb-4">Pre-filled as the reverse of your work trip. Adjust if needed.</p>
      <p class="label">From</p>
      <StationSearch
        bind:query={homeOriginQuery}
        {stops}
        onSelect={(s) => { homeOrigin = s; homeOriginQuery = s.name; }}
      />
      <p class="label mt-4">To</p>
      <StationSearch
        bind:query={homeDestQuery}
        {stops}
        onSelect={(s) => { homeDest = s; homeDestQuery = s.name; }}
      />
      <div class="flex gap-3 mt-6">
        <button class="btn-secondary" onclick={() => (step = 1)}>← Back</button>
        <button
          class="btn-primary flex-1"
          disabled={!homeOrigin || !homeDest}
          onclick={save}
        >
          Start tracking
        </button>
      </div>
    </div>
  {/if}
</div>

<!-- Inline station search sub-component -->
{#snippet StationSearch({ query, stops, onSelect, query: q }: { query: string; stops: Stop[]; onSelect: (s: Stop) => void; })}
  <!-- This is a simplified inline search. See note below. -->
{/snippet}
```

> **Note:** The `CommuteSetup` component needs an inline station search input with autocomplete dropdown. Rather than duplicating the existing `SearchOverlay` logic, extract a reusable `StationSearchInput` component (see Task 8) and use it here.

**Step 2: Replace the stub above with the proper component after Task 8 is done.**

**Step 3: Commit placeholder**

```bash
git add web/src/lib/components/CommuteSetup.svelte
git commit -m "feat(web): add CommuteSetup onboarding component (WIP)"
```

---

## Task 8: Extract `StationSearchInput` reusable component

The existing `SearchOverlay.svelte` has autocomplete logic baked in. Extract the input+dropdown part into a reusable primitive.

**Files:**
- Create: `web/src/lib/components/StationSearchInput.svelte`
- Modify: `web/src/lib/components/SearchOverlay.svelte` (use the new component)

**Step 1: Read the existing SearchOverlay**

```bash
cat web/src/lib/components/SearchOverlay.svelte
```

**Step 2: Create `StationSearchInput.svelte`**

```svelte
<!-- web/src/lib/components/StationSearchInput.svelte -->
<script lang="ts">
  import type { Stop } from '$lib/api';

  let {
    stops,
    placeholder = 'Search stations...',
    value = $bindable(''),
    onSelect
  }: {
    stops: Stop[];
    placeholder?: string;
    value?: string;
    onSelect: (stop: Stop) => void;
  } = $props();

  let results = $state<Stop[]>([]);
  let showDropdown = $state(false);

  function search(q: string) {
    value = q;
    if (q.length < 2) {
      results = [];
      showDropdown = false;
      return;
    }
    const lower = q.toLowerCase();
    results = stops.filter((s) => s.name.toLowerCase().includes(lower)).slice(0, 8);
    showDropdown = results.length > 0;
  }

  function select(stop: Stop) {
    value = stop.name;
    results = [];
    showDropdown = false;
    onSelect(stop);
  }

  function onBlur() {
    setTimeout(() => (showDropdown = false), 150);
  }
</script>

<div class="station-search-input relative">
  <input
    type="text"
    class="w-full bg-[#1e1e1e] text-white font-mono text-sm px-3 py-2 rounded border border-[#333] focus:border-amber-400 focus:outline-none"
    {placeholder}
    {value}
    oninput={(e) => search((e.target as HTMLInputElement).value)}
    onblur={onBlur}
    autocomplete="off"
  />
  {#if showDropdown}
    <ul class="dropdown absolute z-50 w-full mt-1 bg-[#1e1e1e] border border-[#333] rounded shadow-lg max-h-48 overflow-y-auto">
      {#each results as stop}
        <li>
          <button
            class="w-full text-left px-3 py-2 text-sm font-mono text-white hover:bg-[#2a2a2a] focus:bg-[#2a2a2a]"
            onmousedown={() => select(stop)}
          >
            {stop.name}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
```

**Step 3: Update `CommuteSetup.svelte` to use `StationSearchInput`**

Replace the stub snippet in the CommuteSetup component with:

```svelte
<script lang="ts">
  import type { Stop } from '$lib/api';
  import { commute } from '$lib/stores/commute';
  import StationSearchInput from './StationSearchInput.svelte';

  let { stops }: { stops: Stop[] } = $props();

  let step = $state<1 | 2>(1);
  let workOrigin = $state<Stop | null>(null);
  let workDest = $state<Stop | null>(null);
  let homeOrigin = $state<Stop | null>(null);
  let homeDest = $state<Stop | null>(null);

  let workOriginQuery = $state('');
  let workDestQuery = $state('');
  let homeOriginQuery = $state('');
  let homeDestQuery = $state('');

  function goToStep2() {
    if (!workOrigin || !workDest) return;
    homeOrigin = workDest;
    homeDest = workOrigin;
    homeOriginQuery = workDest.name;
    homeDestQuery = workOrigin.name;
    step = 2;
  }

  function save() {
    if (!workOrigin || !workDest || !homeOrigin || !homeDest) return;
    commute.setTrip('toWork', {
      originCode: workOrigin.code,
      originName: workOrigin.name,
      destinationCode: workDest.code,
      destinationName: workDest.name
    });
    commute.setTrip('toHome', {
      originCode: homeOrigin.code,
      originName: homeOrigin.name,
      destinationCode: homeDest.code,
      destinationName: homeDest.name
    });
  }
</script>

<div class="min-h-screen bg-[#111] flex items-center justify-center p-6">
  <div class="w-full max-w-sm">
    <h1 class="text-amber-400 text-xl font-bold font-mono tracking-widest uppercase text-center mb-2">
      Six Rail
    </h1>
    <p class="text-gray-400 text-sm font-mono text-center mb-8">Set up your commute</p>

    <div class="flex items-center justify-center gap-4 mb-8 font-mono text-xs">
      <span class={step === 1 ? 'text-amber-400' : 'text-gray-600'}>1 TO WORK</span>
      <span class="text-gray-700">→</span>
      <span class={step === 2 ? 'text-amber-400' : 'text-gray-600'}>2 TO HOME</span>
    </div>

    {#if step === 1}
      <div class="space-y-4">
        <div>
          <label class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">From</label>
          <StationSearchInput
            {stops}
            bind:value={workOriginQuery}
            placeholder="Origin station"
            onSelect={(s) => { workOrigin = s; }}
          />
        </div>
        <div>
          <label class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">To</label>
          <StationSearchInput
            {stops}
            bind:value={workDestQuery}
            placeholder="Destination station"
            onSelect={(s) => { workDest = s; }}
          />
        </div>
        <button
          class="w-full mt-4 bg-amber-400 text-black font-mono font-bold py-3 rounded disabled:opacity-40 disabled:cursor-not-allowed"
          disabled={!workOrigin || !workDest}
          onclick={goToStep2}
        >
          NEXT →
        </button>
      </div>
    {:else}
      <div class="space-y-4">
        <p class="text-gray-500 text-xs font-mono mb-2">Pre-filled as your reverse trip. Adjust if needed.</p>
        <div>
          <label class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">From</label>
          <StationSearchInput
            {stops}
            bind:value={homeOriginQuery}
            placeholder="Origin station"
            onSelect={(s) => { homeOrigin = s; }}
          />
        </div>
        <div>
          <label class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">To</label>
          <StationSearchInput
            {stops}
            bind:value={homeDestQuery}
            placeholder="Destination station"
            onSelect={(s) => { homeDest = s; }}
          />
        </div>
        <div class="flex gap-3 mt-4">
          <button
            class="flex-1 bg-[#1e1e1e] text-white font-mono py-3 rounded border border-[#333]"
            onclick={() => (step = 1)}
          >
            ← BACK
          </button>
          <button
            class="flex-2 bg-amber-400 text-black font-mono font-bold py-3 px-6 rounded disabled:opacity-40 disabled:cursor-not-allowed"
            disabled={!homeOrigin || !homeDest}
            onclick={save}
          >
            START →
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
```

**Step 4: Run type check**

```bash
cd web && npm run check
```

**Step 5: Commit**

```bash
git add web/src/lib/components/StationSearchInput.svelte web/src/lib/components/CommuteSetup.svelte
git commit -m "feat(web): add StationSearchInput and complete CommuteSetup onboarding"
```

---

## Task 9: Build `SettingsPanel` component

**Files:**
- Create: `web/src/lib/components/SettingsPanel.svelte`

**Step 1: Create the component**

```svelte
<!-- web/src/lib/components/SettingsPanel.svelte -->
<script lang="ts">
  import { commute, notificationPrefs } from '$lib/stores/commute';
  import type { Stop } from '$lib/api';
  import StationSearchInput from './StationSearchInput.svelte';

  let { stops, onClose }: { stops: Stop[]; onClose: () => void } = $props();

  let commuteState = $state({ toWork: null, toHome: null });
  let notifState = $state({ enabled: false, thresholdMinutes: 5 as 5 | 10 | 15 });

  commute.subscribe((s) => (commuteState = s));
  notificationPrefs.subscribe((s) => (notifState = s));

  // Edit fields
  let workOriginQuery = $state(commuteState.toWork?.originName ?? '');
  let workDestQuery = $state(commuteState.toWork?.destinationName ?? '');
  let homeOriginQuery = $state(commuteState.toHome?.originName ?? '');
  let homeDestQuery = $state(commuteState.toHome?.destinationName ?? '');

  let workOrigin = $state<Stop | null>(null);
  let workDest = $state<Stop | null>(null);
  let homeOrigin = $state<Stop | null>(null);
  let homeDest = $state<Stop | null>(null);

  function save() {
    if (workOrigin && workDest) {
      commute.setTrip('toWork', {
        originCode: workOrigin.code, originName: workOrigin.name,
        destinationCode: workDest.code, destinationName: workDest.name
      });
    }
    if (homeOrigin && homeDest) {
      commute.setTrip('toHome', {
        originCode: homeOrigin.code, originName: homeOrigin.name,
        destinationCode: homeDest.code, destinationName: homeDest.name
      });
    }
    onClose();
  }

  function clearAll() {
    commute.clear();
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem('notificationPrefs');
      localStorage.removeItem('favorites');
      localStorage.removeItem('defaultStation');
    }
    onClose();
  }
</script>

<div class="settings-overlay" onclick={onClose} role="dialog" aria-modal="true" aria-label="Settings">
  <div class="settings-panel" onclick={(e) => e.stopPropagation()}>
    <div class="panel-header">
      <h2 class="font-mono text-amber-400 uppercase tracking-widest text-sm">Settings</h2>
      <button class="close-btn font-mono text-gray-500 hover:text-white" onclick={onClose}>✕</button>
    </div>

    <div class="panel-body space-y-6">
      <!-- To Work trip -->
      <section>
        <h3 class="section-title">To Work</h3>
        <div class="space-y-2">
          <StationSearchInput {stops} bind:value={workOriginQuery} placeholder="From" onSelect={(s) => (workOrigin = s)} />
          <StationSearchInput {stops} bind:value={workDestQuery} placeholder="To" onSelect={(s) => (workDest = s)} />
        </div>
      </section>

      <!-- To Home trip -->
      <section>
        <h3 class="section-title">To Home</h3>
        <div class="space-y-2">
          <StationSearchInput {stops} bind:value={homeOriginQuery} placeholder="From" onSelect={(s) => (homeOrigin = s)} />
          <StationSearchInput {stops} bind:value={homeDestQuery} placeholder="To" onSelect={(s) => (homeDest = s)} />
        </div>
      </section>

      <!-- Notifications -->
      <section>
        <h3 class="section-title">Notifications</h3>
        <label class="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={notifState.enabled}
            onchange={(e) => notificationPrefs.setEnabled((e.target as HTMLInputElement).checked)}
            class="accent-amber-400"
          />
          <span class="text-white text-sm font-mono">Notify me if delayed</span>
        </label>
        {#if notifState.enabled}
          <div class="flex gap-2 mt-2">
            {#each [5, 10, 15] as mins}
              <button
                class="threshold-btn font-mono text-xs py-1 px-3 rounded border"
                class:active={notifState.thresholdMinutes === mins}
                onclick={() => notificationPrefs.setThreshold(mins as 5 | 10 | 15)}
              >
                +{mins}m
              </button>
            {/each}
          </div>
        {/if}
      </section>

      <!-- Actions -->
      <div class="flex gap-3">
        <button class="flex-1 bg-amber-400 text-black font-mono font-bold py-2 rounded text-sm" onclick={save}>
          SAVE
        </button>
        <button
          class="flex-1 bg-red-900 text-red-300 font-mono text-sm py-2 rounded border border-red-800"
          onclick={clearAll}
        >
          CLEAR ALL DATA
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  .settings-overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: flex-end; justify-content: center;
    z-index: 100;
  }
  .settings-panel {
    background: #161616; border-top: 1px solid #2a2a2a;
    border-radius: 12px 12px 0 0; width: 100%; max-width: 480px;
    padding: 20px; max-height: 80vh; overflow-y: auto;
  }
  .panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
  .section-title { color: #6b7280; font-family: monospace; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.1em; margin-bottom: 8px; }
  .threshold-btn { border-color: #333; color: #999; background: #1e1e1e; }
  .threshold-btn.active { border-color: #f5a623; color: #f5a623; background: #1a1200; }
</style>
```

**Step 2: Run type check**

```bash
cd web && npm run check
```

**Step 3: Commit**

```bash
git add web/src/lib/components/SettingsPanel.svelte
git commit -m "feat(web): add SettingsPanel component"
```

---

## Task 10: Build `CommuteDashboard` — the main dashboard page

**Files:**
- Create: `web/src/lib/components/CommuteDashboard.svelte`
- Modify: `web/src/routes/+page.svelte`
- Modify: `web/src/routes/+page.server.ts`

**Step 1: Update `+page.server.ts` to load stops and alerts**

```typescript
// web/src/routes/+page.server.ts
import { getAllStops, getAlerts } from '$lib/api';

export async function load() {
  const [stops, alerts] = await Promise.all([
    getAllStops().catch(() => []),
    getAlerts().catch(() => [])
  ]);
  return {
    stops: Array.isArray(stops) ? stops : [],
    alerts: Array.isArray(alerts) ? alerts : []
  };
}
```

**Step 2: Create `CommuteDashboard.svelte`**

```svelte
<!-- web/src/lib/components/CommuteDashboard.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { commute, notificationPrefs, getActiveDirection } from '$lib/stores/commute';
  import type { CommuteStore } from '$lib/stores/commute';
  import type { Stop } from '$lib/api';
  import type { Alert } from '$lib/api';
  import type { Departure } from '$lib/api-client';
  import { fetchDepartures, fetchAlerts } from '$lib/api-client';
  import SplitFlapBoard from './SplitFlapBoard.svelte';
  import CountdownTimer from './CountdownTimer.svelte';
  import AlertBanner from './AlertBanner.svelte';
  import CommuteSetup from './CommuteSetup.svelte';
  import SettingsPanel from './SettingsPanel.svelte';

  let { stops, alerts: initialAlerts }: { stops: Stop[]; alerts: Alert[] } = $props();

  let commuteState = $state<CommuteStore>({ toWork: null, toHome: null });
  commute.subscribe((s) => (commuteState = s));

  let directionOverride = $state<'toWork' | 'toHome' | null>(null);
  let activeDirection = $derived(getActiveDirection(directionOverride));
  let activeTrip = $derived(commuteState[activeDirection]);

  let departures = $state<Departure[]>([]);
  let alerts = $state<Alert[]>(initialAlerts);
  let showSettings = $state(false);
  let loading = $state(false);

  let greeting = $derived(() => {
    const h = new Date().getHours();
    if (h < 12) return 'Good morning';
    if (h < 17) return 'Good afternoon';
    return 'Good evening';
  });

  let dateStr = $derived(() => {
    return new Date().toLocaleDateString('en-CA', {
      weekday: 'long', month: 'long', day: 'numeric'
    });
  });

  let nextDeparture = $derived(departures[0] ?? null);

  async function loadDepartures() {
    if (!activeTrip) return;
    loading = true;
    try {
      departures = await fetchDepartures(activeTrip.originCode);
    } finally {
      loading = false;
    }
  }

  async function loadAlerts() {
    alerts = await fetchAlerts();
  }

  onMount(() => {
    loadDepartures();
    loadAlerts();
    const departInterval = setInterval(loadDepartures, 30_000);
    const alertInterval = setInterval(loadAlerts, 60_000);
    return () => {
      clearInterval(departInterval);
      clearInterval(alertInterval);
    };
  });

  $effect(() => {
    // Reload departures when active trip changes
    activeTrip;
    loadDepartures();
  });

  let activeRouteNames = $derived(
    activeTrip ? [activeTrip.originName, activeTrip.destinationName] : []
  );

  async function requestNotifications() {
    if (!('Notification' in window)) return;
    const permission = await Notification.requestPermission();
    if (permission === 'granted') {
      notificationPrefs.setEnabled(true);
    }
  }

  let notifEnabled = $state(false);
  notificationPrefs.subscribe((s) => (notifEnabled = s.enabled));
</script>

{#if !commuteState.toWork && !commuteState.toHome}
  <CommuteSetup {stops} />
{:else}
  <div class="dashboard bg-[#111] min-h-screen text-white font-mono p-4 flex flex-col gap-4 max-w-lg mx-auto">
    <!-- Header -->
    <div class="flex items-start justify-between pt-2">
      <div>
        <h1 class="text-amber-400 font-bold text-base uppercase tracking-widest">Six Rail</h1>
        <p class="text-gray-500 text-xs mt-0.5">{greeting()} &middot; {dateStr()}</p>
      </div>
      <button
        class="text-gray-500 hover:text-white text-lg leading-none p-1"
        onclick={() => (showSettings = true)}
        aria-label="Settings"
      >
        ⚙
      </button>
    </div>

    <!-- Direction toggle -->
    <div class="flex rounded overflow-hidden border border-[#2a2a2a]">
      <button
        class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
        class:bg-amber-400={activeDirection === 'toWork'}
        class:text-black={activeDirection === 'toWork'}
        class:text-gray-400={activeDirection !== 'toWork'}
        onclick={() => (directionOverride = 'toWork')}
        disabled={!commuteState.toWork}
      >
        To Work
      </button>
      <button
        class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
        class:bg-amber-400={activeDirection === 'toHome'}
        class:text-black={activeDirection === 'toHome'}
        class:text-gray-400={activeDirection !== 'toHome'}
        onclick={() => (directionOverride = 'toHome')}
        disabled={!commuteState.toHome}
      >
        To Home
      </button>
    </div>

    {#if activeTrip}
      <!-- Route header -->
      <div class="text-center">
        <p class="text-xs text-gray-500 uppercase tracking-widest">
          {activeTrip.originName} → {activeTrip.destinationName}
        </p>
      </div>

      <!-- Alert banner -->
      <AlertBanner {alerts} routeNames={activeRouteNames} />

      <!-- Split-flap board -->
      {#if loading && departures.length === 0}
        <div class="text-center text-gray-600 text-xs py-12 font-mono animate-pulse">
          LOADING DEPARTURES...
        </div>
      {:else}
        <SplitFlapBoard {departures} maxRows={3} />
      {/if}

      <!-- Countdown -->
      {#if nextDeparture}
        <div class="flex justify-center mt-2">
          <CountdownTimer scheduledTime={nextDeparture.scheduledTime} />
        </div>
      {/if}

      <!-- Notification toggle -->
      <div class="flex items-center justify-center mt-2">
        {#if notifEnabled}
          <p class="text-green-500 text-xs font-mono">🔔 Delay notifications on</p>
        {:else}
          <button
            class="text-gray-500 text-xs font-mono hover:text-amber-400 transition-colors"
            onclick={requestNotifications}
          >
            🔔 Notify me if delayed
          </button>
        {/if}
      </div>
    {:else}
      <div class="text-center text-gray-600 text-xs py-12">
        No trip configured for this direction.<br />
        <button class="text-amber-400 mt-2" onclick={() => (showSettings = true)}>Set up in settings →</button>
      </div>
    {/if}
  </div>

  {#if showSettings}
    <SettingsPanel {stops} onClose={() => (showSettings = false)} />
  {/if}
{/if}
```

**Step 3: Update `web/src/routes/+page.svelte`**

```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import CommuteDashboard from '$lib/components/CommuteDashboard.svelte';
  let { data } = $props();
</script>

<CommuteDashboard stops={data.stops} alerts={data.alerts} />
```

**Step 4: Run type check**

```bash
cd web && npm run check
```

Expected: no errors.

**Step 5: Run dev and manually verify**

```bash
cd web && npm run dev
```

- Open `http://localhost:5173` — should show onboarding (first run) or dashboard
- Open `http://localhost:5173/map` — should show the map

**Step 6: Commit**

```bash
git add web/src/routes/+page.svelte web/src/routes/+page.server.ts web/src/lib/components/CommuteDashboard.svelte
git commit -m "feat(web): build CommuteDashboard — split-flap board, countdown, alerts, settings"
```

---

## Task 11: Add global bottom navigation bar

**Files:**
- Modify: `web/src/routes/+layout.svelte`

**Step 1: Read the current layout**

```bash
cat web/src/routes/+layout.svelte
```

**Step 2: Add bottom nav**

Add a bottom navigation bar that shows on all pages. Use SvelteKit's `$page` store to highlight the active tab.

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  let { children } = $props();
  let path = $derived($page.url.pathname);
</script>

{@render children()}

<nav class="bottom-nav" aria-label="Main navigation">
  <a href="/" class="nav-item" class:active={path === '/'}>
    <span class="icon">⊟</span>
    <span class="label">Dashboard</span>
  </a>
  <a href="/map" class="nav-item" class:active={path === '/map'}>
    <span class="icon">◎</span>
    <span class="label">Map</span>
  </a>
</nav>

<style>
  :global(body) {
    margin: 0;
    padding-bottom: 60px; /* space for nav */
    background: #111;
  }

  .bottom-nav {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 60px;
    background: #161616;
    border-top: 1px solid #2a2a2a;
    display: flex;
    z-index: 50;
  }

  .nav-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    text-decoration: none;
    color: #6b7280;
    transition: color 0.15s;
    font-family: monospace;
  }

  .nav-item.active {
    color: #f5a623;
  }

  .nav-item:hover {
    color: #d1d5db;
  }

  .icon { font-size: 1.1rem; line-height: 1; }
  .label { font-size: 0.6rem; text-transform: uppercase; letter-spacing: 0.1em; }
</style>
```

**Step 3: Adjust map page to account for bottom nav**

In `web/src/routes/map/+page.svelte`, ensure the map container is `height: calc(100vh - 60px)` or uses `pb-[60px]`.

Find the map container div (currently `style="height: 100vh"` or similar) and update it.

**Step 4: Run type check and verify**

```bash
cd web && npm run check
```

Navigate between `/` and `/map` — bottom nav should highlight the active tab.

**Step 5: Commit**

```bash
git add web/src/routes/+layout.svelte web/src/routes/map/+page.svelte
git commit -m "feat(web): add bottom navigation bar for dashboard/map tabs"
```

---

## Task 12: Add PWA manifest

**Files:**
- Create: `web/static/manifest.json`
- Create: `web/static/icons/icon-192.png` (placeholder — see note)
- Modify: `web/src/routes/+layout.svelte`

**Step 1: Create `manifest.json`**

```json
{
  "name": "Six Rail",
  "short_name": "Six Rail",
  "description": "GO Transit real-time commuter tracker",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#111111",
  "theme_color": "#111111",
  "orientation": "portrait-primary",
  "icons": [
    {
      "src": "/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any maskable"
    }
  ]
}
```

> **Note on icons:** Create simple amber-on-black train icons. You can use any SVG-to-PNG tool, or generate placeholder PNGs using a canvas script. Minimum required: 192x192 and 512x512 PNG files in `web/static/icons/`.

**Step 2: Link manifest in layout**

Add to the `<svelte:head>` in `+layout.svelte`:

```svelte
<svelte:head>
  <link rel="manifest" href="/manifest.json" />
  <meta name="theme-color" content="#111111" />
  <meta name="apple-mobile-web-app-capable" content="yes" />
  <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent" />
  <meta name="apple-mobile-web-app-title" content="Six Rail" />
</svelte:head>
```

**Step 3: Verify**

Open Chrome DevTools → Application → Manifest. Should show Six Rail manifest with no errors.

**Step 4: Commit**

```bash
git add web/static/manifest.json web/static/icons/ web/src/routes/+layout.svelte
git commit -m "feat(web): add PWA manifest for installability"
```

---

## Task 13: Add service worker for offline + background notifications

**Files:**
- Create: `web/static/sw.js`
- Modify: `web/src/routes/+layout.svelte` (register SW)

**Step 1: Create `web/static/sw.js`**

```javascript
// web/static/sw.js
const CACHE_NAME = 'sixrail-v1';
const STATIC_ASSETS = ['/', '/map', '/manifest.json'];

// Install: cache static assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

// Fetch: network-first for API, cache-first for assets
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  if (url.pathname.startsWith('/api/')) {
    // Network-first for API calls
    event.respondWith(
      fetch(event.request).catch(() =>
        caches.match(event.request)
      )
    );
    return;
  }

  // Cache-first for everything else
  event.respondWith(
    caches.match(event.request).then((cached) => cached || fetch(event.request))
  );
});

// Background sync: poll departures and notify if delayed
let lastDelayMinutes = null;
let notifPrefs = { enabled: false, thresholdMinutes: 5 };
let commuteState = { toWork: null, toHome: null };

self.addEventListener('message', (event) => {
  if (event.data?.type === 'UPDATE_PREFS') {
    notifPrefs = event.data.notifPrefs;
    commuteState = event.data.commuteState;
  }
});

async function checkDepartures() {
  if (!notifPrefs.enabled) return;

  const hour = new Date().getHours();
  const direction = hour < 12 ? 'toWork' : 'toHome';
  const trip = commuteState[direction];
  if (!trip) return;

  try {
    const res = await fetch(`/api/departures/${encodeURIComponent(trip.originCode)}`);
    if (!res.ok) return;
    const departures = await res.json();
    if (!departures.length) return;

    const next = departures[0];
    const delay = next.delayMinutes ?? 0;

    if (
      lastDelayMinutes !== null &&
      delay > lastDelayMinutes &&
      delay >= notifPrefs.thresholdMinutes
    ) {
      self.registration.showNotification('Six Rail — Delay Alert', {
        body: `Your ${next.scheduledTime} ${next.line} is now delayed ${delay} min`,
        icon: '/icons/icon-192.png',
        badge: '/icons/icon-192.png',
        tag: 'delay-alert',
        renotify: true
      });
    }

    lastDelayMinutes = delay;
  } catch {
    // Ignore network errors
  }
}

// Poll every 2 minutes
setInterval(checkDepartures, 2 * 60 * 1000);
```

**Step 2: Register the service worker in `+layout.svelte`**

Add inside the `<script>` tag:

```typescript
import { browser } from '$app/environment';
import { onMount } from 'svelte';
import { commute, notificationPrefs } from '$lib/stores/commute';

onMount(() => {
  if (!browser || !('serviceWorker' in navigator)) return;

  navigator.serviceWorker.register('/sw.js').then((reg) => {
    console.log('SW registered:', reg.scope);

    // Send prefs to SW whenever they change
    function sendPrefs(commuteState: any, notifPrefs: any) {
      reg.active?.postMessage({
        type: 'UPDATE_PREFS',
        commuteState,
        notifPrefs
      });
    }

    let cs: any, np: any;
    commute.subscribe((s) => { cs = s; if (np) sendPrefs(cs, np); });
    notificationPrefs.subscribe((s) => { np = s; if (cs) sendPrefs(cs, np); });
  });
});
```

**Step 3: Run type check**

```bash
cd web && npm run check
```

**Step 4: Verify in browser**

Open Chrome DevTools → Application → Service Workers. Should show `sw.js` as active.

**Step 5: Commit**

```bash
git add web/static/sw.js web/src/routes/+layout.svelte
git commit -m "feat(web): add service worker for offline support and background delay notifications"
```

---

## Task 14: Add "Jump to my station" button on map

**Files:**
- Modify: `web/src/routes/map/+page.svelte`

**Step 1: Find where the map overlay buttons are rendered**

Look for the `SearchOverlay` and `AlertsDropdown` in `map/+page.svelte`. Add a "jump" button near the bottom-left of the map.

**Step 2: Add the button**

Add to the map overlay section (after the map container, alongside other overlays):

```svelte
<script>
  // Add to existing imports
  import { commute, getActiveDirection } from '$lib/stores/commute';

  let commuteState = $state({ toWork: null, toHome: null });
  commute.subscribe((s) => (commuteState = s));

  let activeDirection = $derived(getActiveDirection(null));
  let activeTrip = $derived(commuteState[activeDirection]);

  function jumpToMyStation() {
    if (!activeTrip || !map) return;
    // Find the stop by code
    const stop = stops.find((s) => s.code === activeTrip.originCode);
    if (!stop) return;
    map.flyTo({ center: [stop.lon, stop.lat], zoom: 14, duration: 800 });
    selectedStop = stop;
  }
</script>

<!-- Add this button inside the map overlay area -->
{#if activeTrip}
  <button
    class="absolute bottom-20 left-4 z-10 bg-[#161616] border border-[#2a2a2a] text-amber-400 font-mono text-xs px-3 py-2 rounded shadow-lg hover:border-amber-400 transition-colors"
    onclick={jumpToMyStation}
    aria-label="Jump to my station"
  >
    ◎ My Station
  </button>
{/if}
```

**Step 3: Run type check**

```bash
cd web && npm run check
```

**Step 4: Commit**

```bash
git add web/src/routes/map/+page.svelte
git commit -m "feat(web): add 'My Station' jump button to map"
```

---

## Task 15: Format, lint, and final verification

**Step 1: Format all files**

```bash
cd web && npm run format
```

**Step 2: Type check**

```bash
cd web && npm run check
```

**Step 3: Lint**

```bash
cd web && npm run lint
```

Fix any issues reported. Re-run until clean.

**Step 4: Build check**

```bash
cd web && npm run build
```

Expected: successful build with no errors.

**Step 5: Manual smoke test**

- Open `http://localhost:5173` — shows onboarding on first visit
- Complete onboarding (pick two stops) — dashboard appears with split-flap board
- Flip direction toggle — board updates
- Navigate to `/map` — map loads, "My Station" button visible
- Bottom nav highlights correct tab
- Chrome DevTools → Application → Manifest — no errors
- Chrome DevTools → Application → Service Workers — SW active

**Step 6: Final commit**

```bash
git add -A
git commit -m "chore(web): format and lint pass for world-class commuter upgrade"
```

---

## Summary

| Task | Component | Commit |
|------|-----------|--------|
| 1 | Move map to `/map` | `refactor(web): move map to /map route` |
| 2 | Commute + notification stores | `feat(web): add commute and notification stores` |
| 3 | SplitFlapChar | `feat(web): add SplitFlapChar component` |
| 4 | SplitFlapBoard | `feat(web): add SplitFlapBoard component` |
| 5 | CountdownTimer | `feat(web): add CountdownTimer component` |
| 6 | AlertBanner | `feat(web): add AlertBanner component` |
| 7-8 | CommuteSetup + StationSearchInput | `feat(web): add onboarding components` |
| 9 | SettingsPanel | `feat(web): add SettingsPanel` |
| 10 | CommuteDashboard | `feat(web): build CommuteDashboard` |
| 11 | Bottom nav | `feat(web): add bottom navigation bar` |
| 12 | PWA manifest | `feat(web): add PWA manifest` |
| 13 | Service worker | `feat(web): add service worker` |
| 14 | Map jump button | `feat(web): add My Station button` |
| 15 | Format + lint + build | `chore(web): format and lint pass` |
