import { getAllStops, getAlerts } from '$lib/api';

export async function load() {
	const [stops, alerts] = await Promise.all([
		getAllStops().catch(() => []),
		getAlerts().catch(() => [])
	]);
	return {
		stops: Array.isArray(stops) ? stops : [],
		alerts: Array.isArray(alerts) ? alerts : []
	};
}
