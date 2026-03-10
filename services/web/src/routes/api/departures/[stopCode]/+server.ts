import { proxyFetch } from '$lib/server/proxy';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = ({ params, url }) => {
	const stopCode = encodeURIComponent(params.stopCode);
	const dest = url.searchParams.get('dest');
	const path = dest
		? `/departures/${stopCode}?dest=${encodeURIComponent(dest)}`
		: `/departures/${stopCode}`;
	return proxyFetch(path);
};
