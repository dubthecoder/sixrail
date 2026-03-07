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
		return t.slice(0, 5);
	}

	function statusText(d: Departure): string {
		if (d.isCancelled || d.status === 'Cancelled') return 'CANCELLED  ';
		if (d.delayMinutes && d.delayMinutes > 0) return `DELAYED +${d.delayMinutes}MIN`;
		return 'ON TIME    ';
	}

	function statusClass(d: Departure): string {
		if (d.isCancelled || d.status === 'Cancelled') return 'text-red-500';
		if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
		return 'text-green-400';
	}

	function occupancyIcon(pct: number | undefined): string {
		if (pct == null || pct === 0) return '';
		if (pct <= 40) return '\u25CB'; // empty circle - lots of seats
		if (pct <= 75) return '\u25D1'; // half circle - some seats
		return '\u25CF'; // full circle - standing room
	}

	function occupancyClass(pct: number | undefined): string {
		if (pct == null || pct === 0) return '';
		if (pct <= 40) return 'text-green-400';
		if (pct <= 75) return 'text-amber-400';
		return 'text-red-400';
	}

	let rows = $derived(departures.slice(0, maxRows));
</script>

<div class="split-flap-board font-mono select-none" role="region" aria-label="Departure board">
	<!-- Header -->
	<div class="board-header">
		<span class="col-time text-gray-500 text-xs uppercase tracking-widest">Time</span>
		<span class="col-route text-gray-500 text-xs uppercase tracking-widest">Route</span>
		<span class="col-cars text-gray-500 text-xs uppercase tracking-widest">Cars</span>
		<span class="col-platform text-gray-500 text-xs uppercase tracking-widest">Plat</span>
		<span class="col-arrival text-gray-500 text-xs uppercase tracking-widest">Arrv</span>
		<span class="col-occ text-gray-500 text-xs uppercase tracking-widest"></span>
		<span class="col-status text-gray-500 text-xs uppercase tracking-widest">Status</span>
	</div>

	<!-- Rows -->
	{#each rows as dep, i}
		<div
			class="board-row"
			class:next-train={i === 0}
			class:cancelled={dep.isCancelled || dep.status === 'Cancelled'}
		>
			<span class="col-time text-amber-400">
				{#each formatTime(dep.scheduledTime).split('') as char, j}
					<SplitFlapChar value={char} delay={j * 30} />
				{/each}
			</span>

			<span class="col-route text-white">
				{#each padRight(dep.line, 10).split('') as char, j}
					<SplitFlapChar value={char} delay={50 + j * 20} />
				{/each}
			</span>

			<span class="col-cars text-gray-400 text-xs">
				{#if dep.cars}
					{dep.cars}c
				{/if}
			</span>

			<span class="col-platform text-white">
				{#each padRight(dep.platform ?? '--', 4).split('') as char, j}
					<SplitFlapChar value={char} delay={100 + j * 20} />
				{/each}
			</span>

			<span class="col-arrival text-amber-300">
				{#each padRight(dep.arrivalTime ?? '-----', 5).split('') as char, j}
					<SplitFlapChar value={char} delay={110 + j * 25} />
				{/each}
			</span>

			<span
				class="col-occ {occupancyClass(dep.occupancy)}"
				title={dep.occupancy ? `${dep.occupancy}% full` : ''}
			>
				{occupancyIcon(dep.occupancy)}
			</span>

			<span class="col-status {statusClass(dep)}">
				{#each padRight(statusText(dep), 11).split('') as char, j}
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
		grid-template-columns: 5ch 10ch 3ch 4ch 5ch 2ch 11ch;
		gap: 6px;
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
	}

	.board-row.next-train {
		padding: 8px 0;
	}

	.col-time,
	.col-route,
	.col-cars,
	.col-platform,
	.col-arrival,
	.col-occ,
	.col-status {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		overflow: hidden;
	}

	.col-time {
		font-size: 0.95em;
	}
	.col-route {
		font-size: 0.85em;
	}
	.col-cars {
		font-size: 0.75em;
		justify-content: center;
	}
	.col-platform {
		font-size: 0.85em;
	}
	.col-arrival {
		font-size: 0.85em;
	}
	.col-occ {
		font-size: 0.85em;
		justify-content: center;
	}
	.col-status {
		font-size: 0.8em;
		letter-spacing: 0.05em;
	}

	.board-row.cancelled {
		opacity: 0.5;
		text-decoration: line-through;
	}

	@media (max-width: 480px) {
		.board-header,
		.board-row {
			grid-template-columns: 5ch 8ch 3ch 3ch 5ch 2ch 9ch;
			gap: 3px;
		}
	}
</style>
