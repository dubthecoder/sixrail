<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		fetchDepartures,
		fetchNetworkHealth,
		type Departure,
		type NetworkLine
	} from '$lib/api-client';
	import type { Stop } from '$lib/api';
	import SplitFlapChar from '$lib/components/SplitFlapChar.svelte';
	import {
		compactPlatform,
		departureDisplayTime,
		departureTargetMs,
		padRight,
		padCenter,
		statusText,
		statusClass,
		torontoNow
	} from '$lib/display';

	let { data }: { data: { stops: Stop[] } } = $props();

	let isFullscreen = $state(false);
	let isMobile = $state(false);
	let clock = $state('');
	let clockInterval: ReturnType<typeof setInterval>;
	let pollInterval: ReturnType<typeof setInterval>;
	let healthInterval: ReturnType<typeof setInterval>;
	let networkHealth = $state<NetworkLine[]>([]);

	// Station dropdown — defaults to Union Station
	let selectedStation = $state('');
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
		data.stops.find((s) => (s.code || s.id) === selectedStation)?.name ?? ''
	);

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
	let fetchError = $state(false);

	let trainDepartures = $derived(
		allGtfsDepartures.filter((d) => d.routeType !== 3).slice(0, isFullscreen ? 10 : 15)
	);

	let loadController: AbortController | null = null;

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
			fetchError = true;
			console.error('Failed to load departures:', err);
		}
	}

	function selectStation(stop: Stop) {
		selectedStation = stop.code || stop.id;
		allGtfsDepartures = [];
		dropdownOpen = false;
		searchQuery = '';
		loadDepartures();
	}

	function clearStation() {
		selectedStation = '';
		allGtfsDepartures = [];
		dropdownOpen = false;
		searchQuery = '';
		loadDepartures();
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
		return !!document.fullscreenElement || window.innerHeight >= screen.height - 40;
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

	let mobileQuery: MediaQueryList;

	onMount(() => {
		updateClock();
		clockInterval = setInterval(updateClock, 1000);
		loadDepartures();
		pollInterval = setInterval(loadDepartures, 30_000);
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

	function buildMetaParts(dep: {
		isInMotion?: boolean;
		alert?: string;
		stops?: string[];
	}): MetaPart[] {
		const parts: MetaPart[] = [];
		if (dep.isInMotion) parts.push({ text: 'EN ROUTE', cls: 'text-green-400' });
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
	<title>{selectedStopName || 'Union Station'} GO Train Departures & Schedule — Rail Six</title>
	<meta
		name="description"
		content="Live GO Train departure board for {selectedStopName ||
			'Union Station'} — real-time train schedule, platform assignments, and delay alerts for Toronto GO Transit stations."
	/>
	<meta
		name="keywords"
		content="GO Train departures, {selectedStopName ||
			'Union Station'} schedule, GO Transit train times, Toronto GO Train, real-time departure board, GO station schedule, GTA train tracker"
	/>
	<meta
		property="og:title"
		content="{selectedStopName || 'Union Station'} GO Train Departures & Schedule — Rail Six"
	/>
	<meta
		property="og:description"
		content="Live GO Train departure board for {selectedStopName ||
			'Union Station'} — real-time schedule, platforms, and delay alerts."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:url" content="https://railsix.com/departures" />
	<meta property="og:image" content="https://railsix.com/train.png" />
	<meta name="twitter:card" content="summary" />
	<meta
		name="twitter:title"
		content="{selectedStopName || 'Union Station'} GO Train Departures — Rail Six"
	/>
	<meta
		name="twitter:description"
		content="Live GO Train schedule and departures from {selectedStopName ||
			'Union Station'}. Real-time delays and platform info."
	/>
	<meta name="twitter:image" content="https://railsix.com/train.png" />
</svelte:head>

<div class="board font-mono select-none bg-surface-inset text-white" bind:this={boardEl}>
	<!-- Header -->
	<div class="board-header">
		<div>
			<h1 class="text-amber-400 font-bold uppercase tracking-[0.2em]" style="font-size: 0.85em;">
				{selectedStopName || 'Union Station GO'}
			</h1>
			<p class="text-gray-600 tracking-widest uppercase" style="font-size: 0.6em;">Departures</p>
		</div>
		{#if networkHealth.length > 0}
			<div class="network-health">
				<span class="text-gray-500 uppercase tracking-wider" style="font-size: 0.55em;"
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
		<div class="station-picker relative">
			{#if selectedStation}
				<button
					class="uppercase tracking-widest text-amber-400 font-bold flex items-center gap-1"
					style="font-size: 0.6em;"
					onclick={clearStation}
				>
					{selectedStopName}
					<span class="text-gray-500">&times;</span>
				</button>
			{:else}
				<button
					class="uppercase tracking-widest text-gray-500 hover:text-amber-400 transition-colors"
					style="font-size: 0.6em;"
					onclick={() => (dropdownOpen = !dropdownOpen)}
				>
					Station ▾
				</button>
			{/if}

			{#if dropdownOpen}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="dropdown-backdrop" onclick={() => (dropdownOpen = false)}></div>
				<div class="dropdown">
					<!-- svelte-ignore a11y_autofocus -->
					<input
						class="dropdown-search"
						type="text"
						placeholder="Search stations..."
						bind:value={searchQuery}
						autofocus
					/>
					<div class="dropdown-list">
						{#each filteredStops as stop}
							<button class="dropdown-item" onclick={() => selectStation(stop)}>
								{stop.name}
							</button>
						{/each}
						{#if filteredStops.length === 0}
							<div class="px-3 py-2 text-gray-600 text-xs">No stations found</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	</div>

	<div class="col-headers flap-row-station">
		<span class="col-time text-amber-400">TIME</span>
		<span class="col-line text-white">LINE</span>
		<span class="col-cars text-gray-400">CRS</span>
		<span class="col-plat text-white">PLAT</span>
		<span class="col-status text-gray-400">STATUS</span>
	</div>

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
									class="direction-tag {dep.stops.some((s) => s.toUpperCase().includes('UNION'))
										? 'text-green-400'
										: 'text-purple-400'}">TO {dep.stops[dep.stops.length - 1].toUpperCase()}</span
								>
							{/if}
						{/if}
					</span>

					<span class="col-cars text-gray-400">
						{#each padRight(dep.cars && dep.cars !== '-' ? dep.cars + 'C' : '---', 3).split('') as char, j}
							<SplitFlapChar value={char} delay={40 + j * 15} />
						{/each}
					</span>

					<span class="col-plat text-white">
						{#each padCenter(compactPlatform(dep.platform || '--'), 5).split('') as char, j}
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
						class="direction-line {dep.stops.some((s) => s.toUpperCase().includes('UNION'))
							? 'text-green-400'
							: 'text-purple-400'}"
					>
						TO {dep.stops[dep.stops.length - 1].toUpperCase()}
					</div>
				{/if}
				{#if metaParts.length > 0}
					<div class="meta-line" use:marquee>
						<span class="stops-scroll">
							{#each metaParts as part, pi}
								{#if pi > 0}<span class="text-gray-600"> · </span>{/if}
								<span class={part.cls}>{part.text.toUpperCase()}</span>
							{/each}
						</span>
					</div>
				{/if}
				{#if dep.alert}
					<div class="alert-line text-amber-400">! {dep.alert.toUpperCase()}</div>
				{/if}
			</div>
		{/each}

		{#if trainDepartures.length === 0}
			<div
				class="text-gray-700 font-mono text-center tracking-widest uppercase"
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
		font-size: clamp(12px, 2.1vw, 42px);
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
		gap: 0.4em;
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

	/* ── Station dropdown ── */
	.station-picker {
		position: relative;
	}

	.dropdown-backdrop {
		position: fixed;
		inset: 0;
		z-index: 40;
	}

	.dropdown {
		position: absolute;
		right: 0;
		top: 100%;
		width: 260px;
		background: var(--color-surface-overlay);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		z-index: 50;
		overflow: hidden;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
		font-size: 14px; /* dropdown stays fixed size */
	}

	.dropdown-search {
		width: 100%;
		padding: 8px 12px;
		background: var(--color-surface);
		border: none;
		border-bottom: 1px solid var(--color-border);
		color: white;
		font-family: inherit;
		font-size: 0.75rem;
		letter-spacing: 0.05em;
		outline: none;
	}

	.dropdown-search::placeholder {
		color: var(--color-muted);
		text-transform: uppercase;
	}

	.dropdown-list {
		max-height: 240px;
		overflow-y: auto;
	}

	.dropdown-item {
		display: block;
		width: 100%;
		text-align: left;
		padding: 8px 12px;
		background: none;
		border: none;
		color: var(--color-dim);
		font-family: inherit;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		cursor: pointer;
		transition: background 0.1s;
	}

	.dropdown-item:hover {
		background: var(--color-surface-hover);
		color: var(--color-accent);
	}

	.dropdown-list::-webkit-scrollbar {
		width: 4px;
	}

	.dropdown-list::-webkit-scrollbar-track {
		background: var(--color-surface-overlay);
	}

	.dropdown-list::-webkit-scrollbar-thumb {
		background: var(--color-border-input);
		border-radius: 2px;
	}

	/* ── Small screens ── */
	@media (max-width: 480px) {
		.fullscreen-btn {
			display: none;
		}
		.board {
			font-size: clamp(11px, 3.5vw, 18px);
		}

		.board-header {
			flex-wrap: wrap;
			gap: 0.2em 0.5em;
		}

		.network-health {
			order: 4;
			width: 100%;
			justify-content: center;
			padding-top: 0.2em;
			border-top: 1px solid var(--color-border-subtle);
		}

		.flap-row-station {
			grid-template-columns: 5ch 1fr 3ch 5ch 7ch;
			gap: 0.3em;
		}
	}

	/* ── Large screens / TV ── */
	@media (min-width: 1400px) {
		.board {
			font-size: clamp(20px, 2.1vw, 54px);
		}
	}
</style>
