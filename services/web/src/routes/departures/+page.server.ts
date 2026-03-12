import { error } from '@sveltejs/kit';

import { getAllStops } from '$lib/api';

export async function load() {
	try {
		const stops = await getAllStops();
		if (!Array.isArray(stops)) {
			throw error(502, 'Invalid response from departures-api');
		}

		return { stops };
	} catch (err) {
		if (err instanceof Object && 'status' in err) throw err;
		console.error('[SSR] Failed to load departures page data:', err);
		throw error(503, 'Unable to load departures page data');
	}
}
