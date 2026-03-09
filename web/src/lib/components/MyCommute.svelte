<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { commute, getActiveDirection } from '$lib/stores/commute';
	import type { CommuteStore } from '$lib/stores/commute';
	import type { Stop } from '$lib/api';
	import type { Alert } from '$lib/api';
	import type { Departure } from '$lib/api-client';
	import { fetchDepartures, fetchAlerts } from '$lib/api-client';
	import { torontoHour, torontoNow } from '$lib/display';
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

	let departures = $state<Departure[]>([]);
	let alerts = $state<Alert[]>(untrack(() => initialAlerts));
	let showSettings = $state(false);

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
		return departures.filter((d) => {
			const [h, m] = d.scheduledTime.split(':').map(Number);
			return now.todayAt(h, m) > now.ms;
		});
	});

	let nextDeparture = $derived(upcomingDepartures[0] ?? null);
	let followUpDepartures = $derived(upcomingDepartures.slice(1, 3));

	async function loadDepartures(trip = activeTrip) {
		if (!trip) {
			departures = [];
			return;
		}
		try {
			departures = await fetchDepartures(trip.originCode, trip.destinationCode);
		} catch {
			departures = [];
		}
	}

	async function loadAlerts() {
		try {
			alerts = await fetchAlerts();
		} catch {
			// keep existing alerts on error
		}
	}

	let departInterval: ReturnType<typeof setInterval>;
	let alertInterval: ReturnType<typeof setInterval>;
	let tickInterval: ReturnType<typeof setInterval>;

	onMount(() => {
		commute.hydrate();
		mounted = true;
		// Departures load is handled by the $effect reacting to activeTrip after hydrate.
		// Alerts are already loaded via SSR (initialAlerts prop) — skip initial fetch.
		departInterval = setInterval(loadDepartures, 30_000);
		alertInterval = setInterval(loadAlerts, 60_000);
		tickInterval = setInterval(() => (tick += 1), 1000);
	});

	onDestroy(() => {
		clearInterval(departInterval);
		clearInterval(alertInterval);
		clearInterval(tickInterval);
		unsubCommute();
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

	function shortName(name: string): string {
		return name.replace(/\s+(GO|Station|GO Station)$/i, '').trim();
	}

	// Pass empty array — AlertBanner shows all alerts when no route filter is provided
	// TODO: store route names in commute trips to enable route-specific filtering
	let activeRouteNames = $derived<string[]>([]);
</script>

{#if !commuteState.toWork && !commuteState.toHome}
	<CommuteSetup {stops} />
{:else}
	<div
		class="my-commute bg-surface h-[calc(100dvh-60px)] text-white font-mono p-4 flex flex-col justify-center gap-4 max-w-xl mx-auto overflow-hidden"
	>
		<!-- Header -->
		<div class="flex items-start justify-between pt-2">
			<div>
				<h1 class="text-amber-400 font-bold text-base uppercase tracking-widest">Rail Six</h1>
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
		<div class="flex rounded overflow-hidden border border-border">
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
				{commuteState.toWork ? `To ${shortName(commuteState.toWork.destinationName)}` : 'To Station'}
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
				{commuteState.toHome ? `To ${shortName(commuteState.toHome.destinationName)}` : 'To Union'}
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

		<!-- Split-flap board -->
		<SplitFlapBoard {departures} maxRows={3} />

		<!-- Countdown -->
		{#if nextDeparture}
			<div class="flex flex-col items-center mt-2 gap-1">
				<CountdownTimer scheduledTime={nextDeparture.scheduledTime} />
				{#if nextDeparture.delayMinutes && nextDeparture.delayMinutes > 0}
					{@const [dh, dm] = nextDeparture.scheduledTime.split(':').map(Number)}
					{@const totalMin = dh * 60 + dm + nextDeparture.delayMinutes}
					{@const delayedTime = `${String(Math.floor(totalMin / 60) % 24).padStart(2, '0')}:${String(totalMin % 60).padStart(2, '0')}`}
					<div class="flex items-center gap-2 text-amber-400/60 text-xs mt-1">
						<span class="uppercase tracking-wider">With Delay</span>
						<CountdownTimer scheduledTime={delayedTime} size="small" />
					</div>
				{/if}
				{#if followUpDepartures.length > 0}
					<div class="flex gap-4 mt-1">
						{#each followUpDepartures as dep}
							<div class="flex items-center gap-1.5 text-gray-500 text-xs">
								<span class="uppercase tracking-wider">{dep.scheduledTime.slice(0, 5)}</span>
								<CountdownTimer scheduledTime={dep.scheduledTime} size="small" />
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

		<footer class="pt-2 pb-4 text-center max-w-sm mx-auto">
			<p class="text-gray-500 text-[11px] font-mono leading-relaxed">
				Real-time GO Transit tracking with live departures, delay alerts, and countdown timers for
				your daily commute.
			</p>
			<p class="text-gray-500 text-[9px] font-mono mt-3 leading-relaxed">
				Not affiliated with Metrolinx or GO Transit. Schedule data may be inaccurate or delayed.
				Always confirm with official sources before travelling.
			</p>
			<p class="text-gray-500 text-[10px] tracking-wide font-mono mt-3">
				<a href="mailto:hello@railsix.com" class="hover:text-gray-400 transition-colors"
					>hello@railsix.com</a
				>
			</p>
			<p class="text-gray-500 text-[10px] tracking-wide font-mono mt-2">
				&copy; {new Date().getFullYear()}
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
