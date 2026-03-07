<script lang="ts">
	import CommuteDashboard from '$lib/components/CommuteDashboard.svelte';
	let { data } = $props();
</script>

<CommuteDashboard stops={data.stops} alerts={data.alerts} />
