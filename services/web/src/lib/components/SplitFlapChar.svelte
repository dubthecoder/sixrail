<script lang="ts">
	import { untrack } from 'svelte';

	let { value = ' ', delay = 0 }: { value: string; delay?: number } = $props();

	let displayValue = $state(untrack(() => value));
	let isFlipping = $state(false);
	let flipFrom = $state(untrack(() => value));
	let flipTo = $state(untrack(() => value));

	const CHARS = ' ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789:+-.';
	const FLIP_DURATION_MS = 120;
	const FLIP_SETTLE_MS = 30;

	function getNextChar(current: string): string {
		const idx = CHARS.indexOf(current.toUpperCase());
		return CHARS[(idx + 1) % CHARS.length];
	}

	let flipGeneration = 0;
	let hasFlippedOnce = false;

	async function flipToTarget(target: string) {
		const gen = ++flipGeneration;
		const targetUpper = target.toUpperCase();
		if (displayValue.toUpperCase() === targetUpper) return;

		await new Promise((r) => setTimeout(r, delay));
		if (gen !== flipGeneration) return;

		// First flip: single direct flip to target (skip intermediate chars)
		if (!hasFlippedOnce) {
			hasFlippedOnce = true;
			flipFrom = displayValue.toUpperCase();
			flipTo = targetUpper;
			isFlipping = true;
			await new Promise((r) => setTimeout(r, FLIP_DURATION_MS));
			if (gen !== flipGeneration) {
				isFlipping = false;
				return;
			}
			isFlipping = false;
			displayValue = targetUpper;
			return;
		}

		let current = displayValue.toUpperCase();
		let steps = 0;
		while (current !== targetUpper && steps < CHARS.length) {
			if (gen !== flipGeneration) {
				isFlipping = false;
				return;
			}
			const next = getNextChar(current);
			steps++;
			flipFrom = current;
			flipTo = next;
			isFlipping = true;
			await new Promise((r) => setTimeout(r, FLIP_DURATION_MS));
			if (gen !== flipGeneration) {
				isFlipping = false;
				return;
			}
			isFlipping = false;
			displayValue = next;
			current = next;
			await new Promise((r) => setTimeout(r, FLIP_SETTLE_MS));
		}

		// Target char not in CHARS — snap directly
		if (current !== targetUpper) {
			displayValue = targetUpper;
			flipTo = targetUpper;
		}
	}

	$effect(() => {
		const target = value;
		let cancelled = false;

		queueMicrotask(() => {
			if (!cancelled) {
				void flipToTarget(target);
			}
		});

		return () => {
			cancelled = true;
			flipGeneration += 1;
		};
	});
</script>

<span class="split-flap-char" style="--flip-duration: {FLIP_DURATION_MS}ms">
	<!-- New char top half — revealed as the flap falls away -->
	<span class="tile top">
		<span class="char">{isFlipping ? flipTo : displayValue}</span>
	</span>

	<!-- Bottom half — shows current settled char -->
	<span class="tile bottom">
		<span class="char">{displayValue}</span>
	</span>

	{#if isFlipping}
		<span class="flap">
			<!-- Front face: old char top half (visible 0–90°) -->
			<span class="flap-face front">
				<span class="char">{flipFrom}</span>
			</span>
			<!-- Back face: new char bottom half (visible 90–180°) -->
			<span class="flap-face back">
				<span class="char">{flipTo}</span>
			</span>
		</span>
		<!-- Shadow cast on bottom half as flap falls -->
		<span class="bottom-shadow"></span>
	{/if}

	<!-- Center hinge line -->
	<span class="hinge"></span>
</span>

<style>
	.split-flap-char {
		position: relative;
		display: inline-block;
		width: 1.3ch;
		height: 1.4em;
		border-radius: 2px;
		overflow: hidden;
		margin: 0 0.05em;
		font-variant-numeric: tabular-nums;
		perspective: 150px;
	}

	.tile {
		position: absolute;
		width: 100%;
		height: 50%;
		overflow: hidden;
	}

	.tile.top {
		top: 0;
		background: var(--color-surface-input);
	}

	.tile.bottom {
		bottom: 0;
		background: var(--color-flap-dark);
	}

	/* .char spans 200% of the half-tile so the glyph center sits at the fold */
	.char {
		position: absolute;
		width: 100%;
		height: 200%;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.tile.top .char {
		top: 0;
	}

	.tile.bottom .char {
		bottom: 0;
	}

	/* The flap: hinged at its bottom edge (the center line of the char) */
	.flap {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 50%;
		transform-origin: bottom center;
		transform-style: preserve-3d;
		animation: flap-down var(--flip-duration) ease-in forwards;
		z-index: 2;
	}

	.flap-face {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		overflow: hidden;
		backface-visibility: hidden;
	}

	.flap-face.front {
		background: var(--color-surface-input);
	}

	.flap-face.front .char {
		top: 0;
	}

	.flap-face.back {
		background: var(--color-flap-dark);
		transform: rotateX(180deg);
	}

	.flap-face.back .char {
		bottom: 0;
	}

	/* Shadow on the bottom tile that fades in as the flap falls */
	.bottom-shadow {
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 50%;
		background: rgba(0, 0, 0, 0.4);
		animation: shadow-fade var(--flip-duration) ease-in forwards;
		z-index: 1;
		pointer-events: none;
	}

	/* Center hinge / divider */
	.hinge {
		position: absolute;
		top: 50%;
		left: 0;
		width: 100%;
		height: 1px;
		background: rgba(0, 0, 0, 0.7);
		z-index: 3;
		transform: translateY(-0.5px);
	}

	@keyframes flap-down {
		0% {
			transform: rotateX(0deg);
		}
		80% {
			transform: rotateX(-180deg);
		}
		90% {
			transform: rotateX(-174deg);
		}
		100% {
			transform: rotateX(-180deg);
		}
	}

	@keyframes shadow-fade {
		0% {
			opacity: 1;
		}
		80% {
			opacity: 0;
		}
		100% {
			opacity: 0;
		}
	}
</style>
