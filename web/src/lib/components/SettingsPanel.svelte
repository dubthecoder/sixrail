<script lang="ts">
	import { onDestroy, untrack } from 'svelte';
	import { commute, notificationPrefs } from '$lib/stores/commute';
	import type { CommuteTrip } from '$lib/stores/commute';
	import type { Stop } from '$lib/api';
	import StationSearchInput from './StationSearchInput.svelte';

	let { stops, onClose }: { stops: Stop[]; onClose: () => void } = $props();

	let commuteState = $state({
		toWork: null as CommuteTrip | null,
		toHome: null as CommuteTrip | null
	});
	let notifState = $state({ enabled: false, thresholdMinutes: 5 as 5 | 10 | 15 });

	const unsubCommute = commute.subscribe((s) => (commuteState = s));
	const unsubNotif = notificationPrefs.subscribe((s) => (notifState = s));

	onDestroy(() => {
		unsubCommute();
		unsubNotif();
	});

	let workOriginQuery = $state(untrack(() => commuteState.toWork?.originName ?? ''));
	let workDestQuery = $state(untrack(() => commuteState.toWork?.destinationName ?? ''));
	let homeOriginQuery = $state(untrack(() => commuteState.toHome?.originName ?? ''));
	let homeDestQuery = $state(untrack(() => commuteState.toHome?.destinationName ?? ''));

	let workOrigin = $state<Stop | null>(null);
	let workDest = $state<Stop | null>(null);
	let homeOrigin = $state<Stop | null>(null);
	let homeDest = $state<Stop | null>(null);

	function save() {
		if (workOrigin && workDest) {
			commute.setTrip('toWork', {
				originCode: workOrigin.code || workOrigin.id,
				originName: workOrigin.name,
				destinationCode: workDest.code || workDest.id,
				destinationName: workDest.name
			});
		}
		if (homeOrigin && homeDest) {
			commute.setTrip('toHome', {
				originCode: homeOrigin.code || homeOrigin.id,
				originName: homeOrigin.name,
				destinationCode: homeDest.code || homeDest.id,
				destinationName: homeDest.name
			});
		}
		onClose();
	}

	function clearAll() {
		commute.clear();
		if (typeof localStorage !== 'undefined') {
			localStorage.removeItem('notificationPrefs');
		}
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
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
	>
		<div class="panel-header">
			<h2 class="font-mono text-amber-400 uppercase tracking-widest text-sm">Settings</h2>
			<button class="close-btn font-mono text-gray-500 hover:text-white" onclick={onClose}>✕</button
			>
		</div>

		<div class="panel-body space-y-6">
			<!-- To Work trip -->
			<section>
				<h3 class="section-title">To Work</h3>
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

			<!-- To Home trip -->
			<section>
				<h3 class="section-title">To Home</h3>
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

			<!-- Notifications -->
			<section>
				<h3 class="section-title">Notifications</h3>
				<label class="flex items-center gap-3 cursor-pointer">
					<input
						type="checkbox"
						checked={notifState.enabled}
						onchange={(e) => notificationPrefs.setEnabled((e.target as HTMLInputElement).checked)}
						class="accent-amber-400"
					/>
					<span class="text-white text-sm font-mono">Notify me if delayed</span>
				</label>
				{#if notifState.enabled}
					<div class="flex gap-2 mt-2">
						{#each [5, 10, 15] as mins}
							<button
								class="threshold-btn font-mono text-xs py-1 px-3 rounded border"
								class:active={notifState.thresholdMinutes === mins}
								onclick={() => notificationPrefs.setThreshold(mins as 5 | 10 | 15)}
							>
								+{mins}m
							</button>
						{/each}
					</div>
				{/if}
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
		background: #161616;
		border-top: 1px solid #2a2a2a;
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
		color: #6b7280;
		font-family: monospace;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		margin-bottom: 8px;
	}
	.threshold-btn {
		border-color: #333;
		color: #999;
		background: #1e1e1e;
	}
	.threshold-btn.active {
		border-color: #f5a623;
		color: #f5a623;
		background: #1a1200;
	}
</style>
