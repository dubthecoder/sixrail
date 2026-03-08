import type { Alert } from './api';

export async function fetchAlerts(): Promise<Alert[]> {
	const res = await fetch('/api/alerts');
	if (!res.ok) return [];
	return res.json();
}

export type Departure = {
	line: string;
	lineName?: string;
	scheduledTime: string;
	arrivalTime?: string;
	status: string;
	platform?: string;
	delayMinutes?: number;
	stops?: string[];
	occupancy?: number;
	cars?: string;
	isInMotion?: boolean;
	isCancelled?: boolean;
	alert?: string;
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
	service: string;
	platform: string;
	time: string;
	info: string;
	stops: string[];
	cars?: string;
	occupancy?: number;
	isInMotion?: boolean;
	isCancelled?: boolean;
	alert?: string;
};

export async function fetchUnionDepartures(): Promise<UnionDeparture[]> {
	const res = await fetch('/api/union-departures');
	if (!res.ok) return [];
	return res.json();
}

export type NetworkLine = {
	lineCode: string;
	lineName: string;
	activeTrips: number;
};

export async function fetchNetworkHealth(): Promise<NetworkLine[]> {
	const res = await fetch('/api/network-health');
	if (!res.ok) return [];
	return res.json();
}

export type FareInfo = {
	category: string;
	fareType: string;
	amount: number;
};

export async function fetchFares(from: string, to: string): Promise<FareInfo[]> {
	const res = await fetch(`/api/fares/${encodeURIComponent(from)}/${encodeURIComponent(to)}`);
	if (!res.ok) return [];
	return res.json();
}
