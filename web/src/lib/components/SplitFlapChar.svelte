<script lang="ts">
	import { untrack } from 'svelte';

	let { value = ' ', delay = 0 }: { value: string; delay?: number } = $props();

	// Use untrack to read the initial prop value non-reactively for $state initialization.
	// The $effect below drives all subsequent updates reactively via flipTo(value).
	let displayValue = $state(untrack(() => value));
	let isFlipping = $state(false);
	let topValue = $state(untrack(() => value));
	let bottomValue = $state(untrack(() => value));

	const CHARS = ' ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789:+-.';

	function getNextChar(current: string): string {
		const idx = CHARS.indexOf(current.toUpperCase());
		return CHARS[(idx + 1) % CHARS.length];
	}

	let flipGeneration = 0;

	async function flipTo(target: string) {
		const gen = ++flipGeneration;
		const targetUpper = target.toUpperCase();
		if (displayValue.toUpperCase() === targetUpper) return;

		await new Promise((r) => setTimeout(r, delay));
		if (gen !== flipGeneration) return; // cancelled

		let current = displayValue.toUpperCase();
		while (current !== targetUpper) {
			if (gen !== flipGeneration) return; // cancelled
			current = getNextChar(current);
			topValue = current;
			isFlipping = true;
			await new Promise((r) => setTimeout(r, 40));
			if (gen !== flipGeneration) return;
			isFlipping = false;
			bottomValue = current;
			displayValue = current;
			await new Promise((r) => setTimeout(r, 10));
		}
	}

	$effect(() => {
		flipTo(value);
	});
</script>

<span class="split-flap-char" style="--flip-delay: {delay}ms">
	<span class="tile top">{topValue}</span>
	<span class="tile bottom">{bottomValue}</span>
	{#if isFlipping}
		<span class="tile flipping">{topValue}</span>
	{/if}
</span>

<style>
	.split-flap-char {
		position: relative;
		display: inline-flex;
		flex-direction: column;
		width: 1ch;
		height: 1.4em;
		background: #1e1e1e;
		border-radius: 2px;
		overflow: hidden;
		box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.5);
		margin: 0 1px;
		font-variant-numeric: tabular-nums;
	}

	.tile {
		position: absolute;
		width: 100%;
		height: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		line-height: 2;
	}

	.tile.top {
		top: 0;
		align-items: flex-end;
		background: #1e1e1e;
		border-bottom: 1px solid #000;
	}

	.tile.bottom {
		bottom: 0;
		align-items: flex-start;
		background: #181818;
	}

	.tile.flipping {
		top: 0;
		height: 100%;
		animation: flip 80ms linear forwards;
		transform-origin: center;
		background: #1e1e1e;
		z-index: 2;
	}

	@keyframes flip {
		0% {
			transform: rotateX(0deg);
		}
		50% {
			transform: rotateX(-90deg);
		}
		100% {
			transform: rotateX(0deg);
			opacity: 0;
		}
	}
</style>
