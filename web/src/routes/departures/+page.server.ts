import { getAllStops } from '$lib/api';

export async function load() {
	const stops = await getAllStops().catch((err) => {
		console.error('[SSR] Failed to load stops:', err);
		return [];
	});
	return {
		stops: Array.isArray(stops) ? stops : []
	};
}
