import { proxyFetch } from '$lib/server/proxy';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = () => proxyFetch('/network-health');
