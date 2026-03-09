<script lang="ts">
	import SplitFlapChar from './SplitFlapChar.svelte';
	import type { Departure } from '$lib/api-client';
	import { padRight, padCenter, compactPlatform, departureDisplayTime } from '$lib/display';

	import { onMount } from 'svelte';

	let {
		departures = [],
		maxRows = 3
	}: {
		departures: Departure[];
		maxRows?: number;
	} = $props();

	let isMobile = $state(false);

	onMount(() => {
		const mq = window.matchMedia('(max-width: 480px)');
		isMobile = mq.matches;
		mq.addEventListener('change', (e) => (isMobile = e.matches));
	});

	function formatTime(t: string): string {
		return t.slice(0, 5);
	}

	function boardStatusText(d: Departure): string {
		if (d.isCancelled || d.status === 'Cancelled') return 'CANCELLED';
		if (d.delayMinutes && d.delayMinutes > 0) return `DLY +${d.delayMinutes} MIN`;
		return 'ON TIME';
	}

	function boardStatusClass(d: Departure): string {
		if (d.isCancelled || d.status === 'Cancelled') return 'text-red-500';
		if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
		return 'text-green-400';
	}

	let rows = $derived(departures.slice(0, maxRows));
</script>

<div class="split-flap-board font-mono select-none" role="region" aria-label="Departure board">
	<!-- Header -->
	<div class="board-row board-header-row">
		<span class="col-time text-amber-400">TIME</span>
		<span class="col-route text-white">LINE</span>
		<span class="col-cars text-gray-400">CRS</span>
		<span class="col-platform text-white">PLT</span>
		<span class="col-arrival text-amber-300">ARRV</span>
		<span class="col-status text-gray-400">STATUS</span>
	</div>

	<!-- Rows -->
	{#each rows as dep, i}
		<div
			class="board-row"
			class:next-train={i === 0}
			class:cancelled={dep.isCancelled || dep.status === 'Cancelled'}
		>
			<span class="col-time text-amber-400">
				{#each formatTime(departureDisplayTime(dep)).split('') as char, j}
					<SplitFlapChar value={char} delay={j * 15} />
				{/each}
			</span>

			<span class="col-route text-white">
				{#each padRight(dep.isExpress ? dep.line + ' E' : dep.line, isMobile ? 4 : 6).split('') as char, j}
					<SplitFlapChar value={char} delay={20 + j * 10} />
				{/each}
			</span>

			<span class="col-cars text-gray-400">
				{#each padRight(dep.cars ? dep.cars + 'C' : '---', 3).split('') as char, j}
					<SplitFlapChar value={char} delay={40 + j * 15} />
				{/each}
			</span>

			<span class="col-platform text-white">
				{#each padCenter(compactPlatform(dep.platform ?? 'WAIT'), 5).split('') as char, j}
					<SplitFlapChar value={char} delay={50 + j * 12} />
				{/each}
			</span>

			<span class="col-arrival text-amber-300">
				{#each padRight(dep.arrivalTime ?? '-----', 5).split('') as char, j}
					<SplitFlapChar value={char} delay={60 + j * 10} />
				{/each}
			</span>

			<span class="col-status {boardStatusClass(dep)}">
				{#each padRight(boardStatusText(dep), 11).split('') as char, j}
					<SplitFlapChar value={char} delay={70 + j * 10} />
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
		background: var(--color-surface);
		border-radius: 8px;
		padding: 16px;
		width: fit-content;
		margin: 0 auto;
		overflow: hidden;
	}

	.board-row {
		display: grid;
		grid-template-columns: 8ch 8ch 5ch 8ch 8ch 15ch;
		gap: 8px;
		align-items: center;
		padding: 6px 0;
	}

	.board-header-row {
		border-bottom: 1px solid var(--color-border-header);
		margin-bottom: 8px;
		padding-bottom: 8px;
	}

	.board-row {
		border-bottom: 1px solid var(--color-border-subtle);
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
	.col-status {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		overflow: hidden;
	}

	.col-time {
		font-size: 1.1em;
	}
	.col-route {
		font-size: 1em;
	}
	.col-cars {
		font-size: 0.9em;
		justify-content: center;
	}
	.col-platform {
		font-size: 1em;
		justify-content: center;
	}
	.col-arrival {
		font-size: 1em;
	}
	.col-status {
		font-size: 0.95em;
		letter-spacing: 0.05em;
	}

	.board-row.cancelled {
		opacity: 0.5;
		text-decoration: line-through;
	}

	@media (max-width: 480px) {
		.split-flap-board {
			font-size: 11px;
			padding: 8px;
			width: 100%;
			box-sizing: border-box;
		}

		.board-row {
			grid-template-columns: 6fr 5fr 4fr 6fr 6fr 9fr;
			gap: 2px;
		}
	}
</style>
