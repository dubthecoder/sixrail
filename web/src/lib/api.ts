import { env } from '$env/dynamic/private';
import { dev } from '$app/environment';

function getBaseUrl() {
	const url = env.API_BASE_URL || (dev ? 'http://localhost:8080' : '');
	if (!url) {
		throw new Error('API_BASE_URL environment variable is required in production');
	}
	return url;
}

async function fetchApi<T>(path: string): Promise<T> {
	const res = await fetch(`${getBaseUrl()}${path}`, {
		signal: AbortSignal.timeout(5000)
	});
	if (!res.ok) {
		throw new Error(`API error: ${res.status} ${res.statusText}`);
	}
	return res.json();
}

export interface Stop {
	id: string;
	code: string;
	name: string;
}

export interface Alert {
	headline: string;
	description: string;
	routeNames?: string[];
}

export function getAllStops() {
	return fetchApi<Stop[]>('/api/stops');
}

export function getStopDepartures(stopCode: string, destCode?: string) {
	const url = destCode
		? `/api/departures/${encodeURIComponent(stopCode)}?dest=${encodeURIComponent(destCode)}`
		: `/api/departures/${encodeURIComponent(stopCode)}`;
	return fetchApi(url);
}

export function getAlerts() {
	return fetchApi<Alert[]>('/api/alerts');
}

export function getUnionDepartures() {
	return fetchApi('/api/union-departures');
}

export function getNetworkHealth() {
	return fetchApi('/api/network-health');
}

export function getFares(fromCode: string, toCode: string) {
	return fetchApi(`/api/fares/${encodeURIComponent(fromCode)}/${encodeURIComponent(toCode)}`);
}
