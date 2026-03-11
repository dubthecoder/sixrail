import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
	ApiError,
	fetchAlerts,
	fetchDepartures,
	fetchNetworkHealth,
	fetchUnionDepartures
} from './api-client';

describe('api client helpers', () => {
	let fetchMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	it('fetches alerts from same-origin proxy', async () => {
		const alerts = [{ headline: 'Signal issue', description: 'Minor delays' }];
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify(alerts), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await expect(fetchAlerts()).resolves.toEqual(alerts);
		expect(fetchMock).toHaveBeenCalledWith('/api/alerts', expect.any(Object));
		expect(fetchMock.mock.calls[0]?.[1]).not.toMatchObject({
			headers: expect.anything()
		});
	});

	it('builds departures URLs with optional encoded destination codes', async () => {
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify({ departures: [] }), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await fetchDepartures('UN', 'KI B');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/departures/UN?dest=KI%20B');
	});

	it('throws ApiError when departures fail', async () => {
		fetchMock.mockResolvedValue(new Response('upstream failed', { status: 502 }));

		await expect(fetchDepartures('UN')).rejects.toThrow(ApiError);
		await expect(fetchDepartures('UN')).rejects.toMatchObject({ status: 502 });
	});

	it('throws ApiError for union departures and network health on non-ok responses', async () => {
		fetchMock.mockResolvedValueOnce(new Response('bad gateway', { status: 502 }));
		await expect(fetchUnionDepartures()).rejects.toThrow(ApiError);

		fetchMock.mockResolvedValueOnce(new Response('bad gateway', { status: 503 }));
		await expect(fetchNetworkHealth()).rejects.toThrow(ApiError);
	});
});
