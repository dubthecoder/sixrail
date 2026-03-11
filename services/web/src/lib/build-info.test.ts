import { describe, expect, it, vi } from 'vitest';

// __APP_VERSION__ is injected by Vite define — mock it for tests
vi.stubGlobal('__APP_VERSION__', '42');

import { getBuildInfo } from './build-info';

describe('getBuildInfo', () => {
	it('uses the Railway git branch and short commit sha for deployed builds', () => {
		expect(
			getBuildInfo({
				RAILWAY_GIT_BRANCH: 'main',
				RAILWAY_GIT_COMMIT_SHA: '0123456789abcdef'
			})
		).toEqual({
			version: '42',
			label: '0123456',
			branch: 'main',
			fullSha: '0123456789abcdef',
			shortSha: '0123456',
			isDeployment: true
		});
	});

	it('shows branch name for non-main branches', () => {
		expect(
			getBuildInfo({
				RAILWAY_GIT_BRANCH: 'feature-x',
				RAILWAY_GIT_COMMIT_SHA: '0123456789abcdef'
			})
		).toEqual({
			version: '42',
			label: 'feature-x@0123456',
			branch: 'feature-x',
			fullSha: '0123456789abcdef',
			shortSha: '0123456',
			isDeployment: true
		});
	});

	it('falls back to a local label when Railway git metadata is unavailable', () => {
		expect(getBuildInfo({})).toEqual({
			version: '42',
			label: 'local',
			branch: null,
			fullSha: null,
			shortSha: null,
			isDeployment: false
		});
	});
});
