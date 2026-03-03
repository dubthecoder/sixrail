<script lang="ts">
	import type { Alert } from '$lib/api';

	let { data } = $props();
	let filter = $state('all');

	let filtered = $derived(
		filter === 'all'
			? data.alerts
			: (data.alerts as Alert[]).filter((a) => a.effect === filter)
	);

	let effects = $derived(
		[...new Set((data.alerts as Alert[]).map((a) => a.effect))].sort()
	);
</script>

<div class="space-y-4">
	<h1 class="text-2xl font-bold">Service Alerts</h1>

	<div class="flex gap-2 flex-wrap">
		<button onclick={() => filter = 'all'} class="px-3 py-1 rounded {filter === 'all' ? 'bg-green-700 text-white' : 'bg-gray-200'}">All</button>
		{#each effects as effect}
			<button onclick={() => filter = effect} class="px-3 py-1 rounded {filter === effect ? 'bg-red-600 text-white' : 'bg-gray-200'}">{effect.replace(/_/g, ' ')}</button>
		{/each}
	</div>

	{#each filtered as alert (alert.id)}
		<div class="bg-red-50 border-l-4 border-red-500 p-4 rounded">
			<p class="font-medium text-red-900">{alert.headline || 'Service disruption'}</p>
			{#if alert.description}
				<p class="text-sm text-red-800 mt-1">{alert.description}</p>
			{/if}
			{#if alert.routeNames?.length}
				<p class="text-sm text-red-700 mt-1">Routes: {alert.routeNames.join(', ')}</p>
			{/if}
			{#if alert.url}
				<a href={alert.url} target="_blank" class="text-sm text-red-700 underline mt-1 block">More info</a>
			{/if}
		</div>
	{:else}
		<p class="text-gray-500 py-8 text-center">No active alerts.</p>
	{/each}
</div>
