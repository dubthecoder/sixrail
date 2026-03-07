<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
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
		return [...deps].sort((a, b) => a.time.localeCompare(b.time));
	}

	let departures = $state<UnionDeparture[]>(untrack(() => sortByTime(data.departures)));
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
			? data.stops
					.filter((s) => s.name.toLowerCase().includes(searchQuery.toLowerCase()))
					.slice(0, 20)
			: data.stops.slice(0, 20)
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
			stationDepartures = deps;
		} else {
			const deps = await fetchUnionDepartures();
			if (deps.length > 0) departures = sortByTime(deps);
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
	});

	onDestroy(() => {
		clearInterval(clockInterval);
		clearInterval(pollInterval);
	});

	function padRight(str: string, len: number): string {
		return str.toUpperCase().padEnd(len, ' ').slice(0, len);
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
</script>

<svelte:head>
	<title>{selectedStopName || 'Union Station'} — Six Rail</title>
</svelte:head>

<div class="board font-mono select-none bg-[#0a0a0a] min-h-screen text-white">
	<!-- Header -->
	<div class="board-header px-4 py-3 flex items-center justify-between border-b border-[#1a1a1a]">
		<div>
			<h1 class="text-amber-400 text-sm font-bold uppercase tracking-[0.2em]">
				{selectedStopName || 'Union Station GO'}
			</h1>
			<p class="text-gray-600 text-xs tracking-widest uppercase">Departures</p>
		</div>
		<div class="text-amber-400 text-lg tracking-widest tabular-nums">{clock}</div>
	</div>

	<!-- Station dropdown -->
	<div class="flex items-center border-b border-[#1a1a1a]">
		<div class="station-picker ml-auto relative">
			{#if selectedStation}
				<button
					class="px-3 py-2 text-xs uppercase tracking-widest text-amber-400 font-bold flex items-center gap-1"
					onclick={clearStation}
				>
					{selectedStopName}
					<span class="text-gray-500">&times;</span>
				</button>
			{:else}
				<button
					class="px-3 py-2 text-xs uppercase tracking-widest text-gray-500 hover:text-gray-300 transition-colors"
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
		<div class="col-headers-station px-4 pt-3 pb-2 text-gray-600 text-xs uppercase tracking-widest border-b border-[#161616]">
			<span class="col-time">Time</span>
			<span class="col-line">Line</span>
			<span class="col-plat">Plat</span>
			<span class="col-status">Status</span>
		</div>

		<div class="rows px-4">
			{#each stationDepartures as dep, i}
				<div class="departure-row" class:first={i === 0}>
					<div class="flap-row-station">
						<span class="col-time text-amber-400">
							{#each padRight(dep.scheduledTime.slice(0, 5), 5).split('') as char, j}
								<SplitFlapChar value={char} delay={j * 15} />
							{/each}
						</span>

						<span class="col-line text-white">
							{#each padRight(dep.lineName || dep.line, 14).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						</span>

						<span class="col-plat text-white">
							{#each padRight(dep.platform || '--', 4).split('') as char, j}
								<SplitFlapChar value={char} delay={50 + j * 12} />
							{/each}
						</span>

						<span class="col-status {statusClass(dep)}">
							{#each padRight(statusText(dep), 7).split('') as char, j}
								<SplitFlapChar value={char} delay={60 + j * 10} />
							{/each}
						</span>
					</div>

					{#if dep.stops && dep.stops.length > 0}
						<div class="stops-line text-gray-400 text-xs tracking-wide truncate pl-[calc(5ch+2px+8px)]">
							{dep.stops.join(' · ')}
						</div>
					{/if}
				</div>
			{/each}

			{#if stationDepartures.length === 0}
				<div class="text-gray-700 font-mono text-sm py-16 text-center tracking-widest uppercase">
					No departures
				</div>
			{/if}
		</div>
	{:else}
		<!-- Union Station departures view -->
		<div class="col-headers px-4 pt-3 pb-2 text-gray-600 text-xs uppercase tracking-widest border-b border-[#161616]">
			<span class="col-time">Time</span>
			<span class="col-service">Service</span>
			<span class="col-plat">Plat</span>
			<span class="col-info">Status</span>
		</div>

		<div class="rows px-4">
			{#each departures as dep, i}
				<div class="departure-row" class:first={i === 0}>
					<div class="flap-row">
						<span class="col-time text-amber-400">
							{#each padRight(dep.time, 5).split('') as char, j}
								<SplitFlapChar value={char} delay={j * 15} />
							{/each}
						</span>

						<span class="col-service text-white">
							{#each padRight(dep.service, 16).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						</span>

						<span class="col-plat text-white">
							{#each padRight(dep.platform || '--', 5).split('') as char, j}
								<SplitFlapChar value={char} delay={50 + j * 12} />
							{/each}
						</span>

						<span class="col-info {infoClass(dep.info)}">
							{#each padRight(dep.info, 7).split('') as char, j}
								<SplitFlapChar value={char} delay={60 + j * 10} />
							{/each}
						</span>
					</div>

					{#if dep.stops.length > 0}
						<div class="stops-line text-gray-400 text-xs tracking-wide truncate pl-[calc(5ch+2px+8px)]">
							{dep.stops.join(' · ')}
						</div>
					{/if}
				</div>
			{/each}

			{#if departures.length === 0}
				<div class="text-gray-700 font-mono text-sm py-16 text-center tracking-widest uppercase">
					No departures
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.col-headers,
	.flap-row {
		display: grid;
		grid-template-columns: 5ch 16ch 5ch 7ch;
		gap: 8px;
		align-items: center;
	}

	.col-headers-station,
	.flap-row-station {
		display: grid;
		grid-template-columns: 5ch 14ch 4ch 7ch;
		gap: 8px;
		align-items: center;
	}

	.col-time,
	.col-service,
	.col-line,
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
	}
	.col-info,
	.col-status {
		font-size: 0.8em;
	}

	.departure-row {
		border-bottom: 1px solid #161616;
		padding: 8px 0;
	}

	.departure-row.first {
		border-bottom-color: #222;
		padding: 10px 0;
	}

	.stops-line {
		margin-top: 3px;
	}

	/* Station dropdown */
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

	@media (max-width: 480px) {
		.col-headers,
		.flap-row {
			grid-template-columns: 5ch 13ch 4ch 7ch;
			gap: 4px;
		}

		.col-headers-station,
		.flap-row-station {
			grid-template-columns: 5ch 11ch 3ch 7ch;
			gap: 4px;
		}
	}
</style>
