import { json } from '@sveltejs/kit';
import { getNetworkHealth } from '$lib/api';

export async function GET() {
	try {
		const lines = await getNetworkHealth();
		return json(lines);
	} catch {
		return json([], { status: 502 });
	}
}
