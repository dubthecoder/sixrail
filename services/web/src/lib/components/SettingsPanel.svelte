<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { commute } from '$lib/stores/commute';
	import type { CommuteTrip } from '$lib/stores/commute';
	import type { Stop } from '$lib/api';
	import { track } from '$lib/track';
	import StationSearchInput from './StationSearchInput.svelte';

	let { stops, onClose }: { stops: Stop[]; onClose: () => void } = $props();

	let panelEl: HTMLDivElement;
	let previouslyFocused: HTMLElement | null = null;

	onMount(() => {
		previouslyFocused = document.activeElement as HTMLElement;
		// Focus the close button on open
		const closeBtn = panelEl?.querySelector<HTMLElement>('.close-btn');
		closeBtn?.focus();
	});

	onDestroy(() => {
		previouslyFocused?.focus();
	});

	function trapFocus(e: KeyboardEvent) {
		if (e.key !== 'Tab') return;
		const focusable = panelEl?.querySelectorAll<HTMLElement>(
			'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
		);
		if (!focusable || focusable.length === 0) return;
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	let commuteState = $state({
		toWork: null as CommuteTrip | null,
		toHome: null as CommuteTrip | null
	});
	const unsubCommute = commute.subscribe((s) => (commuteState = s));

	onDestroy(() => {
		unsubCommute();
	});

	function findStopByCode(code: string | undefined): Stop | null {
		if (!code) return null;
		return stops.find((s) => s.code === code || s.id === code) ?? null;
	}

	let workOriginQuery = $state(untrack(() => commuteState.toWork?.originName ?? ''));
	let workDestQuery = $state(untrack(() => commuteState.toWork?.destinationName ?? ''));
	let homeOriginQuery = $state(untrack(() => commuteState.toHome?.originName ?? ''));
	let homeDestQuery = $state(untrack(() => commuteState.toHome?.destinationName ?? ''));

	let workOrigin = $state<Stop | null>(
		untrack(() => findStopByCode(commuteState.toWork?.originCode))
	);
	let workDest = $state<Stop | null>(
		untrack(() => findStopByCode(commuteState.toWork?.destinationCode))
	);
	let homeOrigin = $state<Stop | null>(
		untrack(() => findStopByCode(commuteState.toHome?.originCode))
	);
	let homeDest = $state<Stop | null>(
		untrack(() => findStopByCode(commuteState.toHome?.destinationCode))
	);

	function tripFromStops(origin: Stop, dest: Stop): CommuteTrip {
		return {
			originCode: origin.code || origin.id,
			originName: origin.name,
			destinationCode: dest.code || dest.id,
			destinationName: dest.name
		};
	}

	function save() {
		if (workOrigin && workDest) {
			commute.setTrip('toWork', tripFromStops(workOrigin, workDest));
		}
		if (homeOrigin && homeDest) {
			commute.setTrip('toHome', tripFromStops(homeOrigin, homeDest));
		}
		track('settings-save');
		// Navigate to the updated trip URL
		if (workOrigin && workDest) {
			const from = workOrigin.code || workOrigin.id;
			const to = workDest.code || workDest.id;
			void goto(`/?from=${from}&to=${to}&dir=toWork`, { replaceState: true, noScroll: true });
		}
		onClose();
	}

	function clearAll() {
		commute.clear();
		track('settings-clear-all');
		void goto('/', { replaceState: true });
		onClose();
	}
</script>

<div
	class="settings-overlay"
	role="dialog"
	aria-modal="true"
	aria-label="Settings"
	tabindex="-1"
	onclick={onClose}
	onkeydown={(e) => e.key === 'Escape' && onClose()}
>
	<div
		class="settings-panel"
		role="presentation"
		bind:this={panelEl}
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => {
			e.stopPropagation();
			trapFocus(e);
		}}
	>
		<div class="panel-header">
			<h2 class="font-mono text-amber-400 uppercase tracking-widest text-sm">Settings</h2>
			<button class="close-btn font-mono text-gray-400 hover:text-white" onclick={onClose}>✕</button
			>
		</div>

		<div class="panel-body space-y-6">
			<!-- To Station trip -->
			<section>
				<h3 class="section-title">To Station</h3>
				<div class="space-y-2">
					<StationSearchInput
						{stops}
						bind:value={workOriginQuery}
						placeholder="From"
						onSelect={(s) => (workOrigin = s)}
					/>
					<StationSearchInput
						{stops}
						bind:value={workDestQuery}
						placeholder="To"
						onSelect={(s) => (workDest = s)}
					/>
				</div>
			</section>

			<!-- To Union trip -->
			<section>
				<h3 class="section-title">To Union</h3>
				<div class="space-y-2">
					<StationSearchInput
						{stops}
						bind:value={homeOriginQuery}
						placeholder="From"
						onSelect={(s) => (homeOrigin = s)}
					/>
					<StationSearchInput
						{stops}
						bind:value={homeDestQuery}
						placeholder="To"
						onSelect={(s) => (homeDest = s)}
					/>
				</div>
			</section>

			<!-- Actions -->
			<div class="flex gap-3">
				<button
					class="flex-1 bg-amber-400 text-black font-mono font-bold py-2 rounded text-sm"
					onclick={save}
				>
					SAVE
				</button>
				<button
					class="flex-1 bg-red-900 text-red-300 font-mono text-sm py-2 rounded border border-red-800"
					onclick={clearAll}
				>
					CLEAR ALL DATA
				</button>
			</div>
		</div>
	</div>
</div>

<style>
	.settings-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		display: flex;
		align-items: flex-end;
		justify-content: center;
		z-index: 100;
	}
	.settings-panel {
		background: var(--color-surface-raised);
		border-top: 1px solid var(--color-border);
		border-radius: 12px 12px 0 0;
		width: 100%;
		max-width: 480px;
		padding: 20px;
		max-height: 80vh;
		overflow-y: auto;
	}
	.panel-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}
	.section-title {
		color: var(--color-gray-500);
		font-family: monospace;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		margin-bottom: 8px;
	}
</style>
