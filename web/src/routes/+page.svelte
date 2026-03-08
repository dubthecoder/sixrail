<script lang="ts">
	import MyCommute from '$lib/components/MyCommute.svelte';
	let { data } = $props();
</script>

<svelte:head>
	<title>Six Rail — GO Transit Commute Tracker</title>
	<meta
		name="description"
		content="Real-time GO Transit departure board and commute tracker. Track trains, delays, and platform info for your daily commute."
	/>
	<meta name="author" content="Teclara Technologies Inc" />
	<meta property="og:title" content="Six Rail — GO Transit Commute Tracker" />
	<meta
		property="og:description"
		content="Real-time GO Transit departure board and commute tracker. Track trains, delays, and platform info for your daily commute."
	/>
	<meta property="og:type" content="website" />
	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content="Six Rail — GO Transit Commute Tracker" />
	<meta
		name="twitter:description"
		content="Real-time GO Transit departure board and commute tracker."
	/>
</svelte:head>

<MyCommute stops={data.stops} alerts={data.alerts} />
