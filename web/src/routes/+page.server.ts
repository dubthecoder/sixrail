import { getAllStops, getPositions, getAlerts, getRouteShapes } from '$lib/api';

export async function load() {
	const [stops, positions, alerts, shapes] = await Promise.all([
		getAllStops().catch(() => []),
		getPositions().catch(() => []),
		getAlerts().catch(() => []),
		getRouteShapes().catch(() => [])
	]);

	return {
		stops: Array.isArray(stops) ? stops : [],
		positions: Array.isArray(positions) ? positions : [],
		alerts: Array.isArray(alerts) ? alerts : [],
		shapes: Array.isArray(shapes) ? shapes : []
	};
}
