<script lang="ts">
	import type { Stop } from '$lib/api';
	import type { BuildInfo } from '$lib/build-info';
	import { commute } from '$lib/stores/commute';
	import { track } from '$lib/track';
	import BuildStamp from './BuildStamp.svelte';
	import StationSearchInput from './StationSearchInput.svelte';

	let { stops, buildInfo }: { stops: Stop[]; buildInfo: BuildInfo } = $props();

	const trainStops = $derived(stops.filter((s) => /\bGO$/.test(s.name)));

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
			originCode: workOrigin.code || workOrigin.id,
			originName: workOrigin.name,
			destinationCode: workDest.code || workDest.id,
			destinationName: workDest.name
		});
		commute.setTrip('toHome', {
			originCode: homeOrigin.code || homeOrigin.id,
			originName: homeOrigin.name,
			destinationCode: homeDest.code || homeDest.id,
			destinationName: homeDest.name
		});
		track('commute-setup-complete', {
			workOrigin: workOrigin.name,
			workDest: workDest.name
		});
	}
</script>

<div class="h-[calc(100dvh-60px)] bg-surface flex flex-col items-center justify-center p-6">
	<div class="w-full max-w-sm flex-shrink-0">
		<h1
			class="text-amber-400 text-xl font-bold font-mono tracking-widest uppercase text-center mb-2"
		>
			Rail Six
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
						stops={trainStops}
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
						stops={trainStops}
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
						stops={trainStops}
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
						stops={trainStops}
						bind:value={homeDestQuery}
						placeholder="Destination station"
						onSelect={(s) => {
							homeDest = s;
						}}
					/>
				</div>
				<div class="flex gap-3 mt-4">
					<button
						class="flex-1 bg-surface-input text-white font-mono py-3 rounded border border-border-input"
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

	<footer class="pt-4 pb-2 text-center max-w-sm flex-shrink">
		<p class="text-gray-500 text-[11px] font-mono leading-relaxed">
			Real-time GO Transit tracking with live departures, delay alerts, and countdown timers for
			your daily commute.
		</p>
		<p class="text-gray-500 text-[10px] font-mono mt-3 leading-relaxed text-center">
			Set up your commute by selecting your origin and destination stations for each direction. Once
			configured, you'll see live departure times, platform info, and delay updates. You can also
			visit the <a href="/departures" class="text-amber-400 hover:text-amber-300 transition-colors"
				>departure board</a
			> for a full split-flap display of upcoming trains at any station.
		</p>
		<p class="text-gray-500 text-[9px] font-mono mt-3 leading-relaxed">
			Not affiliated with Metrolinx or GO Transit. Schedule data may be inaccurate or delayed.
			Always confirm with official sources before travelling.
		</p>
		<p class="text-gray-500 text-[10px] tracking-wide font-mono mt-3">
			&copy; {new Date().getFullYear()}
			<a
				href="https://teclara.tech"
				target="_blank"
				rel="noopener noreferrer"
				class="hover:text-gray-400 transition-colors">Teclara Technologies Inc.</a
			>
		</p>
		<div class="mt-3 flex justify-center">
			<BuildStamp {buildInfo} />
		</div>
	</footer>
</div>
