import { getAllStops, getPositions, getAlerts } from '$lib/api';

export async function load() {
	const [stops, positions, alerts] = await Promise.all([
		getAllStops().catch(() => []),
		getPositions().catch(() => []),
		getAlerts().catch(() => [])
	]);

	return {
		stops: Array.isArray(stops) ? stops : [],
		positions: Array.isArray(positions) ? positions : [],
		alerts: Array.isArray(alerts) ? alerts : []
	};
}
