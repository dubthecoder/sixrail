import type { Alert } from './api';

export async function fetchAlerts(): Promise<Alert[]> {
	const res = await fetch('/api/alerts');
	if (!res.ok) return [];
	return res.json();
}

export type Departure = {
	line: string;
	destination: string;
	scheduledTime: string;
	arrivalTime?: string;
	status: string;
	platform?: string;
	routeColor?: string;
	delayMinutes?: number;
	stops?: string[];
};

export async function fetchDepartures(stopCode: string, destCode?: string): Promise<Departure[]> {
	const url = destCode
		? `/api/departures/${encodeURIComponent(stopCode)}?dest=${encodeURIComponent(destCode)}`
		: `/api/departures/${encodeURIComponent(stopCode)}`;
	const res = await fetch(url);
	if (!res.ok) return [];
	return res.json();
}

export type UnionDeparture = {
	tripNumber: string;
	service: string;
	platform: string;
	time: string;
	info: string;
	stops: string[];
};

export async function fetchUnionDepartures(): Promise<UnionDeparture[]> {
	const res = await fetch('/api/union-departures');
	if (!res.ok) return [];
	return res.json();
}
