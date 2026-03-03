<!-- web/src/lib/components/StationPicker.svelte -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { defaultStation } from '$lib/stores/favorites';

	let { stops = [] }: { stops: any[] } = $props();
	let query = $state('');

	let filtered = $derived(
		query.length > 0
			? stops.filter((s: any) =>
					(s.StopName || s.Name || '').toLowerCase().includes(query.toLowerCase())
				)
			: stops
	);

	function selectStation(stopCode: string) {
		defaultStation.set(stopCode);
		goto(`/departures/${stopCode}`);
	}
</script>

<div class="w-full max-w-md mx-auto">
	<input
		type="text"
		bind:value={query}
		placeholder="Search for a station..."
		class="w-full px-4 py-3 border border-gray-300 rounded-lg text-lg focus:outline-none focus:ring-2 focus:ring-green-600"
	/>
	{#if query.length > 0 && filtered.length > 0}
		<ul class="mt-2 bg-white border border-gray-200 rounded-lg shadow-lg max-h-60 overflow-y-auto">
			{#each filtered.slice(0, 10) as stop}
				<li>
					<button
						onclick={() => selectStation(stop.StopCode || stop.Code)}
						class="w-full text-left px-4 py-2 hover:bg-green-50 cursor-pointer"
					>
						{stop.StopName || stop.Name}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
