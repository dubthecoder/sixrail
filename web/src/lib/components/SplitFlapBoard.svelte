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
		if (d.status === 'CANCELLED') return 'CANCELLED  ';
		if (d.delayMinutes && d.delayMinutes > 0) return `DELAYED +${d.delayMinutes}MIN`;
		return 'ON TIME    ';
	}

	function statusClass(d: Departure): string {
		if (d.status === 'CANCELLED') return 'text-red-500';
		if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
		return 'text-green-400';
	}

	let rows = $derived(departures.slice(0, maxRows));
</script>

<div class="split-flap-board font-mono select-none" role="region" aria-label="Departure board">
	<!-- Header -->
	<div class="board-header">
		<span class="col-time text-gray-500 text-xs uppercase tracking-widest">Time</span>
		<span class="col-route text-gray-500 text-xs uppercase tracking-widest">Route</span>
		<span class="col-platform text-gray-500 text-xs uppercase tracking-widest">Plat</span>
		<span class="col-status text-gray-500 text-xs uppercase tracking-widest">Status</span>
	</div>

	<!-- Rows -->
	{#each rows as dep, i}
		<div class="board-row" class:next-train={i === 0}>
			<span class="col-time text-amber-400">
				{#each formatTime(dep.scheduledTime).split('') as char, j}
					<SplitFlapChar value={char} delay={j * 30} />
				{/each}
			</span>

			<span class="col-route text-white">
				{#each padRight(dep.line, 14).split('') as char, j}
					<SplitFlapChar value={char} delay={50 + j * 20} />
				{/each}
			</span>

			<span class="col-platform text-white">
				{#each padRight(dep.platform ?? '--', 4).split('') as char, j}
					<SplitFlapChar value={char} delay={100 + j * 20} />
				{/each}
			</span>

			<span class="col-status {statusClass(dep)}">
				{#each padRight(statusText(dep), 14).split('') as char, j}
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
		grid-template-columns: 5ch 15ch 5ch 15ch;
		gap: 8px;
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
		background: #1a1600;
		border-radius: 4px;
		padding: 8px 4px;
		font-size: 1.1em;
	}

	.col-time {
		font-size: 0.95em;
	}
	.col-route {
		font-size: 0.85em;
	}
	.col-platform {
		font-size: 0.85em;
	}
	.col-status {
		font-size: 0.8em;
		letter-spacing: 0.05em;
	}

	@media (max-width: 480px) {
		.board-header,
		.board-row {
			grid-template-columns: 5ch 12ch 4ch 12ch;
			gap: 4px;
		}
	}
</style>
