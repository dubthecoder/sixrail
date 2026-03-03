<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchDepartures } from '$lib/api-client';
	import { favorites } from '$lib/stores/favorites';
	import DepartureBoard from './DepartureBoard.svelte';

	let { stopCode, stopName, onclose }: { stopCode: string; stopName: string; onclose: () => void } =
		$props();

	let departures = $state<any[]>([]);
	let loading = $state(true);
	let isFavorite = $derived($favorites.includes(stopCode));

	async function loadDepartures() {
		loading = true;
		departures = await fetchDepartures(stopCode);
		loading = false;
	}

	onMount(() => {
		loadDepartures();
		const interval = setInterval(loadDepartures, 30_000);
		return () => clearInterval(interval);
	});

	// Reload when stopCode changes
	$effect(() => {
		stopCode;
		loadDepartures();
	});
</script>

<!-- Mobile: bottom sheet. Desktop: side panel -->
<div
	class="
	fixed z-30 bg-white shadow-2xl overflow-hidden flex flex-col
	bottom-0 left-0 right-0 max-h-[60vh] rounded-t-2xl
	md:top-0 md:bottom-0 md:right-auto md:w-[380px] md:h-full md:max-h-full md:rounded-t-none
"
>
	<div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 shrink-0">
		<div class="flex items-center gap-2 min-w-0">
			<h2 class="font-bold text-gray-900 truncate">{stopName}</h2>
			<button
				onclick={() => favorites.toggle(stopCode)}
				class="text-xl shrink-0"
				aria-label={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
			>
				{isFavorite ? '★' : '☆'}
			</button>
		</div>
		<button
			onclick={onclose}
			class="text-gray-500 hover:text-gray-800 p-1 cursor-pointer"
			aria-label="Close"
		>
			<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
			</svg>
		</button>
	</div>

	<div class="flex-1 overflow-y-auto">
		{#if loading && departures.length === 0}
			<div class="flex items-center justify-center py-12">
				<div
					class="w-6 h-6 border-2 border-green-700 border-t-transparent rounded-full animate-spin"
				></div>
			</div>
		{:else}
			<DepartureBoard {departures} />
		{/if}
	</div>

	<div class="px-4 py-1.5 border-t border-gray-100 text-xs text-gray-400 shrink-0">
		Auto-refreshes every 30s
	</div>
</div>
