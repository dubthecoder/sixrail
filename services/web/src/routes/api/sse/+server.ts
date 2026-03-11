import { getSseUrl } from '$lib/server/proxy';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request }) => {
	const sseUrl = getSseUrl();
	if (!sseUrl) {
		return new Response('SSE not configured', { status: 503 });
	}

	const abortController = new AbortController();
	const upstream = await fetch(`${sseUrl}/sse`, { signal: abortController.signal });
	if (!upstream.ok || !upstream.body) {
		return new Response('SSE upstream unavailable', { status: 502 });
	}

	// Abort the upstream connection when the client disconnects.
	request.signal.addEventListener('abort', () => abortController.abort(), { once: true });

	// Pipe upstream chunks through an explicit ReadableStream so each SSE event
	// (including keepalive comments) is flushed to the client immediately rather
	// than being buffered by the Response passthrough.
	const reader = upstream.body.getReader();
	const stream = new ReadableStream({
		async pull(controller) {
			try {
				const { done, value } = await reader.read();
				if (done) {
					controller.close();
					return;
				}
				controller.enqueue(value);
			} catch {
				controller.close();
			}
		},
		cancel() {
			reader.cancel();
			abortController.abort();
		}
	});

	return new Response(stream, {
		headers: {
			'Content-Type': 'text/event-stream',
			'Cache-Control': 'no-cache',
			'X-Accel-Buffering': 'no'
		}
	});
};
