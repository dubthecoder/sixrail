import { error } from '@sveltejs/kit';

import { getAllStops, getAlerts } from '$lib/api';

export async function load() {
	try {
		const [stops, alerts] = await Promise.all([getAllStops(), getAlerts()]);

		if (!Array.isArray(stops) || !Array.isArray(alerts)) {
			throw error(502, 'Invalid response from departures-api');
		}

		return {
			stops,
			alerts
		};
	} catch (err) {
		if (err instanceof Object && 'status' in err) throw err;
		console.error('[SSR] Failed to load homepage data:', err);
		throw error(503, 'Unable to load homepage data');
	}
}
