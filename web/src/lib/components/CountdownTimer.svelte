<script lang="ts">
	import { onDestroy } from 'svelte';
	import { torontoNow } from '$lib/display';

	let { scheduledTime }: { scheduledTime: string } = $props();

	let display = $state('--:--');

	function computeCountdown(scheduled: string): string {
		const [h, m] = scheduled.split(':').map(Number);
		const toronto = torontoNow();
		let targetMs = toronto.todayAt(h, m);
		const nowMs = toronto.ms;
		if (targetMs <= nowMs) targetMs += 24 * 3600 * 1000;
		const diffMs = targetMs - nowMs;
		const mins = Math.floor(diffMs / 60000);
		const secs = Math.floor((diffMs % 60000) / 1000);
		return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
	}

	let interval: ReturnType<typeof setInterval> | undefined;

	$effect(() => {
		// Re-run whenever scheduledTime changes
		const st = scheduledTime;
		display = computeCountdown(st);
		if (interval) clearInterval(interval);
		interval = setInterval(() => {
			display = computeCountdown(st);
		}, 1000);
	});

	onDestroy(() => {
		if (interval) clearInterval(interval);
	});
</script>

<div class="countdown" role="timer" aria-label="Time until next departure">
	<span class="label text-gray-500 text-xs uppercase tracking-widest">Next train in</span>
	<span class="time font-mono text-amber-400 tabular-nums">{display}</span>
</div>

<style>
	.countdown {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		background: var(--color-surface-overlay);
		border-radius: 8px;
		padding: 12px 24px;
		min-width: 160px;
	}

	.time {
		font-size: 2rem;
		letter-spacing: 0.1em;
	}
</style>
