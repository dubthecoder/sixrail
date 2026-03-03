import { json } from '@sveltejs/kit';
import { getStopDepartures } from '$lib/api';

export async function GET({ params }) {
	try {
		const departures = await getStopDepartures(params.stopCode);
		return json(departures);
	} catch {
		return json([], { status: 502 });
	}
}
