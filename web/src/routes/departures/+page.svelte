<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Stop } from '$lib/api';
	import { fetchDepartures, type Departure } from '$lib/api-client';
	import StationSearchInput from '$lib/components/StationSearchInput.svelte';
	import SplitFlapChar from '$lib/components/SplitFlapChar.svelte';
	import { padRight, padCenter, statusText, statusClass } from '$lib/display';

	let { data }: { data: { stops: Stop[] } } = $props();

	const trainStops = $derived(data.stops.filter((s) => /\bGO$/.test(s.name)));

	let selectedStop = $state<Stop | null>(null);
	let searchQuery = $state('');
	let departures = $state<Departure[]>([]);
	let loading = $state(false);
	let refreshInterval: ReturnType<typeof setInterval>;

	async function loadDepartures() {
		if (!selectedStop) return;
		const code = selectedStop.code || selectedStop.id;
		try {
			departures = await fetchDepartures(code);
		} catch {
			departures = [];
		}
		loading = false;
	}

	function selectStation(stop: Stop) {
		selectedStop = stop;
		departures = [];
		loading = true;
		loadDepartures();
	}

	onMount(() => {
		refreshInterval = setInterval(() => {
			if (selectedStop) loadDepartures();
		}, 30_000);
	});

	onDestroy(() => {
		clearInterval(refreshInterval);
	});

	function directionLabel(dep: Departure): { text: string; cls: string } {
		if (dep.stops?.some((s) => /union station/i.test(s)))
			return { text: 'TO UNION', cls: 'dir-to-union' };
		return { text: 'FROM UNION', cls: 'dir-from-union' };
	}
</script>

<svelte:head>
	<title>{selectedStop?.name ?? 'Station Lookup'} — Six Rail</title>
	<meta
		name="description"
		content="Look up real-time GO Transit departures from any station. View train times, platforms, delays, and service status."
	/>
	<meta property="og:title" content="Station Departures — Six Rail" />
	<meta
		property="og:description"
		content="Look up real-time GO Transit departures from any station."
	/>
	<meta property="og:type" content="website" />
</svelte:head>

<div class="departures-page bg-[#111] min-h-screen text-white font-mono flex flex-col gap-4">
	<!-- Header -->
	<div class="page-header">
		<h1 class="text-amber-400 font-bold uppercase tracking-widest" style="font-size: 1.1em;">
			Station Departures
		</h1>
		<p class="text-gray-500" style="font-size: 0.6em;">Look up departures from any GO station</p>
	</div>

	<!-- Station search -->
	<div class="search-wrapper">
		<StationSearchInput
			stops={trainStops}
			bind:value={searchQuery}
			placeholder="Search for a station..."
			onSelect={selectStation}
		/>
	</div>

	{#if loading}
		<div
			class="text-gray-500 text-center uppercase tracking-widest"
			style="font-size: 0.6em; padding: 2em 0;"
		>
			Loading departures...
		</div>
	{:else if selectedStop && departures.length > 0}
		<!-- Station name -->
		<div class="text-center" style="padding: 0 0.8em;">
			<p class="text-gray-500 uppercase tracking-widest" style="font-size: 0.6em;">
				{selectedStop.name}
			</p>
		</div>

		<!-- Column headers -->
		<div class="col-headers">
			<span class="col-time text-amber-400">TIME</span>
			<span class="col-line text-white">LINE</span>
			<span class="col-cars text-gray-400">CRS</span>
			<span class="col-plat text-white">PLATFRM</span>
			<span class="col-status text-gray-400">STATUS</span>
		</div>

		<!-- Departure rows -->
		<div class="rows">
			{#each departures as dep}
				{@const dir = directionLabel(dep)}
				<div class="departure-row" class:cancelled={dep.isCancelled}>
					<div class="flap-row">
						<span class="col-time text-amber-400">
							{#each padRight(dep.scheduledTime.slice(0, 5), 5).split('') as char, j}
								<SplitFlapChar value={char} delay={j * 15} />
							{/each}
						</span>

						<span class="col-line text-white">
							{#each padRight(dep.lineName || dep.line, 15).split('') as char, j}
								<SplitFlapChar value={char} delay={20 + j * 10} />
							{/each}
						</span>

						<span class="col-cars text-gray-400">
							{#each padRight(dep.cars && dep.cars !== '-' ? dep.cars + 'C' : '---', 3).split('') as char, j}
								<SplitFlapChar value={char} delay={35 + j * 15} />
							{/each}
						</span>

						<span class="col-plat text-white">
							{#each padCenter(dep.platform || '--', 7).split('') as char, j}
								<SplitFlapChar value={char} delay={40 + j * 12} />
							{/each}
						</span>

						<span class="col-status {statusClass(dep)}">
							{#each padRight(statusText(dep), 7).split('') as char, j}
								<SplitFlapChar value={char} delay={50 + j * 10} />
							{/each}
						</span>
					</div>

					<div class="meta-line">
						<span class="direction-tag {dir.cls}">{dir.text}</span>
						{#if dep.stops && dep.stops.length > 0}
							<span class="text-gray-600"> · </span>
							<span class="text-gray-500">{dep.stops.join(' · ').toUpperCase()}</span>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		<p class="text-gray-700 text-center" style="font-size: 0.45em; padding: 0.5em 0.8em;">
			Auto-refreshes every 30 seconds
		</p>
	{:else if selectedStop && departures.length === 0}
		<div
			class="text-gray-600 text-center uppercase tracking-widest"
			style="font-size: 0.7em; padding: 3em 0;"
		>
			No upcoming departures from {selectedStop.name}
		</div>
	{:else}
		<div class="text-gray-600 text-center" style="font-size: 0.7em; padding: 3em 0;">
			<p class="uppercase tracking-widest">Select a station above</p>
			<p class="text-gray-700" style="margin-top: 0.5em;">to view upcoming train departures</p>
		</div>
	{/if}
</div>

<style>
	.departures-page {
		font-size: clamp(12px, 2.1vw, 32px);
	}

	.page-header {
		padding: 0.6em 0.8em 0;
	}

	.search-wrapper {
		padding: 0 0.8em;
	}

	.col-headers,
	.flap-row {
		display: grid;
		grid-template-columns: 5ch 1fr 3ch 7ch 7ch;
		gap: 0.4em;
		align-items: center;
	}

	.col-headers {
		padding: 0.3em 0.8em;
		border-bottom: 1px solid #161616;
		letter-spacing: 0.1em;
		text-transform: uppercase;
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

	.col-cars {
		font-size: 0.8em;
		justify-content: center;
	}

	.col-plat {
		font-size: 0.85em;
		justify-content: center;
	}

	.col-status {
		font-size: 0.8em;
	}

	.rows {
		padding: 0 0.8em;
	}

	.departure-row {
		padding: 0.45em 0;
	}

	.departure-row.cancelled .col-time,
	.departure-row.cancelled .col-line,
	.departure-row.cancelled .col-cars,
	.departure-row.cancelled .col-plat {
		text-decoration: line-through;
		opacity: 0.4;
	}

	.meta-line {
		margin-top: 0.15em;
		font-size: 0.55em;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		letter-spacing: 0.05em;
	}

	.direction-tag {
		font-weight: bold;
	}

	.dir-to-union {
		color: #4ade80;
	}

	.dir-from-union {
		color: #c084fc;
	}
</style>
