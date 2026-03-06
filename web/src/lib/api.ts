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
	const res = await fetch(`${getBaseUrl()}${path}`);
	if (!res.ok) {
		throw new Error(`API error: ${res.status} ${res.statusText}`);
	}
	return res.json();
}

export interface Stop {
	id: string;
	code: string;
	name: string;
	lat: number;
	lon: number;
	parentId?: string;
}

export interface VehiclePosition {
	vehicleId: string;
	tripId: string;
	routeId: string;
	routeName: string;
	routeColor: string;
	routeType: number;
	lat: number;
	lon: number;
	bearing?: number;
	speed?: number;
	timestamp: number;
}

export interface Alert {
	id: string;
	effect: string;
	headline: string;
	description: string;
	url?: string;
	routeIds?: string[];
	routeNames?: string[];
	startTime?: number;
	endTime?: number;
}

export interface RouteShape {
	routeId: string;
	routeName: string;
	color: string;
	points: [number, number][]; // [lon, lat]
}

export function getRouteShapes() {
	return fetchApi<RouteShape[]>('/api/shapes');
}

export interface UpcomingStop {
	name: string;
	platform?: string;
	time: string;
	delayMinutes: number;
}

export interface TripDetail {
	tripId: string;
	vehicleId: string;
	routeName: string;
	routeColor: string;
	origin: string;
	destination: string;
	scheduleStart: string;
	scheduleEnd: string;
	status: string;
	delayMinutes: number;
	currentStop?: string;
	upcomingStops: UpcomingStop[];
}

export function getTripDetail(tripId: string) {
	return fetchApi<TripDetail>(`/api/trip/${tripId}`);
}

export function getAllStops() {
	return fetchApi<Stop[]>('/api/stops');
}

export function getStopDepartures(stopCode: string) {
	return fetchApi(`/api/departures/${stopCode}`);
}

export function getPositions() {
	return fetchApi<VehiclePosition[]>('/api/positions');
}

export function getAlerts() {
	return fetchApi<Alert[]>('/api/alerts');
}
