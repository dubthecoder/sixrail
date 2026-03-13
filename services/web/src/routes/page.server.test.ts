import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api', () => ({
	getAllStops: vi.fn(),
	getAlerts: vi.fn()
}));

import { getAlerts, getAllStops } from '$lib/api';
import { load } from './+page.server';

describe('homepage server load', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('returns SSR data when both backend calls succeed', async () => {
		vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
		vi.mocked(getAlerts).mockResolvedValue([{
			headline: 'Notice',
			description: 'Normal service'
		}]);

		const url = new URL('http://localhost/');
		await expect(load({ url } as any)).resolves.toEqual({
			stops: [{ code: 'UN', id: '1', name: 'Union' }],
			alerts: [{ headline: 'Notice', description: 'Normal service' }],
			urlTrip: null
		});
	});

	it('throws a 503 when homepage data cannot be loaded', async () => {
		vi.mocked(getAllStops).mockRejectedValue(new Error('backend unavailable'));
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/');
		await expect(load({ url } as any)).rejects.toMatchObject({ status: 503 });
	});

	it('returns urlTrip when valid from/to/dir params are provided', async () => {
		vi.mocked(getAllStops).mockResolvedValue([
			{ code: 'UN', id: '1', name: 'Union' },
			{ code: 'OASH', id: '2', name: 'Oakville' }
		]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/?from=UN&to=OASH&dir=toWork');
		const result = await load({ url } as any);
		expect(result.urlTrip).toEqual({
			fromCode: 'UN',
			fromName: 'Union',
			toCode: 'OASH',
			toName: 'Oakville',
			dir: 'toWork'
		});
	});

	it('returns urlTrip null when from param is missing', async () => {
		vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/?to=UN&dir=toWork');
		const result = await load({ url } as any);
		expect(result.urlTrip).toBeNull();
	});

	it('returns urlTrip null when from stop code is invalid', async () => {
		vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/?from=FAKE&to=UN&dir=toWork');
		const result = await load({ url } as any);
		expect(result.urlTrip).toBeNull();
	});

	it('returns urlTrip null when to stop code is invalid', async () => {
		vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/?from=UN&to=FAKE&dir=toWork');
		const result = await load({ url } as any);
		expect(result.urlTrip).toBeNull();
	});

	it('returns urlTrip null when dir is invalid', async () => {
		vi.mocked(getAllStops).mockResolvedValue([
			{ code: 'UN', id: '1', name: 'Union' },
			{ code: 'OASH', id: '2', name: 'Oakville' }
		]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/?from=UN&to=OASH&dir=invalid');
		const result = await load({ url } as any);
		expect(result.urlTrip).toBeNull();
	});

	it('returns urlTrip null when no URL params (bare /)', async () => {
		vi.mocked(getAllStops).mockResolvedValue([{ code: 'UN', id: '1', name: 'Union' }]);
		vi.mocked(getAlerts).mockResolvedValue([]);

		const url = new URL('http://localhost/');
		const result = await load({ url } as any);
		expect(result.urlTrip).toBeNull();
	});
});
