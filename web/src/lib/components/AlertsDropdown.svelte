<script lang="ts">
	import type { Alert } from '$lib/api';

	let { alerts = [] }: { alerts: Alert[] } = $props();
	let open = $state(false);
	let filter = $state('all');

	let filtered = $derived(filter === 'all' ? alerts : alerts.filter((a) => a.effect === filter));

	let effects = $derived([...new Set(alerts.map((a) => a.effect))].sort());
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
{#if open}
	<div class="fixed inset-0 z-10" onclick={() => (open = false)} onkeydown={() => {}}></div>
{/if}

<div class="absolute top-4 right-4 z-20">
	<button
		onclick={() => (open = !open)}
		class="relative bg-white rounded-lg shadow-lg px-3 py-2.5 border border-gray-200 hover:bg-gray-50 cursor-pointer"
	>
		<svg
			class="w-5 h-5 text-gray-700 inline-block"
			fill="none"
			viewBox="0 0 24 24"
			stroke="currentColor"
			stroke-width="2"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
			/>
		</svg>
		{#if alerts.length > 0}
			<span
				class="absolute -top-1.5 -right-1.5 bg-red-500 text-white text-xs font-bold rounded-full w-5 h-5 flex items-center justify-center"
			>
				{alerts.length > 99 ? '99+' : alerts.length}
			</span>
		{/if}
	</button>

	{#if open}
		<div
			class="absolute right-0 top-full mt-2 w-[90vw] max-w-sm bg-white rounded-lg shadow-xl border border-gray-200 max-h-[70vh] overflow-y-auto"
		>
			<div class="sticky top-0 bg-white border-b border-gray-200 p-3">
				<h3 class="font-semibold text-gray-900 mb-2">Service Alerts</h3>
				<div class="flex gap-1.5 flex-wrap">
					<button
						onclick={() => (filter = 'all')}
						class="px-2 py-0.5 rounded text-xs {filter === 'all'
							? 'bg-green-700 text-white'
							: 'bg-gray-100 text-gray-700'}"
					>
						All
					</button>
					{#each effects as effect}
						<button
							onclick={() => (filter = effect)}
							class="px-2 py-0.5 rounded text-xs {filter === effect
								? 'bg-red-600 text-white'
								: 'bg-gray-100 text-gray-700'}"
						>
							{effect.replace(/_/g, ' ')}
						</button>
					{/each}
				</div>
			</div>

			<div class="p-3 space-y-2">
				{#each filtered as alert (alert.id)}
					<div class="bg-red-50 border-l-4 border-red-500 p-3 rounded">
						<p class="font-medium text-red-900 text-sm">{alert.headline || 'Service disruption'}</p>
						{#if alert.description}
							<p class="text-xs text-red-800 mt-1">{alert.description}</p>
						{/if}
						{#if alert.routeNames?.length}
							<p class="text-xs text-red-700 mt-1">Routes: {alert.routeNames.join(', ')}</p>
						{/if}
						{#if alert.url}
							<a
								href={alert.url}
								target="_blank"
								rel="noopener"
								class="text-xs text-red-700 underline mt-1 block">More info</a
							>
						{/if}
					</div>
				{:else}
					<p class="text-gray-500 py-4 text-center text-sm">No active alerts.</p>
				{/each}
			</div>
		</div>
	{/if}
</div>
