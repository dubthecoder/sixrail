import type { Alert } from './api';

type AlertLike = Partial<Alert> & {
	routeNames?: unknown;
};

export function alertKey(alert: Alert): string {
	const routeNames = Array.isArray(alert.routeNames)
		? [...alert.routeNames]
				.filter((route) => route.length > 0)
				.sort()
				.join('|')
		: '';

	return `${alert.headline}\u0000${alert.description}\u0000${routeNames}`;
}

export function normalizeAlerts(value: unknown): Alert[] {
	if (!Array.isArray(value)) return [];

	const seen = new Set<string>();
	const normalized: Alert[] = [];

	for (const item of value) {
		if (!item || typeof item !== 'object') continue;

		const alert = item as AlertLike;
		const headline = typeof alert.headline === 'string' ? alert.headline.trim() : '';
		if (!headline) continue;

		const description = typeof alert.description === 'string' ? alert.description.trim() : '';
		const routeNames = Array.isArray(alert.routeNames)
			? alert.routeNames.filter(
					(route): route is string => typeof route === 'string' && route.length > 0
				)
			: undefined;

		const normalizedAlert: Alert = routeNames?.length
			? { headline, description, routeNames }
			: { headline, description };
		const key = alertKey(normalizedAlert);

		if (seen.has(key)) continue;

		seen.add(key);
		normalized.push(normalizedAlert);
	}

	return normalized;
}
