// web/src/routes/+page.server.ts
import { getAllStops } from '$lib/api';

export async function load() {
	try {
		const stops = await getAllStops();
		return { stops: Array.isArray(stops) ? stops : [] };
	} catch {
		return { stops: [] };
	}
}
