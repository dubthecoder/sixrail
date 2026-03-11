import { afterEach, describe, expect, it, vi } from 'vitest';

import { resetLimiterStateForTests } from '$lib/server/rate-limit';
import { handle } from './hooks.server';

function makeEvent(path: string, headers: Record<string, string> = {}) {
	return {
		url: new URL(`https://railsix.com${path}`),
		request: new Request(`https://railsix.com${path}`, {
			headers: new Headers(headers)
		}),
		getClientAddress: () => '127.0.0.1'
	};
}

describe('web hook API protections', () => {
	afterEach(() => {
		resetLimiterStateForTests();
	});

	it('allows same-origin API requests', async () => {
		const resolve = vi.fn().mockResolvedValue(new Response('ok'));

		const response = await handle({
			event: makeEvent('/api/alerts', {
				origin: 'https://railsix.com',
				'sec-fetch-site': 'same-origin'
			}) as never,
			resolve
		});

		expect(resolve).toHaveBeenCalledTimes(1);
		expect(response.status).toBe(200);
		expect(response.headers.get('Cross-Origin-Resource-Policy')).toBe('same-origin');
	});

	it('allows same-origin API requests with only a same-origin referer', async () => {
		const resolve = vi.fn().mockResolvedValue(new Response('ok'));

		const response = await handle({
			event: makeEvent('/api/alerts', {
				referer: 'https://railsix.com/commute'
			}) as never,
			resolve
		});

		expect(resolve).toHaveBeenCalledTimes(1);
		expect(response.status).toBe(200);
	});

	it('rejects cross-origin API requests', async () => {
		const resolve = vi.fn().mockResolvedValue(new Response('ok'));

		await expect(
			handle({
				event: makeEvent('/api/alerts', {
					origin: 'https://evil.example',
					'sec-fetch-site': 'cross-site'
				}) as never,
				resolve
			})
		).rejects.toMatchObject({ status: 403 });

		expect(resolve).not.toHaveBeenCalled();
	});

	it('rejects cross-site browser requests without an origin header', async () => {
		const resolve = vi.fn().mockResolvedValue(new Response('ok'));

		await expect(
			handle({
				event: makeEvent('/api/alerts', {
					referer: 'https://evil.example/widget',
					'sec-fetch-site': 'cross-site'
				}) as never,
				resolve
			})
		).rejects.toMatchObject({ status: 403 });

		expect(resolve).not.toHaveBeenCalled();
	});

	it('rejects header-less direct API requests', async () => {
		const resolve = vi.fn().mockResolvedValue(new Response('ok'));

		await expect(
			handle({
				event: makeEvent('/api/alerts') as never,
				resolve
			})
		).rejects.toMatchObject({ status: 403 });

		expect(resolve).not.toHaveBeenCalled();
	});
});
