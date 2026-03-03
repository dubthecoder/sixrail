<!-- web/src/routes/stations/+page.svelte -->
<script lang="ts">
	import { favorites } from '$lib/stores/favorites';

	let { data } = $props();
	let query = $state('');

	let filtered = $derived(
		query.length > 0
			? data.stops.filter((s: any) =>
					(s.StopName || s.Name || '').toLowerCase().includes(query.toLowerCase())
				)
			: data.stops
	);

	let favoriteStops = $derived(
		data.stops.filter((s: any) => $favorites.includes(s.StopCode || s.Code))
	);
</script>

<div class="space-y-6">
	<h1 class="text-2xl font-bold">Stations</h1>

	<input type="text" bind:value={query} placeholder="Search stations..."
		class="w-full px-4 py-2 border border-gray-300 rounded-lg" />

	{#if favoriteStops.length > 0 && query.length === 0}
		<div>
			<h2 class="text-lg font-medium mb-2">Favorites</h2>
			<div class="grid gap-2">
				{#each favoriteStops as stop}
					<a href="/departures/{(stop as any).StopCode || (stop as any).Code}"
						class="block bg-green-50 border border-green-200 rounded-lg p-3 hover:bg-green-100">
						{(stop as any).StopName || (stop as any).Name}
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<div>
		<h2 class="text-lg font-medium mb-2">All Stations</h2>
		<div class="grid gap-2">
			{#each filtered as stop}
				<a href="/departures/{(stop as any).StopCode || (stop as any).Code}"
					class="block bg-white border border-gray-200 rounded-lg p-3 hover:bg-gray-50">
					{(stop as any).StopName || (stop as any).Name}
				</a>
			{:else}
				<p class="text-gray-500">No stations found.</p>
			{/each}
		</div>
	</div>
</div>
