<script lang="ts">
	import { fetchTripDetail } from '$lib/api-client';
	import type { TripDetail } from '$lib/api';

	let { tripId, onclose }: { tripId: string; onclose: () => void } = $props();

	let detail = $state<TripDetail | null>(null);
	let loading = $state(true);

	$effect(() => {
		loading = true;
		fetchTripDetail(tripId).then((d) => {
			detail = d;
			loading = false;
		});
	});
</script>

<div class="train-popup">
	{#if loading}
		<div class="p-3 text-center text-gray-400 text-sm">Loading...</div>
	{:else if detail}
		<!-- Header -->
		<div class="px-3 pt-3 pb-2">
			<div class="flex items-center justify-between gap-2">
				<div class="flex items-center gap-2">
					<div
						class="w-3 h-3 rounded-full flex-shrink-0"
						style="background-color: #{detail.routeColor || '15803d'}"
					></div>
					<span class="font-bold text-sm text-gray-900">{detail.routeName}</span>
				</div>
				<button class="text-gray-400 hover:text-gray-600 text-lg leading-none" onclick={onclose}
					>&times;</button
				>
			</div>
			<div class="text-xs text-gray-500 mt-0.5">#{detail.vehicleId}</div>
			{#if detail.delayMinutes > 0}
				<span
					class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700"
				>
					Delayed {detail.delayMinutes}m
				</span>
			{:else if detail.status === 'Cancelled'}
				<span
					class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700"
				>
					Cancelled
				</span>
			{:else}
				<span
					class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700"
				>
					On Time
				</span>
			{/if}
		</div>

		<!-- Route info -->
		<div class="px-3 py-2 border-t border-gray-100 space-y-1 text-xs">
			{#if detail.origin && detail.destination}
				<div class="flex items-center gap-1 text-gray-700">
					<span class="font-medium">{detail.origin}</span>
					<span class="text-gray-400">→</span>
					<span class="font-medium">{detail.destination}</span>
				</div>
			{/if}
			{#if detail.scheduleStart && detail.scheduleEnd}
				<div class="text-gray-500">
					Schedule: {detail.scheduleStart} – {detail.scheduleEnd}
				</div>
			{/if}
			{#if detail.currentStop}
				<div class="text-gray-500">
					Next: <span class="text-gray-700">{detail.currentStop}</span>
				</div>
			{/if}
		</div>

		<!-- Upcoming stops -->
		{#if detail.upcomingStops.length > 0}
			<div class="px-3 py-2 border-t border-gray-100">
				<div class="text-[10px] uppercase tracking-wider text-gray-400 font-semibold mb-1.5">
					Upcoming Stops ({detail.upcomingStops.length})
				</div>
				<div class="max-h-40 overflow-y-auto space-y-0.5">
					{#each detail.upcomingStops as stop}
						<div class="flex items-center justify-between text-xs py-0.5">
							<div class="flex items-center gap-1.5">
								<div class="w-1.5 h-1.5 rounded-full bg-gray-300 flex-shrink-0"></div>
								<span class="text-gray-700">{stop.name}</span>
								{#if stop.platform}
									<span class="text-[10px] font-medium bg-gray-100 text-gray-500 px-1 rounded">
										P{stop.platform}
									</span>
								{/if}
							</div>
							<div class="flex items-center gap-1 flex-shrink-0 ml-2">
								<span class="text-gray-500">{stop.time}</span>
								{#if stop.delayMinutes > 0}
									<span class="text-red-500 text-[10px]">(+{stop.delayMinutes}m)</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{:else}
		<div class="p-3 text-center text-gray-400 text-sm">Trip not found</div>
	{/if}
</div>

<style>
	.train-popup {
		min-width: 260px;
		max-width: 320px;
	}
</style>
