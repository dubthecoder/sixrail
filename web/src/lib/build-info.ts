export type BuildInfo = {
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

export function getBuildInfo(values: Record<string, string | undefined>): BuildInfo {
	const branch = clean(values.RAILWAY_GIT_BRANCH);
	const fullSha = clean(values.RAILWAY_GIT_COMMIT_SHA);
	const shortSha = fullSha?.slice(0, 7) ?? null;

	return {
		label: shortSha ? (branch ? `${branch}@${shortSha}` : shortSha) : 'local',
		branch,
		fullSha,
		shortSha,
		isDeployment: Boolean(shortSha)
	};
}
