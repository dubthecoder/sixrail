<!-- web/src/routes/map/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { env } from '$env/dynamic/public';

	let { data } = $props();
	let mapContainer: HTMLDivElement;
	let map: any;

	onMount(() => {
		let interval: ReturnType<typeof setInterval>;

		(async () => {
			const mapboxgl = (await import('mapbox-gl')).default;

			mapboxgl.accessToken = env.PUBLIC_MAPBOX_TOKEN || '';

			map = new mapboxgl.Map({
				container: mapContainer,
				style: 'mapbox://styles/mapbox/light-v11',
				center: [-79.38, 43.65], // Toronto
				zoom: 9
			});

			map.addControl(new mapboxgl.NavigationControl());

			if ((data.positions as any)?.entity) {
				for (const entity of (data.positions as any).entity) {
					const vp = entity.vehicle?.position;
					if (vp?.latitude && vp?.longitude) {
						new mapboxgl.Marker({ color: '#15803d' })
							.setLngLat([vp.longitude, vp.latitude])
							.setPopup(
								new mapboxgl.Popup().setHTML(
									`<strong>Trip ${entity.vehicle?.trip?.tripId || '—'}</strong><br/>
									 Route: ${entity.vehicle?.trip?.routeId || '—'}`
								)
							)
							.addTo(map);
					}
				}
			}

			// Poll for updates
			interval = setInterval(async () => {
				try {
					await fetch('/map/__data.json?x-sveltekit-invalidated=1');
					// SvelteKit will re-run the load function
				} catch {
					/* ignore */
				}
			}, 15_000);
		})();

		return () => {
			clearInterval(interval);
			map?.remove();
		};
	});
</script>

<svelte:head>
	<link href="https://api.mapbox.com/mapbox-gl-js/v3.4.0/mapbox-gl.css" rel="stylesheet" />
</svelte:head>

<div class="space-y-4">
	<h1 class="text-2xl font-bold">Live Train Map</h1>
	<div bind:this={mapContainer} class="w-full h-[600px] rounded-lg border border-gray-200"></div>
</div>
