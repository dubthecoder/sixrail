const CACHE_VERSION = 'railsix-v2';
const IMMUTABLE_CACHE = 'railsix-immutable';
const PRECACHE_URLS = ['/manifest.json'];

self.addEventListener('install', (event) => {
	event.waitUntil(caches.open(CACHE_VERSION).then((cache) => cache.addAll(PRECACHE_URLS)));
	self.skipWaiting();
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) =>
				Promise.all(
					keys
						.filter((k) => k !== CACHE_VERSION && k !== IMMUTABLE_CACHE)
						.map((k) => caches.delete(k))
				)
			)
	);
	self.clients.claim();
});

self.addEventListener('fetch', (event) => {
	const url = new URL(event.request.url);

	// Only handle same-origin requests
	if (url.origin !== self.location.origin) return;

	// Skip Vite dev server internals
	if (url.pathname.startsWith('/@') || url.pathname.startsWith('/node_modules/')) return;

	// Let search engine crawlers fetch these directly
	if (url.pathname === '/sitemap.xml' || url.pathname === '/robots.txt') return;

	// API requests: network-only (no caching)
	if (url.pathname.startsWith('/api/')) {
		event.respondWith(
			// aikido-ignore: same-origin validated above
			fetch(event.request).catch(
				() =>
					new Response(JSON.stringify([]), {
						status: 503,
						headers: { 'Content-Type': 'application/json' }
					})
			)
		);
		return;
	}

	// Immutable hashed assets (/_app/immutable/): cache-first, safe to cache forever
	if (url.pathname.startsWith('/_app/immutable/')) {
		event.respondWith(
			caches.match(event.request).then(
				(cached) =>
					cached ||
					// aikido-ignore: same-origin validated above
					fetch(event.request).then((response) => {
						if (response.ok) {
							const clone = response.clone();
							caches.open(IMMUTABLE_CACHE).then((cache) => cache.put(event.request, clone));
						}
						return response;
					})
			)
		);
		return;
	}

	// Navigation requests (HTML pages): network-first with cache fallback
	if (event.request.mode === 'navigate') {
		event.respondWith(
			// aikido-ignore: same-origin validated above
			fetch(event.request)
				.then((response) => {
					if (response.ok) {
						const clone = response.clone();
						caches.open(CACHE_VERSION).then((cache) => cache.put(event.request, clone));
					}
					return response;
				})
				.catch(() =>
					caches
						.match(event.request)
						.then(
							(cached) =>
								cached ||
								new Response(
									'<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Offline</title></head><body style="background:#111;color:#999;font-family:monospace;display:flex;align-items:center;justify-content:center;height:100vh;margin:0"><div style="text-align:center"><h1 style="color:#fbbf24">Rail Six</h1><p>You are offline.</p><p>Please check your connection and try again.</p></div></body></html>',
									{ status: 503, headers: { 'Content-Type': 'text/html' } }
								)
						)
				)
		);
		return;
	}

	// Other same-origin assets (non-immutable JS/CSS, images, etc.): network-first
	event.respondWith(
		// aikido-ignore: same-origin validated above
		fetch(event.request)
			.then((response) => {
				if (response.ok) {
					const clone = response.clone();
					caches.open(CACHE_VERSION).then((cache) => cache.put(event.request, clone));
				}
				return response;
			})
			.catch(() => caches.match(event.request))
	);
});
