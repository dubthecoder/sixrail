<script lang="ts">
	import MyCommute from '$lib/components/MyCommute.svelte';
	let { data } = $props();
</script>

<svelte:head>
	{#if data.urlTrip}
		<title
			>{data.urlTrip.fromName} to {data.urlTrip.toName} — GO Train Schedule & Live Countdown | Rail Six</title
		>
		<meta
			name="description"
			content="GO Train schedule from {data.urlTrip.fromName} to {data.urlTrip
				.toName}. Live departure times, platform assignments, and delay alerts. Free — no account, no ads, no tracking."
		/>
		<meta
			property="og:title"
			content="{data.urlTrip.fromName} to {data.urlTrip.toName} — GO Train Schedule | Rail Six"
		/>
		<meta
			property="og:description"
			content="GO Train schedule from {data.urlTrip.fromName} to {data.urlTrip
				.toName}. Live departure times, platform info, and delay alerts."
		/>
		<link
			rel="canonical"
			href="https://railsix.com/?from={data.urlTrip.fromCode}&to={data.urlTrip.toCode}&dir={data
				.urlTrip.dir}"
		/>
		<meta
			property="og:url"
			content="https://railsix.com/?from={data.urlTrip.fromCode}&to={data.urlTrip.toCode}&dir={data
				.urlTrip.dir}"
		/>
		<meta
			name="keywords"
			content="{data.urlTrip.fromName} to {data.urlTrip.toName} GO train schedule, {data.urlTrip
				.fromName} GO train times, GO train {data.urlTrip
				.fromName} to Union Station, GO Transit schedule, GO train real time, GTA commute tracker"
		/>
	{:else}
		<title>Real-Time GO Train Departures, Schedule & Commute Tracker — Rail Six</title>
		<meta
			name="description"
			content="Real-time GO Train departure times, platform info, and delay alerts for the GTA. Save your commute for a live countdown. Free — no account, no ads, no tracking."
		/>
		<meta
			property="og:title"
			content="Real-Time GO Train Departures, Schedule & Commute Tracker — Rail Six"
		/>
		<meta
			property="og:description"
			content="Real-time GO Train departure times, platform info, and delay alerts for the GTA. Free — no account, no ads, no tracking."
		/>
		<link rel="canonical" href="https://railsix.com/" />
		<meta property="og:url" content="https://railsix.com/" />
		<meta
			name="keywords"
			content="GO Train schedule, GO Transit schedule, GO train real time, GO train departures, GO train times, GO train platform, GO train countdown, GO train delay, Union Station GO, GTA commute tracker, Toronto commuter rail, GO train tracker, GO train no account"
		/>
	{/if}
	<meta name="robots" content="index, follow" />
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="Rail Six" />
	<meta property="og:image" content="https://railsix.com/og-image.png" />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta property="og:image:alt" content="Rail Six — GO Train departure board" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:site" content="@railsix" />
	<meta name="twitter:image" content="https://railsix.com/og-image.png" />
</svelte:head>

<MyCommute stops={data.stops} alerts={data.alerts} />
