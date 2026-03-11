<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
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

	let commuteState = $state<CommuteStore>({ toWork: null, toHome: null });
	const unsubCommute = commute.subscribe((s) => (commuteState = s));

	let directionOverride = $state<'toWork' | 'toHome' | null>(null);
	let activeDirection = $derived(getActiveDirection(directionOverride, commuteState));
	let activeTrip = $derived.by(() =>
		activeDirection === 'toWork' ? commuteState.toWork : commuteState.toHome
	);
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
	let followUpDepartures = $derived(upcomingDepartures.slice(1, 3));

	async function loadDepartures(trip = activeTrip) {
		if (!trip) {
			departures = [];
			fetchError = false;
			return;
		}
		try {
			const result = await fetchDepartures(trip.originCode, trip.destinationCode);
			departures = result.departures;
			fetchError = false;
		} catch (err) {
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
		const trip = activeDirection === 'toWork' ? commuteState.toWork : commuteState.toHome;
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

	// Pass empty array — AlertBanner shows all alerts when no route filter is provided
	// TODO: store route names in commute trips to enable route-specific filtering
	let activeRouteNames = $derived<string[]>([]);
</script>

{#if !commuteState.toWork && !commuteState.toHome}
	<CommuteSetup {stops} />
{:else}
	<div
		class="my-commute bg-surface h-[calc(100dvh-60px)] text-white font-mono p-4 flex flex-col gap-3 max-w-xl mx-auto overflow-hidden"
	>
		<!-- Header -->
		<div class="grid grid-cols-[1fr_auto_1fr] items-start pt-2 shrink-0">
			<div aria-hidden="true"></div>
			<div class="text-center">
				<h1 class="text-amber-400 font-bold text-base uppercase tracking-widest">Rail Six</h1>
				<p class="text-gray-500 text-xs mt-0.5">{greeting()} &middot; {dateStr()}</p>
			</div>
			<button
				class="justify-self-end text-gray-500 hover:text-white text-lg leading-none p-1"
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
					directionOverride = 'toWork';
					track('direction-toggle', { direction: 'toWork' });
				}}
				disabled={!commuteState.toWork}
			>
				To Work
			</button>
			<button
				class="flex-1 py-2 text-xs uppercase tracking-wider transition-colors"
				class:bg-amber-400={activeDirection === 'toHome'}
				class:text-black={activeDirection === 'toHome'}
				class:text-gray-400={activeDirection !== 'toHome'}
				onclick={() => {
					directionOverride = 'toHome';
					track('direction-toggle', { direction: 'toHome' });
				}}
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
		{/if}

		<!-- Alert banner -->
		<AlertBanner {alerts} routeNames={activeRouteNames} />

		{#if fetchError}
			<div class="text-amber-400/70 text-xs text-center py-1 tracking-wider uppercase">
				Unable to refresh — showing last known data
			</div>
		{/if}

		<!-- Split-flap board -->
		<SplitFlapBoard {departures} maxRows={3} />

		<!-- Countdown -->
		{#if nextDeparture}
			<div class="flex flex-col items-center gap-1 shrink-0">
				<CountdownTimer
					scheduledTime={departureDisplayTime(nextDeparture)}
					originalScheduledTime={departureDisplayTime(nextDeparture) !== nextDeparture.scheduledTime
						? nextDeparture.scheduledTime
						: undefined}
				/>
				{#if followUpDepartures.length > 0}
					<div class="flex gap-4 mt-1">
						{#each followUpDepartures as dep}
							<div class="flex items-center gap-1.5 text-gray-500 text-xs">
								<span class="uppercase tracking-wider">{departureDisplayTime(dep).slice(0, 5)}</span
								>
								<CountdownTimer scheduledTime={departureDisplayTime(dep)} size="small" />
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		{#if !activeTrip}
			<div class="text-center text-gray-600 text-xs py-12">
				No trip configured for this direction.<br />
				<button class="text-amber-400 mt-2" onclick={() => (showSettings = true)}
					>Set up in settings →</button
				>
			</div>
		{/if}

		<footer class="mt-auto pt-2 pb-2 text-center max-w-sm mx-auto shrink-0">
			<p class="text-gray-500 text-[9px] font-mono leading-relaxed">
				Not affiliated with Metrolinx or GO Transit. Schedule data may be inaccurate or delayed.
				Always confirm with official sources before travelling.
			</p>
			<p class="text-gray-500 text-[10px] tracking-wide font-mono mt-1.5">
				<a href="mailto:hello@railsix.com" class="hover:text-gray-400 transition-colors"
					>hello@railsix.com</a
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
