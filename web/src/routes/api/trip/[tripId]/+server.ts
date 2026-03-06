import { json } from '@sveltejs/kit';
import { getTripDetail } from '$lib/api';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params }) => {
	try {
		const detail = await getTripDetail(params.tripId);
		return json(detail);
	} catch {
		return json({ error: 'not found' }, { status: 404 });
	}
};
