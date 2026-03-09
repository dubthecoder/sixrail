<script lang="ts">
	import MyCommute from '$lib/components/MyCommute.svelte';
	let { data } = $props();
</script>

<svelte:head>
	<title>Rail Six — GO Train Schedule & Real-Time Toronto Commute Tracker</title>
	<meta
		name="description"
		content="Track your GO Train commute in real time. Live departure times, delays, and platform info for all GO Transit stations across the Greater Toronto Area."
	/>
	<meta
		name="keywords"
		content="GO Train, GO Transit, Toronto train schedule, GO Train times, GTA commute tracker, real-time train tracker, GO Transit departures, Union Station GO, Toronto commuter rail"
	/>
	<meta name="author" content="Teclara Technologies Inc" />
	<meta
		property="og:title"
		content="Rail Six — GO Train Schedule & Real-Time Toronto Commute Tracker"
	/>
	<meta
		property="og:description"
		content="Track your GO Train commute in real time. Live departure times, delays, and platform info for all GO Transit stations across the Greater Toronto Area."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:url" content="https://railsix.com/" />
	<meta property="og:image" content="https://railsix.com/train.png" />
	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content="Rail Six — GO Train Schedule & Toronto Commute Tracker" />
	<meta
		name="twitter:description"
		content="Live GO Train departure times, delays, and platform info for your Toronto commute."
	/>
	<meta name="twitter:image" content="https://railsix.com/train.png" />
</svelte:head>

<MyCommute stops={data.stops} alerts={data.alerts} buildInfo={data.buildInfo} />
