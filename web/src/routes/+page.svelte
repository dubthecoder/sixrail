<!-- web/src/routes/+page.svelte -->
<script lang="ts">
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import StationPicker from '$lib/components/StationPicker.svelte';

	let { data } = $props();

	$effect(() => {
		if (browser) {
			const saved = localStorage.getItem('defaultStation');
			if (saved) {
				goto(`/departures/${saved}`);
			}
		}
	});
</script>

<div class="flex flex-col items-center justify-center py-20">
	<h1 class="text-4xl font-bold text-gray-900 mb-2">GoPulse</h1>
	<p class="text-gray-600 mb-8">Real-time GO Transit tracking</p>
	<StationPicker stops={data.stops} />
</div>
