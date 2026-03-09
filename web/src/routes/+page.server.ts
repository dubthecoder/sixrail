import { getAllStops, getAlerts } from '$lib/api';

export async function load() {
	const [stops, alerts] = await Promise.all([
		getAllStops().catch((err) => {
			console.error('[SSR] Failed to load stops:', err);
			return [];
		}),
		getAlerts().catch((err) => {
			console.error('[SSR] Failed to load alerts:', err);
			return [];
		})
	]);
	return {
		stops: Array.isArray(stops) ? stops : [],
		alerts: Array.isArray(alerts) ? alerts : []
	};
}
