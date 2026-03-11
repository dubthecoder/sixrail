export type BuildInfo = {
	version: string;
	label: string;
	branch: string | null;
	fullSha: string | null;
	shortSha: string | null;
	isDeployment: boolean;
};

function clean(value: string | undefined): string | null {
	const trimmed = value?.trim();
	return trimmed ? trimmed : null;
}

declare const __APP_VERSION__: string;

export function getBuildInfo(values: Record<string, string | undefined>): BuildInfo {
	const version = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0';
	const branch = clean(values.RAILWAY_GIT_BRANCH);
	const fullSha = clean(values.RAILWAY_GIT_COMMIT_SHA);
	const shortSha = fullSha?.slice(0, 7) ?? null;

	return {
		version,
		label: shortSha
			? branch && branch !== 'main'
				? `${branch}@${shortSha}`
				: shortSha
			: 'local',
		branch,
		fullSha,
		shortSha,
		isDeployment: Boolean(shortSha)
	};
}
