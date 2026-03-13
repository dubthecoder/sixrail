import type { Stop } from '$lib/api';

/** Convert a stop name to a URL-friendly slug: "Oakville GO" → "oakville" */
export function stopToSlug(stop: Stop): string {
	return stop.name
		.replace(/\s+GO\s+Station\b.*/i, '') // "Brampton Innovation District GO Station Rail" → "Brampton Innovation District"
		.replace(/\s+GO$/i, '')              // "Oakville GO" → "Oakville"
		.replace(/\s+Station$/i, '')         // "Union Station" → "Union"
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/(^-|-$)/g, '');
}

/** Find a stop by its URL slug. Matches against name-derived slugs. */
export function findStopBySlug(stops: Stop[], slug: string): Stop | undefined {
	return stops.find((s) => stopToSlug(s) === slug);
}
