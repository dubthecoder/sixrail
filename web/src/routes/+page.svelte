<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { env } from '$env/dynamic/public';
	import type { Stop, VehiclePosition, Alert } from '$lib/api';
	import { fetchPositions, fetchAlerts } from '$lib/api-client';
	import { defaultStation } from '$lib/stores/favorites';
	import SearchOverlay from '$lib/components/SearchOverlay.svelte';
	import AlertsDropdown from '$lib/components/AlertsDropdown.svelte';
	import DeparturesPanel from '$lib/components/DeparturesPanel.svelte';

	let { data } = $props();

	let stops = $derived<Stop[]>(data.stops);
	let positions = $state<VehiclePosition[]>([]);
	let alerts = $state<Alert[]>([]);

	// Seed client state from server data on first render
	$effect(() => {
		if (positions.length === 0 && data.positions.length > 0) positions = data.positions;
		if (alerts.length === 0 && data.alerts.length > 0) alerts = data.alerts;
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
		source.setData({
			type: 'FeatureCollection',
			features: positions
				.filter((p) => p.lat && p.lon)
				.map((p) => ({
					type: 'Feature',
					geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
					properties: {
						routeName: p.routeName || p.routeId || '',
						tripId: p.tripId || '',
						color: p.routeColor ? `#${p.routeColor}` : '#15803d'
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

				// Vehicle positions
				map.addSource('positions', {
					type: 'geojson',
					data: { type: 'FeatureCollection', features: [] }
				});

				map.addLayer({
					id: 'positions-layer',
					type: 'circle',
					source: 'positions',
					paint: {
						'circle-radius': 5,
						'circle-color': ['get', 'color'],
						'circle-stroke-width': 1.5,
						'circle-stroke-color': '#ffffff'
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

				// Vehicle click handler
				map.on('click', 'positions-layer', (e: any) => {
					const feature = e.features?.[0];
					if (!feature) return;
					const props = feature.properties;
					const [lon, lat] = feature.geometry.coordinates;
					new mapboxgl.Popup()
						.setLngLat([lon, lat])
						.setHTML(`<strong>${props.routeName || '—'}</strong><br/>Trip: ${props.tripId || '—'}`)
						.addTo(map);
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
		updatePositionLayer();
	});
</script>

<svelte:head>
	<link href="https://api.mapbox.com/mapbox-gl-js/v3.19.0/mapbox-gl.css" rel="stylesheet" />
</svelte:head>

<div class="relative w-screen h-screen overflow-hidden">
	<div class="absolute inset-0">
		<div bind:this={mapContainer} class="w-full h-full"></div>
	</div>

	<SearchOverlay {stops} onstationselect={selectStation} />
	<AlertsDropdown {alerts} />

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
</div>
