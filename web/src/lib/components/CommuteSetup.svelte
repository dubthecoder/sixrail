<script lang="ts">
	import type { Stop } from '$lib/api';
	import { commute } from '$lib/stores/commute';
	import StationSearchInput from './StationSearchInput.svelte';

	let { stops }: { stops: Stop[] } = $props();

	let step = $state<1 | 2>(1);
	let workOrigin = $state<Stop | null>(null);
	let workDest = $state<Stop | null>(null);
	let homeOrigin = $state<Stop | null>(null);
	let homeDest = $state<Stop | null>(null);

	let workOriginQuery = $state('');
	let workDestQuery = $state('');
	let homeOriginQuery = $state('');
	let homeDestQuery = $state('');

	function goToStep2() {
		if (!workOrigin || !workDest) return;
		homeOrigin = workDest;
		homeDest = workOrigin;
		homeOriginQuery = workDest.name;
		homeDestQuery = workOrigin.name;
		step = 2;
	}

	$effect(() => {
		if (workOrigin && workOriginQuery !== workOrigin.name) workOrigin = null;
	});
	$effect(() => {
		if (workDest && workDestQuery !== workDest.name) workDest = null;
	});
	$effect(() => {
		if (homeOrigin && homeOriginQuery !== homeOrigin.name) homeOrigin = null;
	});
	$effect(() => {
		if (homeDest && homeDestQuery !== homeDest.name) homeDest = null;
	});

	function save() {
		if (!workOrigin || !workDest || !homeOrigin || !homeDest) return;
		commute.setTrip('toWork', {
			originCode: workOrigin.code,
			originName: workOrigin.name,
			destinationCode: workDest.code,
			destinationName: workDest.name
		});
		commute.setTrip('toHome', {
			originCode: homeOrigin.code,
			originName: homeOrigin.name,
			destinationCode: homeDest.code,
			destinationName: homeDest.name
		});
	}
</script>

<div class="min-h-screen bg-[#111] flex items-center justify-center p-6">
	<div class="w-full max-w-sm">
		<h1
			class="text-amber-400 text-xl font-bold font-mono tracking-widest uppercase text-center mb-2"
		>
			Six Rail
		</h1>
		<p class="text-gray-400 text-sm font-mono text-center mb-8">Set up your commute</p>

		<div class="flex items-center justify-center gap-4 mb-8 font-mono text-xs">
			<span class={step === 1 ? 'text-amber-400' : 'text-gray-600'}>1 TO WORK</span>
			<span class="text-gray-700">→</span>
			<span class={step === 2 ? 'text-amber-400' : 'text-gray-600'}>2 TO HOME</span>
		</div>

		{#if step === 1}
			<div class="space-y-4">
				<div>
					<p class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">From</p>
					<StationSearchInput
						{stops}
						bind:value={workOriginQuery}
						placeholder="Origin station"
						onSelect={(s) => {
							workOrigin = s;
						}}
					/>
				</div>
				<div>
					<p class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">To</p>
					<StationSearchInput
						{stops}
						bind:value={workDestQuery}
						placeholder="Destination station"
						onSelect={(s) => {
							workDest = s;
						}}
					/>
				</div>
				<button
					class="w-full mt-4 bg-amber-400 text-black font-mono font-bold py-3 rounded disabled:opacity-40 disabled:cursor-not-allowed"
					disabled={!workOrigin || !workDest}
					onclick={goToStep2}
				>
					NEXT →
				</button>
			</div>
		{:else}
			<div class="space-y-4">
				<p class="text-gray-500 text-xs font-mono mb-2">
					Pre-filled as your reverse trip. Adjust if needed.
				</p>
				<div>
					<p class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">From</p>
					<StationSearchInput
						{stops}
						bind:value={homeOriginQuery}
						placeholder="Origin station"
						onSelect={(s) => {
							homeOrigin = s;
						}}
					/>
				</div>
				<div>
					<p class="block text-gray-500 text-xs font-mono uppercase tracking-wider mb-1">To</p>
					<StationSearchInput
						{stops}
						bind:value={homeDestQuery}
						placeholder="Destination station"
						onSelect={(s) => {
							homeDest = s;
						}}
					/>
				</div>
				<div class="flex gap-3 mt-4">
					<button
						class="flex-1 bg-[#1e1e1e] text-white font-mono py-3 rounded border border-[#333]"
						onclick={() => (step = 1)}
					>
						← BACK
					</button>
					<button
						class="flex-1 bg-amber-400 text-black font-mono font-bold py-3 px-6 rounded disabled:opacity-40 disabled:cursor-not-allowed"
						disabled={!homeOrigin || !homeDest}
						onclick={save}
					>
						START →
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>
