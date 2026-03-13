<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		fetchDepartures,
		fetchNetworkHealth,
		type Departure,
		type NetworkLine
	} from '$lib/api-client';
	import type { Stop } from '$lib/api';
	import { stopToSlug, stopToDisplayName, stopToSeoName } from '$lib/stations';
	import SplitFlapChar from '$lib/components/SplitFlapChar.svelte';
	import {
		departureDisplayTime,
		departureTargetMs,
		isWaiting,
		padRight,
		padCenter,
		platformText,
		statusText,
		statusClass,
		torontoNow
	} from '$lib/display';

	let { data }: { data: { stops: Stop[]; stationCode: string; stationSlug: string } } = $props();

	let isFullscreen = $state(false);
	let isMobile = $state(false);
	let clock = $state('');
	let clockInterval: ReturnType<typeof setInterval>;
	let pollInterval: ReturnType<typeof setInterval>;
	let healthInterval: ReturnType<typeof setInterval>;
	let networkHealth = $state<NetworkLine[]>([]);

	let dropdownOpen = $state(false);
	let searchQuery = $state('');

	const trainStops = $derived(
		data.stops.filter((s) => /\bGO$/.test(s.name)).sort((a, b) => a.name.localeCompare(b.name))
	);

	let filteredStops = $derived(
		searchQuery.length > 0
			? trainStops.filter((s) => s.name.toLowerCase().includes(searchQuery.toLowerCase()))
			: trainStops
	);

	let selectedStopName = $derived(
		data.stops.find((s) => (s.code || s.id) === data.stationCode)?.name ?? ''
	);

	// Display name with GO Transit boilerplate stripped, for use in meta tags
	let stationDisplayName = $derived(
		selectedStopName ? stopToDisplayName({ name: selectedStopName } as Stop) : 'Union Station'
	);

	// SEO-optimized name: overrides for naming-rights stations and geographic aliases
	// (e.g. "Durham College Oshawa" → "Oshawa", "West Harbour" → "West Harbour (Hamilton)")
	let seoStationName = $derived(
		data.stops.find((s) => (s.code || s.id) === data.stationCode)
			? stopToSeoName(data.stops.find((s) => (s.code || s.id) === data.stationCode)!)
			: stationDisplayName
	);

	let isNonUnion = $derived(data.stationSlug !== 'union');

	function updateClock() {
		const now = new Date();
		clock = now.toLocaleTimeString('en-CA', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: false,
			timeZone: 'America/Toronto'
		});
	}

	function sortByScheduledTime(deps: Departure[]): Departure[] {
		const now = torontoNow();
		return [...deps].sort(
			(a, b) =>
				departureTargetMs(departureDisplayTime(a), now) -
				departureTargetMs(departureDisplayTime(b), now)
		);
	}

	let allGtfsDepartures = $state<Departure[]>([]);
	let stationAlert = $state('');
	let fetchError = $state(false);

	let trainDepartures = $derived(
		allGtfsDepartures.filter((d) => d.routeType !== 3).slice(0, isFullscreen ? 10 : 15)
	);

	let loadController: AbortController | null = null;

	async function loadDepartures() {
		if (loadController) loadController.abort();
		const controller = new AbortController();
		loadController = controller;

		try {
			const result = await fetchDepartures(data.stationCode, undefined, controller.signal);
			if (controller.signal.aborted) return;
			allGtfsDepartures = sortByScheduledTime(result.departures);
			stationAlert = result.stationAlert ?? '';
			fetchError = false;
		} catch (err) {
			if (controller.signal.aborted) return;
			fetchError = true;
			console.error('Failed to load departures:', err);
		}
	}

	function selectStation(stop: Stop) {
		dropdownOpen = false;
		searchQuery = '';
		goto(`/departures/${stopToSlug(stop)}`);
	}

	function goUnion() {
		dropdownOpen = false;
		searchQuery = '';
		goto('/departures/union');
	}

	async function loadNetworkHealth() {
		try {
			networkHealth = await fetchNetworkHealth();
		} catch (err) {
			console.error('Failed to load network health:', err);
		}
	}

	function toggleFullscreen() {
		if (!document.fullscreenElement) {
			document.documentElement.requestFullscreen();
		} else {
			document.exitFullscreen();
		}
	}

	let boardEl: HTMLElement;

	function fitFullscreen() {
		if (!boardEl || !isFullscreen) {
			if (boardEl) boardEl.style.fontSize = '';
			return;
		}
		boardEl.style.fontSize = '';
		const base = parseFloat(getComputedStyle(boardEl).fontSize);
		const available = window.innerHeight;
		const content = boardEl.scrollHeight;
		if (content > available) {
			boardEl.style.fontSize = `${base * (available / content)}px`;
		}
	}

	function detectFullscreen() {
		if (document.fullscreenElement) return true;
		// Viewport heuristic only for TV browsers that lack the Fullscreen API
		if (!document.fullscreenEnabled && window.innerHeight >= screen.height - 40) return true;
		return false;
	}

	function onFullscreenChange() {
		isFullscreen = detectFullscreen();
		document.body.classList.toggle('is-fullscreen', isFullscreen);
		requestAnimationFrame(fitFullscreen);
	}

	$effect(() => {
		trainDepartures;
		if (isFullscreen && typeof window !== 'undefined') {
			requestAnimationFrame(fitFullscreen);
		}
	});

	// Reload departures when the station changes via navigation
	$effect(() => {
		data.stationCode;
		allGtfsDepartures = [];
		loadDepartures();

		// Reset poll interval
		clearInterval(pollInterval);
		pollInterval = setInterval(loadDepartures, 30_000);
	});

	let mobileQuery: MediaQueryList;

	onMount(() => {
		updateClock();
		clockInterval = setInterval(updateClock, 1000);
		loadNetworkHealth();
		healthInterval = setInterval(loadNetworkHealth, 30_000);
		document.addEventListener('fullscreenchange', onFullscreenChange);
		window.addEventListener('resize', onFullscreenChange);
		isFullscreen = detectFullscreen();
		document.body.classList.toggle('is-fullscreen', isFullscreen);
		mobileQuery = window.matchMedia('(max-width: 480px)');
		isMobile = mobileQuery.matches;
		mobileQuery.addEventListener('change', (e) => (isMobile = e.matches));
	});

	onDestroy(() => {
		clearInterval(clockInterval);
		clearInterval(pollInterval);
		clearInterval(healthInterval);
		if (typeof document !== 'undefined') {
			document.removeEventListener('fullscreenchange', onFullscreenChange);
			window.removeEventListener('resize', onFullscreenChange);
		}
	});

	type MetaPart = { text: string; cls: string };

	function buildMetaParts(dep: { alert?: string; stops?: string[] }): MetaPart[] {
		const parts: MetaPart[] = [];
		if (dep.stops && dep.stops.length > 0)
			parts.push({ text: dep.stops.join(' · '), cls: 'text-gray-400' });
		return parts;
	}

	function marquee(node: HTMLElement) {
		const inner = node.querySelector('.stops-scroll') as HTMLElement;
		if (!inner) return;

		function update() {
			const overflow = inner.scrollWidth - node.clientWidth;
			if (overflow > 0) {
				const duration = Math.max(5, overflow / 30);
				inner.style.setProperty('--overflow', `${overflow}px`);
				inner.style.animation = `boomerang ${duration}s ease-in-out infinite alternate`;
			} else {
				inner.style.animation = '';
			}
		}

		// Delay initial check to ensure layout is computed
		requestAnimationFrame(update);
		const ro = new ResizeObserver(update);
		ro.observe(node);
		// Re-check when content changes (stops may render after action mounts)
		const mo = new MutationObserver(() => requestAnimationFrame(update));
		mo.observe(inner, { childList: true, characterData: true, subtree: true });
		return {
			destroy: () => {
				ro.disconnect();
				mo.disconnect();
			}
		};
	}
</script>

<svelte:head>
	<title>{seoStationName} GO Train Departures — Live Schedule | Rail Six</title>
	<meta
		name="description"
		content="Live GO Train departures from {seoStationName}. Real-time schedule, platform assignments, and delay alerts. Free — no account, no ads, no tracking."
	/>
	<meta
		name="keywords"
		content="{seoStationName} GO train schedule, {seoStationName} GO train times, {seoStationName} GO train departures, {seoStationName} GO station, {stationDisplayName} GO train, GO Transit {seoStationName}, GO train real time {seoStationName}, GTA train tracker, Toronto GO Transit"
	/>
	<link rel="canonical" href="https://railsix.com/departures/{data.stationSlug}" />
	<meta
		property="og:title"
		content="{seoStationName} GO Train Departures — Live Schedule | Rail Six"
	/>
	<meta
		property="og:description"
		content="Live GO Train departures from {seoStationName}. Real-time schedule, platform assignments, and delay alerts."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="Rail Six" />
	<meta property="og:url" content="https://railsix.com/departures/{data.stationSlug}" />
	<meta property="og:image" content="https://railsix.com/og-image.png" />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta property="og:image:alt" content="Rail Six — GO Train departure board" />
	<meta name="robots" content="index, follow" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:site" content="@railsix" />
	<meta name="twitter:title" content="{seoStationName} GO Train Departures — Rail Six" />
	<meta
		name="twitter:description"
		content="Live GO Train departures from {seoStationName}. Real-time schedule, platform info, and delay alerts. Free, no account."
	/>
	<meta name="twitter:image" content="https://railsix.com/og-image.png" />
</svelte:head>

<div class="board font-mono select-none bg-surface text-white" bind:this={boardEl}>
	<!-- Header: Desktop -->
	{#if !isMobile}
		<div class="board-header">
			<div>
				{#if !isFullscreen}
					<button
						class="station-title-btn"
						onclick={() => (isNonUnion ? goUnion() : (dropdownOpen = !dropdownOpen))}
						title={isNonUnion ? 'Back to Union Station' : 'Change station'}
					>
						<h1
							class="text-amber-400 font-bold uppercase tracking-[0.2em]"
							style="font-size: 0.85em;"
						>
							{#if isNonUnion}
								<span class="station-back-arrow">←</span>
							{/if}
							{selectedStopName || 'Union Station GO'}
							{#if !isNonUnion}
								<span class="station-caret">▾</span>
							{/if}
						</h1>
					</button>
				{:else}
					<h1
						class="text-amber-400 font-bold uppercase tracking-[0.2em]"
						style="font-size: 0.85em;"
					>
						{selectedStopName || 'Union Station GO'}
					</h1>
				{/if}
				<p class="text-gray-400 tracking-widest uppercase" style="font-size: 0.6em;">Departures</p>
			</div>
			{#if networkHealth.length > 0}
				<div class="network-health">
					<span class="text-gray-400 uppercase tracking-wider" style="font-size: 0.55em;"
						>Active Trains</span
					>
					<div class="network-health-pills">
						{#each networkHealth.toSorted((a, b) => a.lineCode.localeCompare(b.lineCode)) as line}
							<div class="health-pill" title="{line.lineName}: {line.activeTrips} active trains">
								<span class="text-gray-400">{line.lineCode}</span>
								<span class="text-green-400 font-bold">{line.activeTrips}</span>
							</div>
						{/each}
					</div>
				</div>
			{/if}
			<div class="flex items-center gap-0.5em">
				<div class="text-amber-400 tracking-widest tabular-nums" style="font-size: 1.1em;">
					{clock}
				</div>
				<button
					class="fullscreen-btn"
					onclick={toggleFullscreen}
					aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
					title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen (TV mode)'}
				>
					{#if isFullscreen}
						<svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor"
							><path
								d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"
							/></svg
						>
					{:else}
						<svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor"
							><path
								d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"
							/></svg
						>
					{/if}
				</button>
			</div>
		</div>

		{#if dropdownOpen}
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="desktop-picker-backdrop" onclick={() => (dropdownOpen = false)}></div>
			<div class="desktop-picker">
				<div class="desktop-picker-header">
					<h2 class="text-amber-400 uppercase tracking-widest font-bold" style="font-size: 0.7em;">
						Select Station
					</h2>
					<button
						class="text-gray-400 hover:text-white leading-none"
						style="font-size: 1.2em;"
						onclick={() => (dropdownOpen = false)}
					>
						&times;
					</button>
				</div>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					class="desktop-picker-search"
					type="text"
					placeholder="Search stations..."
					bind:value={searchQuery}
					autofocus
				/>
				<div class="desktop-picker-list">
					{#each filteredStops as stop}
						<button class="desktop-picker-item" onclick={() => selectStation(stop)}>
							{stop.name}
						</button>
					{/each}
					{#if filteredStops.length === 0}
						<div class="text-gray-400 text-center py-4" style="font-size: 14px;">
							No stations found
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{:else}
		<!-- Header: Mobile -->
		<div class="mobile-header">
			<div class="mobile-header-top">
				<button
					class="station-title-btn-mobile"
					onclick={() => (isNonUnion ? goUnion() : (dropdownOpen = !dropdownOpen))}
					title={isNonUnion ? 'Back to Union Station' : 'Change station'}
				>
					<h1 class="text-amber-400 font-bold uppercase tracking-wider text-base">
						{#if isNonUnion}
							<span class="station-back-arrow">←</span>
						{/if}
						{selectedStopName || 'Union Station GO'}
						{#if !isNonUnion}
							<span class="station-caret text-gray-400">▾</span>
						{/if}
					</h1>
					<p class="text-gray-400 tracking-widest uppercase text-[10px] mt-0.5">Departures</p>
				</button>
				<div class="text-amber-400 tracking-widest tabular-nums text-lg">
					{clock}
				</div>
			</div>

			{#if dropdownOpen}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="mobile-picker-backdrop" onclick={() => (dropdownOpen = false)}></div>
				<div class="mobile-picker">
					<div class="mobile-picker-header">
						<h2 class="text-amber-400 uppercase tracking-widest text-sm font-bold">
							Select Station
						</h2>
						<button
							class="text-gray-400 hover:text-white text-xl leading-none"
							onclick={() => (dropdownOpen = false)}
						>
							&times;
						</button>
					</div>
					<!-- svelte-ignore a11y_autofocus -->
					<input
						class="mobile-picker-search"
						type="text"
						placeholder="Search stations..."
						bind:value={searchQuery}
						autofocus
					/>
					<div class="mobile-picker-list">
						{#each filteredStops as stop}
							<button class="mobile-picker-item" onclick={() => selectStation(stop)}>
								{stop.name}
							</button>
						{/each}
						{#if filteredStops.length === 0}
							<div class="text-gray-400 text-center py-4 text-sm">No stations found</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<div class="col-headers flap-row-station">
		<span class="col-time text-amber-400">TIME</span>
		<span class="col-line text-white">LINE</span>
		{#if !isMobile}<span class="col-cars text-gray-400">CRS</span>{/if}
		<span class="col-plat text-white">PLAT</span>
		<span class="col-status text-gray-400">STATUS</span>
	</div>

	{#if stationAlert}
		<div class="text-red-500 text-center py-1 tracking-wider uppercase" style="font-size: 0.55em;">
			! {stationAlert.toUpperCase()}
		</div>
	{/if}

	{#if fetchError}
		<div
			class="text-amber-400/70 text-center py-1 tracking-wider uppercase"
			style="font-size: 0.55em;"
		>
			Unable to refresh — showing last known data
		</div>
	{/if}

	<div class="rows">
		{#each trainDepartures as dep}
			{@const metaParts = buildMetaParts(dep)}
			<div class="departure-row" class:cancelled={dep.isCancelled}>
				<div class="flap-row-station">
					<span class="col-time text-amber-400">
						{#each padRight(departureDisplayTime(dep).slice(0, 5), 5).split('') as char, j}
							<SplitFlapChar value={char} delay={j * 15} />
						{/each}
					</span>

					<span class="col-line text-white">
						{#if isMobile}
							{#each padRight((dep.lineName || dep.line) + (dep.isExpress ? ' EXP' : ''), 14).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						{:else}
							{#each padRight((dep.lineName || dep.line) + (dep.isExpress ? ' EXP' : ''), 19).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
							{#if dep.stops && dep.stops.length > 0}
								<span
									class="direction-tag {dep.lastStopId === 'UN'
										? 'text-green-400'
										: 'text-purple-400'}">TO {dep.stops[dep.stops.length - 1].toUpperCase()}</span
								>
							{/if}
						{/if}
					</span>

					{#if !isMobile}
						<span class="col-cars text-gray-400">
							{#each padRight(dep.cars && dep.cars !== '-' ? dep.cars + 'C' : '---', 3).split('') as char, j}
								<SplitFlapChar value={char} delay={40 + j * 15} />
							{/each}
						</span>
					{/if}

					<span class="col-plat text-white" class:text-amber-300={isWaiting(dep)}>
						{#each padCenter(platformText(dep), 5).split('') as char, j}
							<SplitFlapChar value={char} delay={50 + j * 12} />
						{/each}
					</span>

					<span class="col-status {statusClass(dep)}">
						{#each padCenter(statusText(dep), 7).split('') as char, j}
							<SplitFlapChar value={char} delay={60 + j * 10} />
						{/each}
					</span>
				</div>

				{#if isMobile && dep.stops && dep.stops.length > 0}
					<div
						class="direction-line {dep.lastStopId === 'UN' ? 'text-green-400' : 'text-purple-400'}"
					>
						TO {dep.stops[dep.stops.length - 1].toUpperCase()}
					</div>
				{/if}
				{#if metaParts.length > 0}
					<div class="meta-line" use:marquee>
						<span class="stops-scroll">
							{#each metaParts as part, pi}
								{#if pi > 0}<span class="text-gray-400"> · </span>{/if}
								<span class={part.cls}>{part.text.toUpperCase()}</span>
							{/each}
						</span>
					</div>
				{/if}
				{#if dep.alert}
					<div class="alert-line text-red-500">! {dep.alert.toUpperCase()}</div>
				{/if}
			</div>
		{/each}

		{#if trainDepartures.length === 0}
			<div
				class="text-gray-400 font-mono text-center tracking-widest uppercase"
				style="font-size: 0.8em; padding: 2em 0;"
			>
				No departures
			</div>
		{/if}
	</div>
</div>

<style>
	/* ── Fullscreen ── */
	:global(body.is-fullscreen) {
		padding-bottom: 0 !important;
		overflow: hidden !important;
	}

	:global(body.is-fullscreen .bottom-nav) {
		display: none !important;
	}

	.fullscreen-btn {
		background: none;
		border: none;
		color: var(--color-muted);
		font-size: 1em;
		cursor: pointer;
		padding: 0.1em;
		line-height: 1;
		transition: color 0.15s;
		font-family: inherit;
	}

	.fullscreen-btn:hover {
		color: var(--color-accent);
	}

	/* ── Viewport-scaling board ── */
	.board {
		min-height: calc(100dvh - 60px);
		display: flex;
		flex-direction: column;
		font-size: clamp(12px, 2.1vw, 25px);
		max-width: 1200px;
		margin: 0 auto;
		width: 100%;
	}

	.board-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.4em 0.8em;
		border-bottom: 1px solid var(--color-border-subtle);
		flex-shrink: 0;
		flex-wrap: wrap;
		gap: 0.3em;
	}

	/* ── Mobile header ── */
	.mobile-header {
		display: flex;
		flex-direction: column;
		padding: 12px 16px;
		gap: 10px;
		border-bottom: 1px solid var(--color-border-subtle);
		flex-shrink: 0;
	}

	.mobile-header-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	/* ── Mobile station picker ── */
	.mobile-picker-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		z-index: 60;
	}

	.mobile-picker {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 61;
		background: var(--color-surface);
		border-bottom-left-radius: 16px;
		border-bottom-right-radius: 16px;
		max-height: 75dvh;
		display: flex;
		flex-direction: column;
		font-size: 16px;
	}

	.mobile-picker-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 20px 8px;
	}

	.mobile-picker-search {
		width: 100%;
		padding: 12px 20px;
		background: var(--color-surface-overlay);
		border: none;
		border-bottom: 1px solid var(--color-border);
		color: white;
		font-family: inherit;
		font-size: 16px;
		letter-spacing: 0.05em;
		outline: none;
	}

	.mobile-picker-search::placeholder {
		color: var(--color-muted);
		text-transform: uppercase;
	}

	.mobile-picker-list {
		flex: 1;
		overflow-y: auto;
		overscroll-behavior: contain;
		-webkit-overflow-scrolling: touch;
		padding-bottom: env(safe-area-inset-bottom, 20px);
	}

	.mobile-picker-item {
		display: block;
		width: 100%;
		text-align: left;
		padding: 14px 20px;
		background: none;
		border: none;
		border-bottom: 1px solid var(--color-border-subtle);
		color: var(--color-dim);
		font-family: inherit;
		font-size: 14px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		cursor: pointer;
		transition: background 0.1s;
	}

	.mobile-picker-item:active {
		background: var(--color-surface-hover);
		color: var(--color-accent);
	}

	.network-health {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3em;
		align-items: center;
		flex-direction: column;
	}

	.network-health-pills {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3em;
		align-items: center;
	}

	.health-pill {
		display: flex;
		align-items: center;
		gap: 0.2em;
		padding: 0.1em 0.4em;
		background: var(--color-surface-overlay);
		border-radius: 3px;
		font-size: 0.55em;
	}

	.col-headers {
		padding: 0.3em 0.8em;
		border-bottom: 1px solid var(--color-surface-raised);
		flex-shrink: 0;
		letter-spacing: 0.1em;
		text-transform: uppercase;
	}

	.rows {
		flex: 1;
		padding: 0 0.8em;
	}

	.flap-row-station {
		display: grid;
		grid-template-columns: 5ch 1fr 3ch 5ch 7ch;
		gap: 0.8em;
		align-items: center;
	}

	.col-time,
	.col-line,
	.col-cars,
	.col-plat,
	.col-status {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		overflow: hidden;
	}

	.col-line {
		font-size: 0.85em;
	}
	.col-plat {
		font-size: 0.85em;
		justify-content: flex-end;
	}
	.col-cars {
		font-size: 0.8em;
		justify-content: flex-end;
	}
	.col-status {
		font-size: 0.8em;
		justify-content: flex-end;
	}

	.col-headers .col-plat,
	.col-headers .col-status {
		justify-content: center;
	}

	.departure-row {
		padding: 0.45em 0;
	}

	.departure-row.cancelled .col-time,
	.departure-row.cancelled .col-line,
	.departure-row.cancelled .col-plat,
	.departure-row.cancelled .col-cars {
		text-decoration: line-through;
		opacity: 0.4;
	}

	.direction-line {
		font-size: 0.5em;
		font-weight: bold;
		letter-spacing: 0.05em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.direction-tag {
		font-size: 0.5em;
		margin-left: 0.4em;
		white-space: nowrap;
		letter-spacing: 0.05em;
		font-weight: bold;
	}

	.meta-line {
		margin-top: 0.15em;
		font-size: 0.55em;
		overflow: hidden;
		white-space: nowrap;
	}

	.alert-line {
		margin-top: 0.1em;
		font-size: 0.5em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.stops-scroll {
		display: inline-block;
		white-space: nowrap;
	}

	@keyframes boomerang {
		0% {
			transform: translateX(0);
		}
		100% {
			transform: translateX(calc(-1 * var(--overflow)));
		}
	}

	/* ── Station title picker ── */
	.station-title-btn {
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		color: inherit;
	}

	.station-title-btn:hover h1 {
		text-decoration: underline;
		text-decoration-color: var(--color-muted);
		text-underline-offset: 0.2em;
		text-decoration-thickness: 1px;
	}

	.station-title-btn-mobile {
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		color: inherit;
	}

	.station-caret {
		font-size: 0.7em;
		color: var(--color-muted);
		transition: color 0.15s;
	}

	.station-title-btn:hover .station-caret,
	.station-title-btn-mobile:hover .station-caret {
		color: var(--color-accent);
	}

	.station-back-arrow {
		color: var(--color-muted);
		margin-right: 0.15em;
		transition: color 0.15s;
	}

	.station-title-btn:hover .station-back-arrow,
	.station-title-btn-mobile:hover .station-back-arrow {
		color: var(--color-accent);
	}

	/* ── Desktop station picker overlay ── */
	.desktop-picker-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		z-index: 60;
	}

	.desktop-picker {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		z-index: 61;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		width: 380px;
		max-height: 70dvh;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.8);
		font-size: 16px;
	}

	.desktop-picker-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 20px 8px;
	}

	.desktop-picker-header button {
		background: none;
		border: none;
		cursor: pointer;
		font-family: inherit;
	}

	.desktop-picker-search {
		width: 100%;
		padding: 12px 20px;
		background: var(--color-surface-overlay);
		border: none;
		border-bottom: 1px solid var(--color-border);
		color: white;
		font-family: inherit;
		font-size: 16px;
		letter-spacing: 0.05em;
		outline: none;
	}

	.desktop-picker-search::placeholder {
		color: var(--color-muted);
		text-transform: uppercase;
	}

	.desktop-picker-list {
		flex: 1;
		overflow-y: auto;
	}

	.desktop-picker-item {
		display: block;
		width: 100%;
		text-align: left;
		padding: 10px 20px;
		background: none;
		border: none;
		border-bottom: 1px solid var(--color-border-subtle);
		color: var(--color-dim);
		font-family: inherit;
		font-size: 14px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		cursor: pointer;
		transition: background 0.1s;
	}

	.desktop-picker-item:hover {
		background: var(--color-surface-hover);
		color: var(--color-accent);
	}

	.desktop-picker-list::-webkit-scrollbar {
		width: 4px;
	}

	.desktop-picker-list::-webkit-scrollbar-track {
		background: var(--color-surface-overlay);
	}

	.desktop-picker-list::-webkit-scrollbar-thumb {
		background: var(--color-border-input);
		border-radius: 2px;
	}

	/* ── Small screens ── */
	@media (max-width: 480px) {
		.board {
			font-size: clamp(13px, 4.2vw, 20px);
		}

		.flap-row-station {
			grid-template-columns: 5ch 1fr 5ch 7ch;
			gap: 0.6em;
		}
	}

	/* ── Large screens: lock font once board hits max-width ── */
	@media (min-width: 1200px) {
		.board {
			font-size: 25px;
		}
	}

	/* ── Fullscreen / TV: unlock scaling ── */
	:global(body.is-fullscreen) .board {
		max-width: none;
		font-size: clamp(20px, 2.1vw, 54px);
	}
</style>
