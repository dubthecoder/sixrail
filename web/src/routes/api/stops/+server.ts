import { json } from '@sveltejs/kit';
import { getAllStops } from '$lib/api';

export async function GET() {
	try {
		const stops = await getAllStops();
		return json(stops);
	} catch {
		return json([], { status: 502 });
	}
}
