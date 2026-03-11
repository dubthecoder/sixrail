<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { browser } from '$app/environment';
	import { onMount, onDestroy } from 'svelte';
	import { track } from '$lib/track';
	import { connectSSE, disconnectSSE, onSSEStatus } from '$lib/sse';
	let { children } = $props();
	let path = $derived(page.url.pathname);

	// SSE connection status
	let sseConnected = $state(true);

	// Install prompt
	let deferredPrompt = $state<any>(null);
	let showInstallBanner = $state(false);

	function isStandalone() {
		return (
			window.matchMedia('(display-mode: standalone)').matches ||
			(navigator as any).standalone === true
		);
	}

	function dismissInstall() {
		showInstallBanner = false;
		localStorage.setItem('installDismissed', Date.now().toString());
		track('install-dismissed');
	}

	async function installApp() {
		if (deferredPrompt) {
			deferredPrompt.prompt();
			const result = await deferredPrompt.userChoice;
			if (result.outcome === 'accepted') {
				showInstallBanner = false;
				track('install-accepted');
			}
			deferredPrompt = null;
		} else {
			// iOS — show instructions
			showInstallBanner = false;
		}
	}

	let webAppJsonLd = $derived(
		JSON.stringify({
			'@context': 'https://schema.org',
			'@type': 'WebApplication',
			name: 'Rail Six',
			url: 'https://railsix.com',
			description:
				'Real-time GO Train schedule and commute tracker for Toronto. Live departure times, delays, and platform info for all GO Transit stations across the Greater Toronto Area.',
			applicationCategory: 'TravelApplication',
			operatingSystem: 'Web',
			offers: { '@type': 'Offer', price: '0', priceCurrency: 'CAD' },
			author: { '@type': 'Organization', name: 'Teclara Technologies Inc' },
			areaServed: { '@type': 'Place', name: 'Greater Toronto Area, Ontario, Canada' }
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
								name: path === '/departures' ? 'Departures' : path.slice(1),
								item: `https://railsix.com${path}`
							}
						]
					: [])
			]
		})
	);

	onMount(() => {
		if (!browser) return;

		// Install prompt logic
		if (!isStandalone()) {
			const dismissed = localStorage.getItem('installDismissed');
			const weekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
			if (!dismissed || parseInt(dismissed) < weekAgo) {
				// Android/Chrome: capture beforeinstallprompt
				window.addEventListener('beforeinstallprompt', (e: Event) => {
					e.preventDefault();
					deferredPrompt = e;
					showInstallBanner = true;
				});

				// iOS: show banner after 3 seconds (no beforeinstallprompt on iOS)
				const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
				if (isIOS) {
					setTimeout(() => {
						if (!showInstallBanner) showInstallBanner = true;
					}, 3000);
				}
			}
		}

		if ('serviceWorker' in navigator) {
			navigator.serviceWorker.register('/sw.js');
		}

		connectSSE('/api/sse');
		onSSEStatus((connected) => (sseConnected = connected));
	});

	onDestroy(() => {
		disconnectSSE();
	});
</script>

<svelte:head>
	<meta name="theme-color" content="#111111" />
	<meta name="apple-mobile-web-app-capable" content="yes" />
	<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent" />
	<meta name="apple-mobile-web-app-title" content="Rail Six" />
	<link rel="manifest" href="/manifest.json" />

	<link rel="canonical" href="https://railsix.com{path}" />
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html '<script type="application/ld+json">' + webAppJsonLd + '<' + '/script>'}
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html '<script type="application/ld+json">' + breadcrumbJsonLd + '<' + '/script>'}
</svelte:head>

{#if !sseConnected}
	<div class="sse-banner" role="status" aria-live="polite">
		<span class="font-mono text-xs text-amber-400">LIVE DATA RECONNECTING...</span>
	</div>
{/if}

<main class="w-full">
	{@render children()}
</main>

{#if showInstallBanner}
	<div class="install-banner">
		<div class="install-content">
			<div class="install-text">
				<p class="install-title">Install Rail Six</p>
				{#if deferredPrompt}
					<p class="install-desc">Add to your home screen for quick access</p>
				{:else}
					<p class="install-desc">
						Tap <span class="install-share">⎙</span> then "Add to Home Screen"
					</p>
				{/if}
			</div>
			<div class="install-actions">
				{#if deferredPrompt}
					<button class="install-btn" onclick={installApp}>Install</button>
				{/if}
				<button class="install-dismiss" onclick={dismissInstall}>&times;</button>
			</div>
		</div>
	</div>
{/if}

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
		<span class="icon">▤</span>
		<span class="label">Departures</span>
	</a>
</nav>

<style>
	.sse-banner {
		text-align: center;
		padding: 4px 0;
		background: var(--color-surface-raised);
		border-bottom: 1px solid var(--color-border-subtle);
	}

	:global(body) {
		margin: 0;
		padding-bottom: 60px;
		background: var(--color-surface);
	}

	.bottom-nav {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		height: 60px;
		background: var(--color-surface-raised);
		border-top: 1px solid var(--color-border);
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
		color: var(--color-gray-500);
		transition: color 0.15s;
		font-family: monospace;
	}

	.nav-item.active {
		color: var(--color-accent);
	}

	.nav-item:hover {
		color: var(--color-gray-300);
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

	.install-banner {
		position: fixed;
		bottom: 68px;
		left: 8px;
		right: 8px;
		z-index: 60;
		animation: slideUp 0.3s ease-out;
	}

	.install-content {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		background: var(--color-surface-overlay);
		border: 1px solid var(--color-border);
		border-radius: 10px;
		padding: 12px 16px;
		max-width: 480px;
		margin: 0 auto;
	}

	.install-text {
		flex: 1;
		min-width: 0;
	}

	.install-title {
		color: var(--color-accent);
		font-family: monospace;
		font-size: 0.8rem;
		font-weight: bold;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.install-desc {
		color: var(--color-gray-400);
		font-family: monospace;
		font-size: 0.65rem;
		margin-top: 2px;
	}

	.install-share {
		font-size: 1em;
	}

	.install-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}

	.install-btn {
		background: var(--color-accent);
		color: #000;
		font-family: monospace;
		font-size: 0.7rem;
		font-weight: bold;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		padding: 6px 14px;
		border-radius: 6px;
		cursor: pointer;
	}

	.install-dismiss {
		color: var(--color-gray-500);
		font-size: 1.2rem;
		line-height: 1;
		cursor: pointer;
		padding: 4px;
	}

	.install-dismiss:hover {
		color: var(--color-gray-300);
	}

	@keyframes slideUp {
		from {
			transform: translateY(20px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}
</style>
