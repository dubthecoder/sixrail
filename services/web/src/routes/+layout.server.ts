import { env } from '$env/dynamic/private';
import { getBuildInfo } from '$lib/build-info';

export function load() {
	return {
		buildInfo: getBuildInfo(env),
		sseUrl: env.PUBLIC_SSE_URL || ''
	};
}
