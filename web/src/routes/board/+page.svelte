<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		fetchUnionDepartures,
		fetchDepartures,
		type UnionDeparture,
		type Departure
	} from '$lib/api-client';
	import type { Stop } from '$lib/api';
	import SplitFlapChar from '$lib/components/SplitFlapChar.svelte';

	let { data }: { data: { departures: UnionDeparture[]; stops: Stop[] } } = $props();

	function sortByTime(deps: UnionDeparture[]) {
		const nowH = new Date().getHours();
		const adjust = (t: string) => {
			const h = parseInt(t.slice(0, 2), 10);
			// If current hour is afternoon+ and the departure is early morning, push it after midnight
			return h < 6 && nowH >= 12 ? h + 24 : h;
		};
		return [...deps].sort((a, b) => {
			const ha = adjust(a.time) * 60 + parseInt(a.time.slice(3, 5), 10);
			const hb = adjust(b.time) * 60 + parseInt(b.time.slice(3, 5), 10);
			return ha - hb;
		});
	}

	let polledDepartures = $state<UnionDeparture[] | null>(null);
	let departures = $derived(sortByTime(polledDepartures ?? data.departures).slice(0, 10));
	let clock = $state('');
	let clockInterval: ReturnType<typeof setInterval>;
	let pollInterval: ReturnType<typeof setInterval>;

	// Station dropdown
	let selectedStation = $state('');
	let stationDepartures = $state<Departure[]>([]);
	let dropdownOpen = $state(false);
	let searchQuery = $state('');

	let filteredStops = $derived(
		searchQuery.length > 0
			? data.stops.filter((s) => s.name.toLowerCase().includes(searchQuery.toLowerCase()))
			: data.stops
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

	async function loadDepartures() {
		if (selectedStation) {
			const deps = await fetchDepartures(selectedStation);
			const nowH = new Date().getHours();
			const toMin = (t: string) => {
				const h = parseInt(t.slice(0, 2), 10);
				return (h < 6 && nowH >= 12 ? h + 24 : h) * 60 + parseInt(t.slice(3, 5), 10);
			};
			stationDepartures = deps.sort((a, b) => toMin(a.scheduledTime) - toMin(b.scheduledTime)).slice(0, 10);
		} else {
			const deps = await fetchUnionDepartures();
			if (deps.length > 0) polledDepartures = deps;
		}
	}

	function selectStation(stop: Stop) {
		selectedStation = stop.code || stop.id;
		stationDepartures = [];
		dropdownOpen = false;
		searchQuery = '';
		loadDepartures();
	}

	function clearStation() {
		selectedStation = '';
		stationDepartures = [];
		dropdownOpen = false;
		searchQuery = '';
		loadDepartures();
	}

	onMount(() => {
		updateClock();
		clockInterval = setInterval(updateClock, 1000);
		pollInterval = setInterval(loadDepartures, 30_000);
		fitBoard();
		window.addEventListener('resize', fitBoard);
	});

	onDestroy(() => {
		clearInterval(clockInterval);
		clearInterval(pollInterval);
		if (typeof window !== 'undefined') {
			window.removeEventListener('resize', fitBoard);
		}
	});

	function padRight(str: string, len: number): string {
		return str.toUpperCase().padEnd(len, ' ').slice(0, len);
	}

	function padCenter(str: string, len: number): string {
		const s = str.toUpperCase().slice(0, len);
		const left = Math.floor((len - s.length) / 2);
		return s.padStart(s.length + left, ' ').padEnd(len, ' ');
	}

	function occupancyLabel(pct: number | undefined): { text: string; cls: string } {
		if (!pct) return { text: '', cls: '' };
		if (pct <= 33) return { text: '▪', cls: 'text-green-400' };
		if (pct <= 66) return { text: '▪▪', cls: 'text-amber-400' };
		return { text: '▪▪▪', cls: 'text-red-400' };
	}

	function infoClass(info: string): string {
		if (info.includes('PROCEED')) return 'text-green-400';
		if (info.includes('WAIT')) return 'text-amber-400';
		if (info.includes('CANCEL')) return 'text-red-500';
		return 'text-gray-400';
	}

	function statusClass(d: Departure): string {
		if (d.status === 'CANCELLED') return 'text-red-500';
		if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
		return 'text-green-400';
	}

	function statusText(d: Departure): string {
		if (d.status === 'CANCELLED') return 'CANCEL';
		if (d.delayMinutes && d.delayMinutes > 0) return `+${d.delayMinutes}M`;
		return 'ON TIME';
	}

	let boardEl: HTMLElement;

	function fitBoard() {
		if (!boardEl) return;
		// Reset to base size first
		boardEl.style.fontSize = '';
		// Get the computed base font size
		const base = parseFloat(getComputedStyle(boardEl).fontSize);
		// Use the board's actual available height (accounts for nav bars, bottom bars, etc.)
		const available = boardEl.clientHeight;
		const content = boardEl.scrollHeight;
		if (content > available) {
			const scale = available / content;
			boardEl.style.fontSize = `${base * scale}px`;
		}
	}

	// Re-fit when departures change (client-side only)
	$effect(() => {
		departures;
		stationDepartures;
		if (typeof window !== 'undefined') {
			requestAnimationFrame(fitBoard);
		}
	});

	function marquee(node: HTMLElement) {
		const inner = node.querySelector('.stops-scroll') as HTMLElement;
		if (!inner) return;

		function update() {
			const overflow = inner.scrollWidth - node.clientWidth;
			if (overflow > 0) {
				inner.style.setProperty('--overflow', `${overflow}px`);
				inner.style.animation = 'boomerang 10s ease-in-out infinite alternate';
			} else {
				inner.style.animation = '';
			}
		}

		update();
		const ro = new ResizeObserver(update);
		ro.observe(node);
		return { destroy: () => ro.disconnect() };
	}
</script>

<svelte:head>
	<title>{selectedStopName || 'Union Station'} — Six Rail</title>
</svelte:head>

<div class="board font-mono select-none bg-[#0a0a0a] text-white" bind:this={boardEl}>
	<!-- Header -->
	<div class="board-header">
		<div>
			<h1 class="text-amber-400 font-bold uppercase tracking-[0.2em]" style="font-size: 0.85em;">
				{selectedStopName || 'Union Station GO'}
			</h1>
			<p class="text-gray-600 tracking-widest uppercase" style="font-size: 0.6em;">Departures</p>
		</div>
		<div class="text-amber-400 tracking-widest tabular-nums" style="font-size: 1.1em;">{clock}</div>
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
					class="uppercase tracking-widest text-gray-500 hover:text-gray-300 transition-colors"
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

	{#if selectedStation}
		<!-- Station departures view -->
		<div class="col-headers flap-row-station">
			<span class="col-time text-amber-400">TIME</span>
			<span class="col-line text-white">LINE</span>
			<span class="col-cars text-gray-400">CRS</span>
			<span class="col-plat text-white">PLATFRM</span>
			<span class="col-status text-gray-400">STATUS</span>
		</div>

		<div class="rows">
			{#each stationDepartures as dep, i}
				{@const occ = occupancyLabel(dep.occupancy)}
				<div class="departure-row" class:cancelled={dep.isCancelled}>
					<div class="flap-row-station">
						<span class="col-time text-amber-400">
							{#each padRight(dep.scheduledTime.slice(0, 5), 5).split('') as char, j}
								<SplitFlapChar value={char} delay={j * 15} />
							{/each}
						</span>

						<span class="col-line text-white">
							{#if dep.alert}<span class="text-amber-400" title="Service alert">!</span>{/if}
							{#each padRight(dep.lineName || dep.line, dep.alert ? 13 : 14).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						</span>

						<span class="col-cars text-gray-400">
							{#each padRight(dep.cars ? dep.cars + 'C' : '---', 3).split('') as char, j}
								<SplitFlapChar value={char} delay={40 + j * 15} />
							{/each}
						</span>

						<span class="col-plat text-white">
							{#each padCenter(dep.platform || '--', 7).split('') as char, j}
								<SplitFlapChar value={char} delay={50 + j * 12} />
							{/each}
						</span>

						<span class="col-status {statusClass(dep)}">
							{#each padRight(statusText(dep), 7).split('') as char, j}
								<SplitFlapChar value={char} delay={60 + j * 10} />
							{/each}
						</span>
					</div>

					<div class="meta-line">
						{#if dep.isInMotion}
							<span class="text-green-400">EN ROUTE</span>
						{/if}
						{#if occ.text}
							<span class={occ.cls} title="{dep.occupancy}% full">{occ.text}</span>
						{/if}
						{#if dep.stops && dep.stops.length > 0}
							<span class="stops-line text-gray-400 tracking-wide" use:marquee>
								<span class="stops-scroll">{dep.stops.join(' · ')}</span>
							</span>
						{/if}
					</div>
					{#if dep.alert}
						<div class="alert-line text-amber-400">⚠ {dep.alert}</div>
					{/if}
				</div>
			{/each}

			{#if stationDepartures.length === 0}
				<div class="text-gray-700 font-mono text-center tracking-widest uppercase" style="font-size: 0.8em; padding: 2em 0;">
					No departures
				</div>
			{/if}
		</div>
	{:else}
		<!-- Union Station departures view -->
		<div class="col-headers flap-row">
			<span class="col-time text-amber-400">TIME</span>
			<span class="col-service text-white">SERVICE</span>
			<span class="col-cars text-gray-400">CRS</span>
			<span class="col-plat text-white">PLATFRM</span>
			<span class="col-info text-gray-400">STATUS</span>
		</div>

		<div class="rows">
			{#each departures as dep, i}
				{@const occ = occupancyLabel(dep.occupancy)}
				<div class="departure-row" class:cancelled={dep.isCancelled}>
					<div class="flap-row">
						<span class="col-time text-amber-400">
							{#each padRight(dep.time, 5).split('') as char, j}
								<SplitFlapChar value={char} delay={j * 15} />
							{/each}
						</span>

						<span class="col-service text-white">
							{#if dep.alert}<span class="text-amber-400" title="Service alert">!</span>{/if}
							{#each padRight(dep.service, dep.alert ? 15 : 16).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						</span>

						<span class="col-cars text-gray-400">
							{#each padRight(dep.cars ? dep.cars + 'C' : '---', 3).split('') as char, j}
								<SplitFlapChar value={char} delay={40 + j * 15} />
							{/each}
						</span>

						<span class="col-plat text-white">
							{#each padCenter(dep.platform || '--', 7).split('') as char, j}
								<SplitFlapChar value={char} delay={50 + j * 12} />
							{/each}
						</span>

						<span class="col-info {infoClass(dep.info)}">
							{#each padRight(dep.isCancelled ? 'CANCEL' : dep.info, 7).split('') as char, j}
								<SplitFlapChar value={char} delay={60 + j * 10} />
							{/each}
						</span>
					</div>

					<div class="meta-line">
						{#if dep.isInMotion}
							<span class="text-green-400">EN ROUTE</span>
						{/if}
						{#if occ.text}
							<span class={occ.cls} title="{dep.occupancy}% full">{occ.text}</span>
						{/if}
						{#if dep.stops.length > 0}
							<span class="stops-line text-gray-400 tracking-wide" use:marquee>
								<span class="stops-scroll">{dep.stops.join(' · ')}</span>
							</span>
						{/if}
					</div>
					{#if dep.alert}
						<div class="alert-line text-amber-400">⚠ {dep.alert}</div>
					{/if}
				</div>
			{/each}

			{#if departures.length === 0}
				<div class="text-gray-700 font-mono text-center tracking-widest uppercase" style="font-size: 0.8em; padding: 2em 0;">
					No departures
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	/* ── Viewport-scaling board ── */
	.board {
		height: calc(100dvh - 60px);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		/* Scale base font with viewport — works from phone to TV */
		font-size: clamp(14px, 2.4vw, 48px);
	}

	.board-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.4em 0.8em;
		border-bottom: 1px solid #1a1a1a;
		flex-shrink: 0;
	}

	.col-headers {
		padding: 0.3em 0.8em;
		border-bottom: 1px solid #161616;
		flex-shrink: 0;
		letter-spacing: 0.1em;
		text-transform: uppercase;
	}

	.rows {
		flex: 1;
		padding: 0 0.8em;
	}

	.flap-row {
		display: grid;
		grid-template-columns: 5ch 1fr 3ch 7ch 7ch;
		gap: 0.4em;
		align-items: center;
	}

	.flap-row-station {
		display: grid;
		grid-template-columns: 5ch 1fr 3ch 7ch 7ch;
		gap: 0.4em;
		align-items: center;
	}

	.col-time,
	.col-service,
	.col-line,
	.col-cars,
	.col-plat,
	.col-info,
	.col-status {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		overflow: hidden;
	}

	.col-service,
	.col-line {
		font-size: 0.85em;
	}
	.col-plat {
		font-size: 0.85em;
		justify-content: center;
	}
	.col-cars {
		font-size: 0.8em;
		justify-content: center;
	}
	.col-info,
	.col-status {
		font-size: 0.8em;
	}

	.departure-row {
		padding: 0.6em 0;
	}

	.departure-row.cancelled .col-time,
	.departure-row.cancelled .col-service,
	.departure-row.cancelled .col-line,
	.departure-row.cancelled .col-plat,
	.departure-row.cancelled .col-cars {
		text-decoration: line-through;
		opacity: 0.4;
	}

	.meta-line {
		display: flex;
		align-items: center;
		gap: 0.6em;
		margin-top: 0.15em;
		font-size: 0.55em;
	}

	.alert-line {
		font-size: 0.5em;
		margin-top: 0.15em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.stops-line {
		overflow: hidden;
		white-space: nowrap;
		flex: 1;
		min-width: 0;
	}

	.stops-scroll {
		display: inline-block;
		white-space: nowrap;
	}

	@keyframes boomerang {
		0%, 20% {
			transform: translateX(0);
		}
		80%, 100% {
			transform: translateX(calc(-1 * var(--overflow, 0px)));
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
		background: #1a1a1a;
		border: 1px solid #2a2a2a;
		border-radius: 6px;
		z-index: 50;
		overflow: hidden;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
		font-size: 14px; /* dropdown stays fixed size */
	}

	.dropdown-search {
		width: 100%;
		padding: 8px 12px;
		background: #111;
		border: none;
		border-bottom: 1px solid #2a2a2a;
		color: white;
		font-family: inherit;
		font-size: 0.75rem;
		letter-spacing: 0.05em;
		outline: none;
	}

	.dropdown-search::placeholder {
		color: #555;
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
		color: #ccc;
		font-family: inherit;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		cursor: pointer;
		transition: background 0.1s;
	}

	.dropdown-item:hover {
		background: #252525;
		color: #fbbf24;
	}

	.dropdown-list::-webkit-scrollbar {
		width: 4px;
	}

	.dropdown-list::-webkit-scrollbar-track {
		background: #1a1a1a;
	}

	.dropdown-list::-webkit-scrollbar-thumb {
		background: #333;
		border-radius: 2px;
	}

	/* ── Small screens ── */
	@media (max-width: 480px) {
		.board {
			font-size: clamp(12px, 3.8vw, 20px);
		}

		.flap-row {
			grid-template-columns: 5ch 1fr 3ch 5ch 7ch;
			gap: 0.3em;
		}
		.flap-row-station {
			grid-template-columns: 5ch 1fr 3ch 5ch 7ch;
			gap: 0.3em;
		}
	}

	/* ── Large screens / TV ── */
	@media (min-width: 1400px) {
		.board {
			font-size: clamp(24px, 2.4vw, 60px);
		}
	}
</style>
