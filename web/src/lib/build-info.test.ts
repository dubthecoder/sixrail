import { describe, expect, it } from 'vitest';

import { getBuildInfo } from './build-info';

describe('getBuildInfo', () => {
	it('uses the Railway git branch and short commit sha for deployed builds', () => {
		expect(
			getBuildInfo({
				RAILWAY_GIT_BRANCH: 'main',
				RAILWAY_GIT_COMMIT_SHA: '0123456789abcdef'
			})
		).toEqual({
			label: 'main@0123456',
			branch: 'main',
			fullSha: '0123456789abcdef',
			shortSha: '0123456',
			isDeployment: true
		});
	});

	it('falls back to a local label when Railway git metadata is unavailable', () => {
		expect(getBuildInfo({})).toEqual({
			label: 'local',
			branch: null,
			fullSha: null,
			shortSha: null,
			isDeployment: false
		});
	});
});
