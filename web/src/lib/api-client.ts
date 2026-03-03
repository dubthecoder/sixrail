export type { Stop, VehiclePosition, Alert } from './api';
import type { Stop, VehiclePosition, Alert } from './api';

export async function fetchStops(): Promise<Stop[]> {
	const res = await fetch('/api/stops');
	if (!res.ok) return [];
	return res.json();
}

export async function fetchPositions(): Promise<VehiclePosition[]> {
	const res = await fetch('/api/positions');
	if (!res.ok) return [];
	return res.json();
}

export async function fetchAlerts(): Promise<Alert[]> {
	const res = await fetch('/api/alerts');
	if (!res.ok) return [];
	return res.json();
}

export async function fetchDepartures(stopCode: string): Promise<any[]> {
	const res = await fetch(`/api/departures/${encodeURIComponent(stopCode)}`);
	if (!res.ok) return [];
	return res.json();
}
