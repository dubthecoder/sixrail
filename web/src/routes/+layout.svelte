<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { commute, notificationPrefs } from '$lib/stores/commute';

	let { children } = $props();
	let path = $derived($page.url.pathname);

	let webAppJsonLd = $derived(
		JSON.stringify({
			'@context': 'https://schema.org',
			'@type': 'WebApplication',
			name: 'Rail Six',
			url: 'https://railsix.com',
			description:
				'Real-time GO Transit departure board and commute tracker. Track trains, delays, and platform info for your daily commute.',
			applicationCategory: 'TravelApplication',
			operatingSystem: 'Web',
			offers: { '@type': 'Offer', price: '0', priceCurrency: 'CAD' },
			author: { '@type': 'Organization', name: 'Teclara Technologies Inc' }
		})
	);

	let breadcrumbJsonLd = $derived(
		JSON.stringify({
			'@context': 'https://schema.org',
			'@type': 'BreadcrumbList',
			itemListElement: [
				{ '@type': 'ListItem', position: 1, name: 'Home', item: 'https://railsix.com/' },
				...(path !== '/'
					? [
							{
								'@type': 'ListItem',
								position: 2,
								name:
									path === '/departures'
										? 'Departures'
										: path === '/board'
											? 'Board'
											: path.slice(1),
								item: `https://railsix.com${path}`
							}
						]
					: [])
			]
		})
	);

	onMount(() => {
		if (!browser || !('serviceWorker' in navigator)) return;

		let swReg: ServiceWorkerRegistration | null = null;

		function sendPrefs(cs: any, np: any) {
			swReg?.active?.postMessage({ type: 'UPDATE_PREFS', commuteState: cs, notifPrefs: np });
		}

		navigator.serviceWorker.register('/sw.js').then((reg) => {
			swReg = reg;
			// Send current prefs immediately once SW is active
			let cs: any, np: any;
			commute.subscribe((s) => {
				cs = s;
			})();
			notificationPrefs.subscribe((s) => {
				np = s;
			})();
			sendPrefs(cs, np);
		});

		let cs: any, np: any;
		const unsubC = commute.subscribe((s) => {
			cs = s;
			if (swReg) sendPrefs(cs, np);
		});
		const unsubN = notificationPrefs.subscribe((s) => {
			np = s;
			if (swReg) sendPrefs(cs, np);
		});

		return () => {
			unsubC();
			unsubN();
		};
	});
</script>

<svelte:head>
	<meta name="theme-color" content="#111111" />
	<meta name="apple-mobile-web-app-capable" content="yes" />
	<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent" />
	<meta name="apple-mobile-web-app-title" content="Rail Six" />
	<link rel="manifest" href="/manifest.json" />
	<link rel="apple-touch-icon" href="/icons/icon-192.png" />
	<link rel="canonical" href="https://railsix.com{path}" />
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html '<script type="application/ld+json">' + webAppJsonLd + '<' + '/script>'}
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html '<script type="application/ld+json">' + breadcrumbJsonLd + '<' + '/script>'}
</svelte:head>

{@render children()}

<nav class="bottom-nav" aria-label="Main navigation">
	<a href="/" class="nav-item" class:active={path === '/'} data-sveltekit-reload>
		<span class="icon">⊟</span>
		<span class="label">My Commute</span>
	</a>
	<a
		href="/departures"
		class="nav-item"
		class:active={path === '/departures'}
		data-sveltekit-reload
	>
		<span class="icon">◉</span>
		<span class="label">Departures</span>
	</a>
	<a href="/board" class="nav-item" class:active={path === '/board'} data-sveltekit-reload>
		<span class="icon">▤</span>
		<span class="label">Board</span>
	</a>
</nav>

<style>
	:global(body) {
		margin: 0;
		padding-bottom: 60px;
		background: #111;
	}

	.bottom-nav {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		height: 60px;
		background: #161616;
		border-top: 1px solid #2a2a2a;
		display: flex;
		z-index: 50;
	}

	.nav-item {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		text-decoration: none;
		color: #6b7280;
		transition: color 0.15s;
		font-family: monospace;
	}

	.nav-item.active {
		color: #f5a623;
	}

	.nav-item:hover {
		color: #d1d5db;
	}

	.icon {
		font-size: 1.1rem;
		line-height: 1;
	}
	.label {
		font-size: 0.6rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
	}
</style>
