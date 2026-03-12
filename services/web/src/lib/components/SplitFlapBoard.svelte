<script lang="ts">
	import SplitFlapChar from './SplitFlapChar.svelte';
	import type { Departure } from '$lib/api-client';
	import {
		padRight,
		padCenter,
		departureDisplayTime,
		isUpcomingDeparture,
		isWaiting,
		platformText,
		torontoNow
	} from '$lib/display';

	let {
		departures = [],
		maxRows = 3,
		tick = 0,
		fillEmpty = false
	}: {
		departures: Departure[];
		maxRows?: number;
		tick?: number;
		fillEmpty?: boolean;
	} = $props();

	function formatTime(t: string): string {
		return t.slice(0, 5);
	}

	function hasDeparted(d: Departure): boolean {
		tick; // re-evaluate each tick
		return !isUpcomingDeparture(d, torontoNow());
	}

	function isEmpty(d: Departure): boolean {
		return d === emptyDep;
	}

	function boardStatusText(d: Departure): string {
		if (isEmpty(d)) return '-------';
		if (d.isCancelled || d.status === 'Cancelled') return 'CANCEL';
		if (hasDeparted(d)) return 'DEPART';
		if (d.delayMinutes && d.delayMinutes > 0) return `DLY +${d.delayMinutes}`;
		const s = d.status?.toUpperCase() ?? '';
		if (s === 'PROCEED') return s;
		return 'ON TIME';
	}

	function boardStatusClass(d: Departure): string {
		if (isEmpty(d)) return 'text-gray-400';
		if (d.isCancelled || d.status === 'Cancelled') return 'text-red-500';
		if (hasDeparted(d)) return 'text-gray-400';
		if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
		const s = d.status?.toUpperCase() ?? '';
		if (s === 'PROCEED') return 'text-green-400';
		return 'text-green-400';
	}

	const TIME_DELAY_MS = 35;
	const ROUTE_DELAY_BASE_MS = 90;
	const ROUTE_DELAY_MS = 28;
	const CARS_DELAY_BASE_MS = 150;
	const CARS_DELAY_MS = 35;
	const PLATFORM_DELAY_BASE_MS = 210;
	const PLATFORM_DELAY_MS = 30;
	const ARRIVAL_DELAY_BASE_MS = 260;
	const ARRIVAL_DELAY_MS = 28;
	const STATUS_DELAY_BASE_MS = 320;
	const STATUS_DELAY_MS = 28;

	const emptyDep: Departure = {
		line: 'NONE',
		scheduledTime: '--:--',
		status: ''
	};

	let rows = $derived.by(() => {
		const real = departures.slice(0, maxRows);
		if (!fillEmpty) return real;
		const padded = [...real];
		while (padded.length < maxRows) padded.push(emptyDep);
		return padded;
	});
</script>

<div class="split-flap-board font-mono select-none" role="region" aria-label="Departure board">
	<!-- Header -->
	<div class="board-row board-header-row">
		<span class="col-time text-amber-400">TIME</span>
		<span class="col-route text-white">LINE</span>
		<span class="col-cars hide-mobile text-gray-400">CRS</span>
		<span class="col-platform text-white">PLAT</span>
		<span class="col-arrival hide-mobile text-amber-300">ARRV</span>
		<span class="col-status text-gray-400">STATUS</span>
	</div>

	<!-- Rows -->
	{#each rows as dep, i}
		<div
			class="board-row"
			class:next-train={i === 0}
			class:empty-row={isEmpty(dep)}
			class:cancelled={dep.isCancelled || dep.status === 'Cancelled'}
		>
			<span class="col-time text-amber-400">
				{#each formatTime(departureDisplayTime(dep)).split('') as char, j}
					<SplitFlapChar value={char} delay={j * TIME_DELAY_MS} />
				{/each}
			</span>

			<span class="col-route text-white">
				{#each padRight(dep.isExpress ? dep.line + ' X' : dep.line, 4).split('') as char, j}
					<SplitFlapChar value={char} delay={ROUTE_DELAY_BASE_MS + j * ROUTE_DELAY_MS} />
				{/each}
			</span>

			<span class="col-cars hide-mobile text-gray-400">
				{#each padRight(dep.cars && dep.cars !== '-' ? dep.cars + 'C' : '---', 3).split('') as char, j}
					<SplitFlapChar value={char} delay={CARS_DELAY_BASE_MS + j * CARS_DELAY_MS} />
				{/each}
			</span>

			<span class="col-platform text-white" class:text-amber-300={isWaiting(dep)}>
				{#each padCenter(platformText(dep), 5).split('') as char, j}
					<SplitFlapChar value={char} delay={PLATFORM_DELAY_BASE_MS + j * PLATFORM_DELAY_MS} />
				{/each}
			</span>

			<span class="col-arrival hide-mobile text-amber-300">
				{#each padRight(dep.arrivalTime ?? '-----', 5).split('') as char, j}
					<SplitFlapChar value={char} delay={ARRIVAL_DELAY_BASE_MS + j * ARRIVAL_DELAY_MS} />
				{/each}
			</span>

			<span class="col-status {boardStatusClass(dep)}">
				{#each padRight(boardStatusText(dep), 7).split('') as char, j}
					<SplitFlapChar value={char} delay={STATUS_DELAY_BASE_MS + j * STATUS_DELAY_MS} />
				{/each}
			</span>
		</div>
	{/each}

	{#if rows.length === 0}
		<div class="board-empty text-gray-400 font-mono text-sm py-8 text-center">
			NO DEPARTURES FOUND
		</div>
	{/if}
</div>

<style>
	.split-flap-board {
		background: var(--color-surface);
		border-radius: 8px;
		padding: 16px;
		width: 100%;
		margin: 0 auto;
		overflow: hidden;
	}

	.board-row {
		display: grid;
		grid-template-columns: 7fr 5fr 4fr 7fr 7fr 9fr;
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
		justify-content: center;
		overflow: hidden;
	}

	.board-row.cancelled {
		opacity: 0.5;
		text-decoration: line-through;
	}

	.board-row.empty-row {
		color: var(--color-muted);
	}

	@media (max-width: 480px) {
		.split-flap-board {
			padding: 8px;
			width: 100%;
			box-sizing: border-box;
		}

		.hide-mobile {
			display: none;
		}

		.board-row {
			grid-template-columns: 7fr 5fr 6fr 8fr;
			gap: 6px;
		}
	}
</style>
