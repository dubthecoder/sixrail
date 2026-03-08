<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { commute, notificationPrefs, getActiveDirection } from '$lib/stores/commute';
	import type { CommuteStore } from '$lib/stores/commute';
	import type { Stop } from '$lib/api';
	import type { Alert } from '$lib/api';
	import type { Departure } from '$lib/api-client';
	import { fetchDepartures, fetchAlerts } from '$lib/api-client';
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
	let activeDirection = $derived(getActiveDirection(directionOverride));
	let activeTrip = $derived(commuteState[activeDirection]);

	let departures = $state<Departure[]>([]);
	let alerts = $state<Alert[]>(untrack(() => initialAlerts));
	let showSettings = $state(false);
	let loading = $state(false);

	function greeting(): string {
		const h = new Date().getHours();
		if (h < 12) return 'Good morning';
		if (h < 17) return 'Good afternoon';
		return 'Good evening';
	}

	function dateStr(): string {
		return new Date().toLocaleDateString('en-CA', {
			weekday: 'long',
			month: 'long',
			day: 'numeric'
		});
	}

	let nextDeparture = $derived(departures[0] ?? null);

	async function loadDepartures() {
		if (!activeTrip) {
			departures = [];
			return;
		}
		loading = true;
		try {
			departures = await fetchDepartures(activeTrip.originCode, activeTrip.destinationCode);
		} catch {
			departures = [];
		} finally {
			loading = false;
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

	onMount(() => {
		loadDepartures();
		// Alerts are already loaded via SSR (initialAlerts prop) — skip initial fetch.
		departInterval = setInterval(loadDepartures, 30_000);
		alertInterval = setInterval(loadAlerts, 60_000);
		mounted = true;
	});

	onDestroy(() => {
		clearInterval(departInterval);
		clearInterval(alertInterval);
		unsubCommute();
	});

	let mounted = $state(false);
	$effect(() => {
		// Reload when active trip changes (skip SSR and initial mount — onMount handles that)
		activeTrip;
		if (browser && mounted) {
			loadDepartures();
		}
	});

	// Pass empty array — AlertBanner shows all alerts when no route filter is provided
	// TODO: store route names in commute trips to enable route-specific filtering
	let activeRouteNames = $derived<string[]>([]);

	async function requestNotifications() {
		if (!('Notification' in window)) return;
		const permission = await Notification.requestPermission();
		if (permission === 'granted') {
			notificationPrefs.setEnabled(true);
		}
	}

	let notifEnabled = $state(false);
	const unsubNotif = notificationPrefs.subscribe((s) => (notifEnabled = s.enabled));
	onDestroy(() => unsubNotif());
</script>

{#if !commuteState.toWork && !commuteState.toHome}
	<CommuteSetup {stops} />
{:else}
	<div
		class="my-commute bg-[#111] min-h-screen text-white font-mono p-4 flex flex-col gap-4 max-w-lg mx-auto"
	>
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
				<button class="text-amber-400 mt-2" onclick={() => (showSettings = true)}
					>Set up in settings →</button
				>
			</div>
		{/if}

		<footer class="mt-auto pt-6 pb-4 text-center">
			<p class="text-gray-700 text-[9px] font-mono leading-relaxed">
				Not affiliated with Metrolinx or GO Transit. Schedule data may be inaccurate or delayed.
			</p>
			<p class="text-gray-600 text-[10px] tracking-wide font-mono mt-2">
				&copy; {new Date().getFullYear()}
				<a
					href="https://teclara.tech"
					target="_blank"
					rel="noopener noreferrer"
					class="hover:text-gray-400 transition-colors">Teclara Technologies Inc</a
				>
			</p>
		</footer>
	</div>

	{#if showSettings}
		<SettingsPanel {stops} onClose={() => (showSettings = false)} />
	{/if}
{/if}
