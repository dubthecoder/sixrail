<script lang="ts">
	import { formatCountdown } from '$lib/display';

	let {
		scheduledTime,
		originalScheduledTime,
		size = 'large',
		empty = false
	}: {
		scheduledTime: string;
		originalScheduledTime?: string;
		size?: 'large' | 'small';
		empty?: boolean;
	} = $props();

	let display = $state('--:--');
	let originalDisplay = $state('');

	function tick() {
		if (empty) {
			display = '00:00';
			originalDisplay = '';
			return;
		}
		display = formatCountdown(scheduledTime);
		originalDisplay = originalScheduledTime ? formatCountdown(originalScheduledTime) : '';
	}

	$effect(() => {
		tick();
		const interval = setInterval(tick, 1000);
		return () => clearInterval(interval);
	});
</script>

<div
	class="countdown"
	class:countdown-small={size === 'small'}
	role="timer"
	aria-label="Time until departure"
>
	{#if size === 'large'}
		<span class="label text-gray-400 text-xs uppercase tracking-widest">Next train in</span>
	{/if}
	<span
		class="time font-mono tabular-nums"
		class:text-amber-400={display !== '00:00'}
		class:text-gray-400={display === '00:00'}
		class:time-small={size === 'small'}>{display}</span
	>
	{#if size === 'large' && originalDisplay}
		<div class="scheduled-line">
			<span class="text-amber-400/60 text-xs uppercase tracking-wider">Scheduled</span>
			<span class="font-mono text-amber-400/60 tabular-nums text-sm">{originalDisplay}</span>
		</div>
	{/if}
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
		min-height: auto;
		justify-content: center;
	}

	.time {
		font-size: 2rem;
		letter-spacing: 0.1em;
	}

	.scheduled-line {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 2px;
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
