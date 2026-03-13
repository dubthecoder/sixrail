import { error } from '@sveltejs/kit';

import { getAllStops, getAlerts } from '$lib/api';
import { stopToDisplayName } from '$lib/stations';

export async function load({ url }: { url: URL }) {
	try {
		const [stops, alerts] = await Promise.all([getAllStops(), getAlerts()]);

		if (!Array.isArray(stops) || !Array.isArray(alerts)) {
			throw error(502, 'Invalid response from departures-api');
		}

		// Parse URL trip params
		const from = url.searchParams.get('from');
		const to = url.searchParams.get('to');
		const dir = url.searchParams.get('dir');

		let urlTrip: {
			fromCode: string;
			fromName: string;
			toCode: string;
			toName: string;
			dir: 'toWork' | 'toHome';
		} | null = null;

		if (from && to && (dir === 'toWork' || dir === 'toHome')) {
			const fromStop = stops.find((s) => (s.code || s.id) === from);
			const toStop = stops.find((s) => (s.code || s.id) === to);
			if (fromStop && toStop) {
				urlTrip = {
					fromCode: fromStop.code || fromStop.id,
					fromName: stopToDisplayName(fromStop),
					toCode: toStop.code || toStop.id,
					toName: stopToDisplayName(toStop),
					dir
				};
			}
		}

		return {
			stops,
			alerts,
			urlTrip
		};
	} catch (err) {
		if (err instanceof Object && 'status' in err) throw err;
		console.error('[SSR] Failed to load homepage data:', err);
		throw error(503, 'Unable to load homepage data');
	}
}
