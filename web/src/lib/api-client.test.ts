import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Mock $env/dynamic/public before importing api-client
vi.mock('$env/dynamic/public', () => ({
	env: { PUBLIC_API_URL: 'http://test-api:8080' }
}));

import {
	ApiError,
	fetchAlerts,
	fetchDepartures,
	fetchFares,
	fetchNetworkHealth,
	fetchUnionDepartures
} from './api-client';

const BASE = 'http://test-api:8080';

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

	it('fetches alerts and returns parsed JSON', async () => {
		const alerts = [{ headline: 'Signal issue', description: 'Minor delays' }];
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify(alerts), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await expect(fetchAlerts()).resolves.toEqual(alerts);
		expect(fetchMock).toHaveBeenCalledWith(`${BASE}/api/alerts`, expect.any(Object));
	});

	it('builds departures URLs with optional encoded destination codes', async () => {
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify([]), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await fetchDepartures('UN', 'KI B');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(fetchMock.mock.calls[0]?.[0]).toBe(`${BASE}/api/departures/UN?dest=KI%20B`);
	});

	it('throws ApiError when departures fail', async () => {
		fetchMock.mockResolvedValue(new Response('upstream failed', { status: 502 }));

		await expect(fetchDepartures('UN')).rejects.toThrow(ApiError);
		await expect(fetchDepartures('UN')).rejects.toMatchObject({ status: 502 });
	});

	it('fetches fares with both path parameters encoded', async () => {
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify([{ amount: 4.5 }]), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await fetchFares('UN', 'BR&GO');

		expect(fetchMock.mock.calls[0]?.[0]).toBe(`${BASE}/api/fares/UN/BR%26GO`);
	});

	it('throws ApiError for union departures and network health on non-ok responses', async () => {
		fetchMock.mockResolvedValueOnce(new Response('bad gateway', { status: 502 }));
		await expect(fetchUnionDepartures()).rejects.toThrow(ApiError);

		fetchMock.mockResolvedValueOnce(new Response('bad gateway', { status: 503 }));
		await expect(fetchNetworkHealth()).rejects.toThrow(ApiError);
	});
});
