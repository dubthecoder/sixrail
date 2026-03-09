<script lang="ts">
	import { onDestroy } from 'svelte';
	import { formatCountdown } from '$lib/display';

	let { scheduledTime, size = 'large' }: { scheduledTime: string; size?: 'large' | 'small' } =
		$props();

	let display = $state('--:--');

	let interval: ReturnType<typeof setInterval> | undefined;

	$effect(() => {
		// Re-run whenever scheduledTime changes
		const st = scheduledTime;
		display = formatCountdown(st);
		if (interval) clearInterval(interval);
		interval = setInterval(() => {
			display = formatCountdown(st);
		}, 1000);
	});

	onDestroy(() => {
		if (interval) clearInterval(interval);
	});
</script>

<div
	class="countdown"
	class:countdown-small={size === 'small'}
	role="timer"
	aria-label="Time until departure"
>
	{#if size === 'large'}
		<span class="label text-gray-500 text-xs uppercase tracking-widest">Next train in</span>
	{/if}
	<span class="time font-mono text-amber-400 tabular-nums" class:time-small={size === 'small'}
		>{display}</span
	>
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

	.countdown-small {
		padding: 6px 14px;
		min-width: auto;
		background: transparent;
	}

	.time-small {
		font-size: 1rem;
		opacity: 0.7;
	}
</style>
