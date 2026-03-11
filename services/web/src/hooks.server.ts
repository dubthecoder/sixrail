import { env } from '$env/dynamic/private';
import { error, type Handle } from '@sveltejs/kit';
import { closeSSE, isRateLimited, openSSE } from '$lib/server/rate-limit';

const RATE_LIMIT = 60; // max requests per window
const SSE_MAX_PER_IP = 3;
const ALLOWED_FETCH_SITES = new Set(['same-origin', 'same-site', 'none']);

function getClientIp(event: Parameters<Handle>[0]['event']): string {
	const forwardedFor = event.request.headers.get('x-forwarded-for')?.split(',')[0]?.trim();

	if (!env.ADDRESS_HEADER && forwardedFor) {
		return forwardedFor.replace(/^::ffff:/, '');
	}

	try {
		return event.getClientAddress().replace(/^::ffff:/, '');
	} catch {
		return forwardedFor?.replace(/^::ffff:/, '') || 'unknown';
	}
}

function isAllowedBrowserApiRequest(event: Parameters<Handle>[0]['event']): boolean {
	const origin = event.request.headers.get('origin');
	if (origin && origin !== event.url.origin) {
		return false;
	}

	const referer = event.request.headers.get('referer');
	if (!origin && referer) {
		try {
			if (new URL(referer).origin !== event.url.origin) {
				return false;
			}
		} catch {
			return false;
		}
	}

	const fetchSite = event.request.headers.get('sec-fetch-site');
	if (fetchSite && !ALLOWED_FETCH_SITES.has(fetchSite)) {
		return false;
	}

	// Require at least one browser-supplied provenance signal so direct
	// header-less requests from non-browser clients do not pass through.
	return Boolean(origin || referer || fetchSite);
}

export const handle: Handle = async ({ event, resolve }) => {
	const { pathname } = event.url;

	if (!pathname.startsWith('/api/')) {
		const response = await resolve(event);
		return addSecurityHeaders(response);
	}

	if (!isAllowedBrowserApiRequest(event)) {
		throw error(403, 'Cross-origin API requests are not allowed');
	}

	const ip = getClientIp(event);

	// SSE: browser same-origin checks apply above; this is best-effort abuse control.
	if (pathname === '/api/sse') {
		if (!(await openSSE(ip, SSE_MAX_PER_IP))) {
			throw error(429, 'Too many SSE connections');
		}

		// Decrement count when client disconnects — no TransformStream wrapping
		// which was breaking the SSE stream.
		event.request.signal.addEventListener('abort', () => {
			void closeSSE(ip);
		});

		try {
			const response = await resolve(event);
			return addSecurityHeaders(response);
		} catch (err) {
			await closeSSE(ip);
			throw err;
		}
	}

	if (await isRateLimited(ip, RATE_LIMIT)) {
		throw error(429, 'Too many requests');
	}

	const response = await resolve(event);
	return addSecurityHeaders(response);
};

function addSecurityHeaders(response: Response): Response {
	response.headers.set('X-Frame-Options', 'DENY');
	response.headers.set('X-Content-Type-Options', 'nosniff');
	response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
	response.headers.set('Cross-Origin-Resource-Policy', 'same-origin');
	response.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
	return response;
}
