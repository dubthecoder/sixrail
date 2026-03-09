import { getAllStops } from '$lib/api';

export async function load() {
	const stops = await getAllStops().catch(() => []);
	return {
		stops: Array.isArray(stops) ? stops : []
	};
}
