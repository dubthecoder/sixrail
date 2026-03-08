const CACHE_NAME = 'sixrail-v2';
const STATIC_ASSETS = ['/', '/manifest.json'];

self.addEventListener('install', (event) => {
	event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS)));
	self.skipWaiting();
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) =>
				Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
			)
	);
	self.clients.claim();
});

self.addEventListener('fetch', (event) => {
	const url = new URL(event.request.url);

	if (url.pathname.startsWith('/api/')) {
		event.respondWith(
			fetch(event.request).catch(
				() =>
					caches.match(event.request) ||
					new Response(JSON.stringify([]), {
						status: 503,
						headers: { 'Content-Type': 'application/json' }
					})
			)
		);
		return;
	}

	event.respondWith(caches.match(event.request).then((cached) => cached || fetch(event.request)));
});

let lastDelayMinutes = null;
let notifPrefs = { enabled: false, thresholdMinutes: 5 };
let commuteState = { toWork: null, toHome: null };

self.addEventListener('message', (event) => {
	if (event.data?.type === 'UPDATE_PREFS') {
		notifPrefs = event.data.notifPrefs;
		commuteState = event.data.commuteState;
	}
});

async function checkDepartures() {
	if (!notifPrefs.enabled) return;

	const hour = new Date().getHours();
	const direction = hour < 12 ? 'toWork' : 'toHome';
	const trip = commuteState[direction];
	if (!trip) return;

	try {
		const res = await fetch(`/api/departures/${encodeURIComponent(trip.originCode)}`);
		if (!res.ok) return;
		const departures = await res.json();
		if (!departures.length) return;

		const next = departures[0];
		const delay = next.delayMinutes ?? 0;

		if (
			lastDelayMinutes !== null &&
			delay > lastDelayMinutes &&
			delay >= notifPrefs.thresholdMinutes
		) {
			self.registration.showNotification('Six Rail — Delay Alert', {
				body: `Your ${next.scheduledTime} ${next.line} is now delayed ${delay} min`,
				icon: '/icons/icon-192.png',
				badge: '/icons/icon-192.png',
				tag: 'delay-alert',
				renotify: true
			});
		}

		lastDelayMinutes = delay;
	} catch {
		// ignore network errors
	}
}

setInterval(checkDepartures, 2 * 60 * 1000);
