import { describe, expect, it } from 'vitest';

import { alertKey, normalizeAlerts } from './alerts';

describe('alert utilities', () => {
	it('returns an empty list for non-array payloads', () => {
		expect(normalizeAlerts(null)).toEqual([]);
		expect(normalizeAlerts({ headline: 'Signal issue' })).toEqual([]);
	});

	it('drops malformed alerts and deduplicates equivalent entries', () => {
		expect(
			normalizeAlerts([
				null,
				{ description: 'Missing headline' },
				{ headline: ' Signal issue ', description: ' Minor delays ', routeNames: ['LW'] },
				{ headline: 'Signal issue', description: 'Minor delays', routeNames: ['LW'] },
				{ headline: 'Signal issue', description: 'Minor delays', routeNames: ['LW', 'MI'] }
			])
		).toEqual([
			{ headline: 'Signal issue', description: 'Minor delays', routeNames: ['LW'] },
			{ headline: 'Signal issue', description: 'Minor delays', routeNames: ['LW', 'MI'] }
		]);
	});

	it('builds the same key regardless of route order', () => {
		expect(
			alertKey({
				headline: 'Service change',
				description: 'Boarding at a different platform',
				routeNames: ['LW', 'MI']
			})
		).toBe(
			alertKey({
				headline: 'Service change',
				description: 'Boarding at a different platform',
				routeNames: ['MI', 'LW']
			})
		);
	});
});
