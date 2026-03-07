import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getFares } from '$lib/api';

export const GET: RequestHandler = async ({ params }) => {
	try {
		const fares = await getFares(params.from, params.to);
		return json(fares);
	} catch {
		return json([], { status: 502 });
	}
};
