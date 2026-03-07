import { getUnionDepartures, getAllStops } from '$lib/api';

export async function load() {
	const [departures, stops] = await Promise.all([
		getUnionDepartures().catch(() => []),
		getAllStops().catch(() => [])
	]);
	return {
		departures: Array.isArray(departures) ? departures : [],
		stops: Array.isArray(stops) ? stops : []
	};
}
