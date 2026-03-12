import { getSseUrl } from '$lib/server/proxy';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	const sseUrl = getSseUrl();

	let upstream: Response;
	try {
		upstream = await fetch(`${sseUrl}/sse`);
	} catch {
		return new Response('SSE upstream unreachable', { status: 502 });
	}
	if (!upstream.ok || !upstream.body) {
		return new Response('SSE upstream unavailable', { status: 502 });
	}

	return new Response(upstream.body, {
		headers: {
			'Content-Type': 'text/event-stream',
			'Cache-Control': 'no-cache',
			'X-Accel-Buffering': 'no'
		}
	});
};
