<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { env } from '$env/dynamic/public';
	import type { Stop, VehiclePosition, Alert, RouteShape } from '$lib/api';
	import { fetchPositions, fetchAlerts, fetchTripDetail } from '$lib/api-client';
	import type { TripDetail } from '$lib/api';
	import { defaultStation } from '$lib/stores/favorites';
	import SearchOverlay from '$lib/components/SearchOverlay.svelte';
	import AlertsDropdown from '$lib/components/AlertsDropdown.svelte';
	import DeparturesPanel from '$lib/components/DeparturesPanel.svelte';
	import FilterChips from '$lib/components/FilterChips.svelte';
	import { filters, type FilterState } from '$lib/stores/filters';
	import { commute, getActiveDirection } from '$lib/stores/commute';

	let { data } = $props();

	let commuteState = $state<{ toWork: any; toHome: any }>({ toWork: null, toHome: null });
	$effect(() => {
		const unsub = commute.subscribe((s) => (commuteState = s));
		return unsub;
	});

	let activeTrip = $derived(commuteState[getActiveDirection(null)]);

	let stops = $derived<Stop[]>(data.stops);
	let shapes = $derived<RouteShape[]>(data.shapes);
	let positions = $state<VehiclePosition[]>([]);
	let alerts = $state<Alert[]>([]);

	let filterState = $state<FilterState>({
		showTrains: true,
		showBuses: false,
		activeRoutes: [],
		activeStatuses: []
	});

	// Seed client state from server data on first render
	$effect(() => {
		if (positions.length === 0 && data.positions.length > 0) positions = data.positions;
		if (alerts.length === 0 && data.alerts.length > 0) alerts = data.alerts;
	});

	$effect(() => {
		const unsub = filters.subscribe((s) => (filterState = s));
		return unsub;
	});

	let mapContainer: HTMLDivElement;
	let map: any;
	let mapboxgl: any;
	let mapReady = $state(false);

	let selectedStop = $state<Stop | null>(null);
	let mapError = $state<string | null>(null);

	function selectStation(stop: Stop) {
		selectedStop = stop;
		if (map) {
			map.flyTo({ center: [stop.lon, stop.lat], zoom: 14, duration: 1000 });
		}
	}

	function closePanel() {
		selectedStop = null;
	}

	function updatePositionLayer() {
		if (!map || !mapReady) return;
		const source = map.getSource('positions');
		if (!source) return;

		const filtered = positions.filter((p) => {
			if (!p.lat || !p.lon) return false;
			const isRail = p.routeType === 2 || p.routeType === 0;
			const isBus = p.routeType === 3;
			if (isRail && !filterState.showTrains) return false;
			if (isBus && !filterState.showBuses) return false;
			if (filterState.activeRoutes.length > 0 && !filterState.activeRoutes.includes(p.routeName))
				return false;
			if (filterState.activeStatuses.length > 0) {
				const status = 'ontime';
				if (!filterState.activeStatuses.includes(status)) return false;
			}
			return true;
		});

		source.setData({
			type: 'FeatureCollection',
			features: filtered.map((p) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
				properties: {
					routeName: p.routeName || p.routeId || '',
					tripId: p.tripId || '',
					color: p.routeColor ? `#${p.routeColor}` : '#15803d',
					routeType: p.routeType
				}
			}))
		});
	}

	onMount(() => {
		let posInterval: ReturnType<typeof setInterval>;
		let alertInterval: ReturnType<typeof setInterval>;

		(async () => {
			try {
				mapboxgl = (await import('mapbox-gl')).default;
				mapboxgl.accessToken = env.PUBLIC_MAPBOX_TOKEN || '';

				map = new mapboxgl.Map({
					container: mapContainer,
					style: 'mapbox://styles/mapbox/light-v11',
					center: [-79.38, 43.65],
					zoom: 9
				});

				map.addControl(new mapboxgl.NavigationControl(), 'bottom-right');
			} catch (e: any) {
				mapError = e?.message || String(e);
				console.error('Map init error:', e);
				return;
			}

			map.on('load', () => {
				// Rail route lines
				map.addSource('route-lines', {
					type: 'geojson',
					data: {
						type: 'FeatureCollection',
						features: shapes
							.filter((s) => s.points.length >= 2)
							.map((s) => ({
								type: 'Feature',
								geometry: {
									type: 'LineString',
									coordinates: s.points
								},
								properties: {
									routeName: s.routeName,
									color: s.color ? `#${s.color}` : '#15803d'
								}
							}))
					}
				});

				map.addLayer({
					id: 'route-lines-outline',
					type: 'line',
					source: 'route-lines',
					layout: { 'line-join': 'round', 'line-cap': 'round' },
					paint: {
						'line-color': '#ffffff',
						'line-width': 5,
						'line-opacity': 0.6
					}
				});

				map.addLayer({
					id: 'route-lines-layer',
					type: 'line',
					source: 'route-lines',
					layout: { 'line-join': 'round', 'line-cap': 'round' },
					paint: {
						'line-color': ['get', 'color'],
						'line-width': 3,
						'line-opacity': 0.8
					}
				});

				// Station markers
				map.addSource('stops', {
					type: 'geojson',
					data: {
						type: 'FeatureCollection',
						features: stops
							.filter((s) => s.lat && s.lon)
							.map((s) => ({
								type: 'Feature',
								geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
								properties: { id: s.id, code: s.code, name: s.name }
							}))
					}
				});

				map.addLayer({
					id: 'stops-layer',
					type: 'circle',
					source: 'stops',
					paint: {
						'circle-radius': 6,
						'circle-color': '#15803d',
						'circle-stroke-width': 2,
						'circle-stroke-color': '#ffffff'
					}
				});

				// Vehicle position icons
				const trainSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="11" fill="white" stroke="white" stroke-width="2"/><path d="M8 18l-2 3h12l-2-3M6 13V6a4 4 0 014-4h4a4 4 0 014 4v7M6 13h12M6 13l-1 2h14l-1-2" fill="none" stroke="%23374151" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><circle cx="9" cy="16" r="1" fill="%23374151"/><circle cx="15" cy="16" r="1" fill="%23374151"/><line x1="9" y1="6" x2="15" y2="6" stroke="%23374151" stroke-width="1.5"/><line x1="9" y1="9" x2="15" y2="9" stroke="%23374151" stroke-width="1.5"/></svg>`;
				const busSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="11" fill="white" stroke="white" stroke-width="2"/><path d="M5 15V7a4 4 0 014-4h6a4 4 0 014 4v8M5 15l-1 2h16l-1-2M5 15h14" fill="none" stroke="%233b82f6" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><circle cx="8" cy="17.5" r="1" fill="%233b82f6"/><circle cx="16" cy="17.5" r="1" fill="%233b82f6"/><rect x="6" y="5" width="12" height="5" rx="1" fill="none" stroke="%233b82f6" stroke-width="1.5"/></svg>`;

				const trainImg = new Image(24, 24);
				trainImg.onload = () => map.addImage('train-icon', trainImg);
				trainImg.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(trainSvg);

				const busImg = new Image(24, 24);
				busImg.onload = () => map.addImage('bus-icon', busImg);
				busImg.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(busSvg);

				// Vehicle positions
				map.addSource('positions', {
					type: 'geojson',
					data: { type: 'FeatureCollection', features: [] }
				});

				map.addLayer({
					id: 'positions-layer',
					type: 'symbol',
					source: 'positions',
					layout: {
						'icon-image': ['case', ['==', ['get', 'routeType'], 3], 'bus-icon', 'train-icon'],
						'icon-size': 0.9,
						'icon-allow-overlap': true,
						'icon-ignore-placement': true
					}
				});

				mapReady = true;
				updatePositionLayer();

				// Station click handler
				map.on('click', 'stops-layer', (e: any) => {
					const feature = e.features?.[0];
					if (!feature) return;
					const props = feature.properties;
					const [lon, lat] = feature.geometry.coordinates;
					selectStation({
						id: props.id,
						code: props.code,
						name: props.name,
						lat,
						lon
					});
				});

				// Vehicle click handler — rich popup with trip detail
				map.on('click', 'positions-layer', async (e: any) => {
					const feature = e.features?.[0];
					if (!feature) return;
					const props = feature.properties;
					const [lon, lat] = feature.geometry.coordinates;

					const popup = new mapboxgl.Popup({ maxWidth: '320px', className: 'train-popup' })
						.setLngLat([lon, lat])
						.setHTML(`<div class="p-3 text-center text-gray-400 text-sm">Loading...</div>`)
						.addTo(map);

					const detail: TripDetail | null = await fetchTripDetail(props.tripId);
					if (!detail) {
						popup.setHTML(`<div class="p-3"><strong>${props.routeName || '—'}</strong></div>`);
						return;
					}

					const color = detail.routeColor ? `#${detail.routeColor}` : '#15803d';
					const statusBadge =
						detail.delayMinutes > 0
							? `<span class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium" style="background:#fef2f2;color:#b91c1c">Delayed ${detail.delayMinutes}m</span>`
							: detail.status === 'Cancelled'
								? `<span class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium" style="background:#fef2f2;color:#b91c1c">Cancelled</span>`
								: `<span class="inline-block mt-1 px-2 py-0.5 rounded-full text-xs font-medium" style="background:#f0fdf4;color:#15803d">On Time</span>`;

					let stopsHTML = '';
					if (detail.upcomingStops.length > 0) {
						const stopRows = detail.upcomingStops
							.map((s) => {
								const delay =
									s.delayMinutes > 0
										? `<span style="color:#ef4444;font-size:10px">(+${s.delayMinutes}m)</span>`
										: '';
								return `<div style="display:flex;justify-content:space-between;align-items:center;padding:2px 0;font-size:12px">
							<div style="display:flex;align-items:center;gap:6px">
								<div style="width:6px;height:6px;border-radius:50%;background:#d1d5db;flex-shrink:0"></div>
								<span style="color:#374151">${s.name}</span>
							</div>
							<div style="flex-shrink:0;margin-left:8px;color:#6b7280">${s.time} ${delay}</div>
						</div>`;
							})
							.join('');
						stopsHTML = `<div style="border-top:1px solid #f3f4f6;padding:8px 12px">
					<div style="font-size:10px;text-transform:uppercase;letter-spacing:0.05em;color:#9ca3af;font-weight:600;margin-bottom:6px">Upcoming Stops (${detail.upcomingStops.length})</div>
					<div style="max-height:160px;overflow-y:auto">${stopRows}</div>
				</div>`;
					}

					popup.setHTML(`
				<div style="min-width:260px;max-width:320px">
					<div style="padding:12px 12px 8px">
						<div style="display:flex;align-items:center;gap:8px">
							<div style="width:12px;height:12px;border-radius:50%;background:${color};flex-shrink:0"></div>
							<span style="font-weight:700;font-size:14px;color:#111827">${detail.routeName}</span>
						</div>
						<div style="font-size:12px;color:#6b7280;margin-top:2px">#${detail.vehicleId}</div>
						${statusBadge}
					</div>
					<div style="border-top:1px solid #f3f4f6;padding:8px 12px;font-size:12px">
						${detail.origin && detail.destination ? `<div style="color:#374151"><strong>${detail.origin}</strong> <span style="color:#9ca3af">→</span> <strong>${detail.destination}</strong></div>` : ''}
						${detail.scheduleStart && detail.scheduleEnd ? `<div style="color:#6b7280;margin-top:4px">Schedule: ${detail.scheduleStart} – ${detail.scheduleEnd}</div>` : ''}
						${detail.currentStop ? `<div style="color:#6b7280;margin-top:4px">Next: <span style="color:#374151">${detail.currentStop}</span></div>` : ''}
					</div>
					${stopsHTML}
				</div>
			`);
				});

				// Cursor changes
				map.on('mouseenter', 'stops-layer', () => (map.getCanvas().style.cursor = 'pointer'));
				map.on('mouseleave', 'stops-layer', () => (map.getCanvas().style.cursor = ''));
				map.on('mouseenter', 'positions-layer', () => (map.getCanvas().style.cursor = 'pointer'));
				map.on('mouseleave', 'positions-layer', () => (map.getCanvas().style.cursor = ''));
			});
		})();

		// Poll positions every 15s, alerts every 60s
		posInterval = setInterval(async () => {
			positions = await fetchPositions();
		}, 15_000);

		alertInterval = setInterval(async () => {
			alerts = await fetchAlerts();
		}, 60_000);

		// Auto-select default station from localStorage
		if (browser && $defaultStation) {
			const stop = stops.find((s) => s.code === $defaultStation);
			if (stop) selectStation(stop);
		}

		return () => {
			clearInterval(posInterval);
			clearInterval(alertInterval);
			map?.remove();
		};
	});

	// Update position markers when positions change
	$effect(() => {
		positions;
		filterState;
		updatePositionLayer();
	});
</script>

<svelte:head>
	<link href="https://api.mapbox.com/mapbox-gl-js/v3.19.0/mapbox-gl.css" rel="stylesheet" />
</svelte:head>

<div class="relative w-screen overflow-hidden" style="height: calc(100vh - 60px)">
	<div class="absolute inset-0">
		<div bind:this={mapContainer} class="w-full h-full"></div>
	</div>

	<SearchOverlay {stops} onstationselect={selectStation} />
	<AlertsDropdown {alerts} />
	<FilterChips {positions} {filterState} />

	{#if mapError}
		<div class="absolute inset-0 z-30 flex items-center justify-center bg-white/80">
			<div class="bg-red-50 border border-red-500 rounded-lg p-4 max-w-md text-red-900 text-sm">
				<p class="font-bold">Map failed to load</p>
				<p class="mt-1">{mapError}</p>
			</div>
		</div>
	{/if}

	{#if selectedStop}
		<DeparturesPanel
			stopCode={selectedStop.code}
			stopName={selectedStop.name}
			onclose={closePanel}
		/>
	{/if}

	{#if activeTrip}
		<button
			class="absolute bottom-20 left-4 z-10 bg-[#161616] border border-[#2a2a2a] text-amber-400 font-mono text-xs px-3 py-2 rounded shadow-lg hover:border-amber-400 transition-colors"
			onclick={() => {
				const stop = stops.find((s) => s.code === activeTrip.originCode);
				if (stop && map) {
					map.flyTo({ center: [stop.lon, stop.lat], zoom: 14, duration: 1000 });
				}
			}}
		>
			◎ My Station
		</button>
	{/if}
</div>
