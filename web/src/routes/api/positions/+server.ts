import { json } from '@sveltejs/kit';
import { getPositions } from '$lib/api';

export async function GET() {
	try {
		const positions = await getPositions();
		return json(positions);
	} catch {
		return json([], { status: 502 });
	}
}
