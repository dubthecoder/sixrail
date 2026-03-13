<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { commute, getActiveDirection } from '$lib/stores/commute';
	import type { CommuteStore } from '$lib/stores/commute';
	import type { Stop } from '$lib/api';
	import type { Alert } from '$lib/api';
	import type { Departure } from '$lib/api-client';
	import { fetchAlerts, fetchDepartures } from '$lib/api-client';
	import { normalizeAlerts } from '$lib/alerts';
	import { onSSE, onSSEStatus } from '$lib/sse';
	import { departureDisplayTime, isUpcomingDeparture, torontoHour, torontoNow } from '$lib/display';
	import { track } from '$lib/track';
	import { untrack } from 'svelte';
	import SplitFlapBoard from './SplitFlapBoard.svelte';
	import CountdownTimer from './CountdownTimer.svelte';
	import AlertBanner from './AlertBanner.svelte';
	import CommuteSetup from './CommuteSetup.svelte';
	import SettingsPanel from './SettingsPanel.svelte';

	let { stops, alerts: initialAlerts }: { stops: Stop[]; alerts: Alert[] } = $props();

	// Derive urlTrip from the actual URL — stays in sync with replaceState
	// Match on code or id (stops may have empty code, using id as fallback)
	function findStop(stops: Stop[], val: string) {
		return stops.find((s) => (s.code || s.id) === val);
	}

	let urlTrip = $derived.by(() => {
		const from = page.url.searchParams.get('from');
		const to = page.url.searchParams.get('to');
		const dir = page.url.searchParams.get('dir');
		if (!from || !to || (dir !== 'toWork' && dir !== 'toHome')) return null;
		const fromStop = findStop(stops, from);
		const toStop = findStop(stops, to);
		if (!fromStop || !toStop) return null;
		return {
			fromCode: fromStop.code || fromStop.id,
			fromName: fromStop.name,
			toCode: toStop.code || toStop.id,
			toName: toStop.name,
			dir: dir as 'toWork' | 'toHome'
		};
	});
	let isUrlMode = $derived(urlTrip !== null);

	let commuteState = $state<CommuteStore>({ toWork: null, toHome: null });
	const unsubCommute = commute.subscribe((s) => (commuteState = s));

	let directionOverride = $state<'toWork' | 'toHome' | null>(null);
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
	const ALERT_REFRESH_INTERVAL_MS = 30_000;

	let departures = $state<Departure[]>([]);
	let alerts = $state<Alert[]>(normalizeAlerts(untrack(() => initialAlerts)));
	let showSettings = $state(false);
	let fetchError = $state(false);

	function greeting(): string {
		const h = torontoHour();
		if (h < 12) return 'Good morning';
		if (h < 17) return 'Good afternoon';
		return 'Good evening';
	}

	function dateStr(): string {
		return new Date().toLocaleDateString('en-CA', {
			weekday: 'long',
			month: 'long',
			day: 'numeric',
			timeZone: 'America/Toronto'
		});
	}

	let tick = $state(0);
	let upcomingDepartures = $derived.by(() => {
		tick; // re-evaluate each tick
		const now = torontoNow();
		return departures.filter((d) => isUpcomingDeparture(d, now));
	});

	let nextDeparture = $derived(upcomingDepartures[0] ?? null);
	let followUpDepartures = $derived(upcomingDepartures.slice(1, 4));

	function syncUrl(
		trip: { originCode: string; destinationCode: string } | null,
		dir: 'toWork' | 'toHome'
	) {
		if (!browser || !trip) return;
		const params = new URLSearchParams({
			from: trip.originCode,
			to: trip.destinationCode,
			dir
		});
		replaceState(`/?${params}`, {});
	}

	let loadController: AbortController | null = null;

	async function loadDepartures(trip = activeTrip) {
		if (loadController) loadController.abort();

		if (!trip) {
			departures = [];
			fetchError = false;
			return;
		}

		const controller = new AbortController();
		loadController = controller;

		try {
			const result = await fetchDepartures(
				trip.originCode,
				trip.destinationCode,
				controller.signal
			);
			if (controller.signal.aborted) return;
			departures = result.departures;
			fetchError = false;
		} catch (err) {
			if (controller.signal.aborted) return;
			fetchError = true;
			console.error('Failed to load departures:', err);
		}
	}

	let alertRefreshController = $state<AbortController | null>(null);
	function replaceAlerts(next: unknown) {
		alerts = normalizeAlerts(next);
	}

	function applyRealtimeAlerts(next: unknown) {
		alertRefreshController?.abort();
		replaceAlerts(next);
	}

	async function loadAlerts() {
		alertRefreshController?.abort();
		const controller = new AbortController();
		alertRefreshController = controller;

		try {
			const nextAlerts = await fetchAlerts(controller.signal);
			if (controller.signal.aborted || alertRefreshController !== controller) return;
			replaceAlerts(nextAlerts);
		} catch (err) {
			if (controller.signal.aborted) return;
			console.error('Failed to refresh alerts:', err);
		} finally {
			if (alertRefreshController === controller) {
				alertRefreshController = null;
			}
		}
	}

	let departInterval: ReturnType<typeof setInterval>;
	let alertInterval: ReturnType<typeof setInterval>;
	let tickInterval: ReturnType<typeof setInterval>;
	let unsubSSEAlerts: (() => void) | undefined;
	let unsubSSEStatus: (() => void) | undefined;

	onMount(() => {
		function refreshAlertsOnResume() {
			if (document.visibilityState !== 'hidden') {
				void loadAlerts();
			}
		}

		commute.hydrate();
		mounted = true;
		// Sync URL to reflect active commute after hydrate
		// Deferred to ensure SvelteKit router is initialized before replaceState
		setTimeout(() => {
			if (!isUrlMode && activeTrip) {
				syncUrl(activeTrip, activeDirection);
			}
		}, 0);
		// Departures load is handled by the $effect reacting to activeTrip after hydrate.
		departInterval = setInterval(loadDepartures, 30_000);
		alertInterval = setInterval(() => void loadAlerts(), ALERT_REFRESH_INTERVAL_MS);
		tickInterval = setInterval(() => (tick += 1), 1000);
		void loadAlerts();
		window.addEventListener('focus', refreshAlertsOnResume);
		document.addEventListener('visibilitychange', refreshAlertsOnResume);
		// Real-time alerts via SSE
		unsubSSEAlerts = onSSE('alerts', (data) => {
			applyRealtimeAlerts(data);
		});
		unsubSSEStatus = onSSEStatus((connected) => {
			if (connected) void loadAlerts();
		});

		return () => {
			clearInterval(departInterval);
			clearInterval(alertInterval);
			clearInterval(tickInterval);
			alertRefreshController?.abort();
			window.removeEventListener('focus', refreshAlertsOnResume);
			document.removeEventListener('visibilitychange', refreshAlertsOnResume);
			unsubSSEAlerts?.();
			unsubSSEStatus?.();
			unsubCommute();
		};
	});

	let mounted = $state(false);
	$effect(() => {
		const trip = activeTrip;
		if (browser && mounted) {
			void loadDepartures(trip);
		}
	});

	// When all departures have passed, fetch fresh data immediately
	let prevNext: Departure | null = null;
	$effect(() => {
		if (prevNext && !nextDeparture && activeTrip) {
			void loadDepartures();
		}
		prevNext = nextDeparture;
	});

	// When commute is cleared, also clear URL params so setup screen shows
	let hadCommute = $state(false);
	$effect(() => {
		const hasCommute = !!(commuteState.toWork || commuteState.toHome);
		if (hadCommute && !hasCommute && mounted) {
			setTimeout(() => replaceState('/', {}), 0);
		}
		hadCommute = hasCommute;
	});

	// Pass empty array — AlertBanner shows all alerts when no route filter is provided
	// TODO: store route names in commute trips to enable route-specific filtering
	const activeRouteNames: string[] = [];
</script>

{#if !isUrlMode && !commuteState.toWork && !commuteState.toHome}
	<CommuteSetup {stops} />
{:else}
	<div
		class="my-commute bg-surface h-[calc(100dvh-60px)] text-white font-mono px-3 py-4 flex flex-col gap-3 max-w-lg mx-auto overflow-hidden"
	>
		<!-- Header -->
		<div class="grid grid-cols-[1fr_auto_1fr] items-start pt-2 shrink-0">
			<div aria-hidden="true"></div>
			<div class="text-center">
				<h1 class="text-amber-400 font-bold text-base uppercase tracking-widest">Rail Six</h1>
				<p class="text-gray-400 text-xs mt-0.5">{greeting()} &middot; {dateStr()}</p>
			</div>
			<button
				class="justify-self-end text-gray-400 hover:text-white text-lg leading-none p-1"
				onclick={() => (showSettings = true)}
				aria-label="Settings"
			>
				⚙
			</button>
		</div>

		<!-- Direction toggle -->
		<div class="flex rounded overflow-hidden border border-border shrink-0">
			<button
				class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
				class:bg-amber-400={activeDirection === 'toWork'}
				class:text-black={activeDirection === 'toWork'}
				class:text-gray-400={activeDirection !== 'toWork'}
				onclick={() => {
					if (isUrlMode) {
						const params = new URLSearchParams({
							from: urlTrip!.toCode,
							to: urlTrip!.fromCode,
							dir: 'toWork'
						});
						replaceState(`/?${params}`, {});
					} else {
						directionOverride = 'toWork';
						if (commuteState.toWork) {
							syncUrl(commuteState.toWork, 'toWork');
						}
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
						const params = new URLSearchParams({
							from: urlTrip!.toCode,
							to: urlTrip!.fromCode,
							dir: 'toHome'
						});
						replaceState(`/?${params}`, {});
					} else {
						directionOverride = 'toHome';
						if (commuteState.toHome) {
							syncUrl(commuteState.toHome, 'toHome');
						}
					}
					track('direction-toggle', { direction: 'toHome' });
				}}
				disabled={!isUrlMode && !commuteState.toHome}
			>
				To Home
			</button>
		</div>

		<!-- Route header -->
		<div class="text-center">
			<p class="text-xs text-gray-400 uppercase tracking-widest">
				{#if activeTrip}
					{activeTrip.originName} → {activeTrip.destinationName}
				{:else}
					No trip configured
				{/if}
			</p>
		</div>

		<!-- Alert banner -->
		<AlertBanner {alerts} routeNames={activeRouteNames} />

		{#if fetchError}
			<div class="text-amber-400/70 text-xs text-center py-1 tracking-wider uppercase">
				Unable to refresh — showing last known data
			</div>
		{/if}

		<!-- Split-flap board -->
		<SplitFlapBoard {departures} maxRows={4} {tick} fillEmpty />

		<!-- Countdown -->
		<div class="countdown-group shrink-0">
			{#if nextDeparture}
				<CountdownTimer
					scheduledTime={nextDeparture.scheduledTime}
					delayedTime={departureDisplayTime(nextDeparture) !== nextDeparture.scheduledTime
						? departureDisplayTime(nextDeparture)
						: undefined}
				/>
			{:else}
				<CountdownTimer scheduledTime="" empty />
			{/if}
			<div class="followup-timers">
				{#each Array(3) as _, i}
					{#if followUpDepartures[i]}
						<CountdownTimer
							scheduledTime={departureDisplayTime(followUpDepartures[i])}
							departureTime={followUpDepartures[i].scheduledTime}
							size="small"
						/>
					{:else}
						<CountdownTimer scheduledTime="" size="small" empty />
					{/if}
				{/each}
			</div>
		</div>

		<footer class="mt-auto pt-2 pb-2 text-center max-w-sm mx-auto shrink-0">
			<p class="text-gray-400 text-[10px] font-mono leading-relaxed">
				Not affiliated with Metrolinx or GO Transit. Schedule data may be inaccurate or delayed.
				Always confirm with official sources.
			</p>
			<p class="text-gray-400 text-[10px] tracking-wide font-mono mt-1.5">
				<a
					href="mailto:hello@railsix.com"
					class="text-amber-400 hover:text-amber-300 transition-colors">hello@railsix.com</a
				>
				&middot; &copy; {new Date().getFullYear()}
				<a
					href="https://teclara.tech"
					target="_blank"
					rel="noopener noreferrer"
					class="hover:text-gray-400 transition-colors">Teclara Technologies Inc.</a
				>
			</p>
		</footer>
	</div>

	{#if showSettings}
		<SettingsPanel {stops} onClose={() => (showSettings = false)} />
	{/if}
{/if}

<style>
	.countdown-group {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
	}

	.followup-timers {
		display: flex;
		flex-direction: row;
		gap: 16px;
		margin-top: 4px;
	}
</style>
