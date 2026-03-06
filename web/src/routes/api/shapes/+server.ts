import { json } from '@sveltejs/kit';
import { getRouteShapes } from '$lib/api';

export async function GET() {
	try {
		const shapes = await getRouteShapes();
		return json(shapes);
	} catch {
		return json([], { status: 502 });
	}
}
