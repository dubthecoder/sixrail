import type { Stop } from '$lib/api';

/**
 * Strip GO Transit boilerplate suffixes from a stop name for clean display/slugs.
 * "Brampton Innovation District GO Station Rail" → "Brampton Innovation District"
 * "Oakville GO" → "Oakville"
 * "Union Station GO" → "Union Station"
 * "Hamilton GO Centre" → unchanged (GO is part of the official name, not a suffix)
 */
function stripStopSuffixes(name: string): string {
	return name
		.replace(/\s+GO\s+Station\b.*/i, '') // "… GO Station Rail" → strip from GO Station onward
		.replace(/\s+GO$/i, '') // trailing "GO"
		.replace(/\s+Station$/i, ''); // trailing "Station"
}

/**
 * Human-readable display name — strips GO suffix but keeps "Station" for named stops.
 * "Oakville GO" → "Oakville", "Union Station GO" → "Union Station"
 */
export function stopToDisplayName(stop: Stop): string {
	return stop.name.replace(/\s+GO\s+Station\b.*/i, '').replace(/\s+GO$/i, '') || stop.name;
}

/** Convert a stop name to a URL-friendly slug: "Oakville GO" → "oakville" */
export function stopToSlug(stop: Stop): string {
	return stripStopSuffixes(stop.name)
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/(^-|-$)/g, '');
}

/** Find a stop by its URL slug. Matches against name-derived slugs. */
export function findStopBySlug(stops: Stop[], slug: string): Stop | undefined {
	return stops.find((s) => stopToSlug(s) === slug);
}

/**
 * SEO-optimized display name for page titles and meta descriptions.
 * Handles stations with naming-rights rebrands or geographic ambiguity where the
 * official name doesn't match what commuters search for.
 *
 * Examples:
 *   "Durham College Oshawa GO" → "Oshawa"      (searchers use "oshawa go train")
 *   "Brampton Innovation District GO" → "Brampton"  (searchers use "brampton go train")
 *   "West Harbour GO" → "West Harbour (Hamilton)"   (searchers use "hamilton go train")
 *   "Allandale Waterfront GO" → "Allandale Waterfront (Barrie)"
 *   "Guelph Central GO" → "Guelph"
 */
const SEO_NAME_OVERRIDES: Record<string, string> = {
	'durham-college-oshawa': 'Oshawa',
	'brampton-innovation-district': 'Brampton',
	'west-harbour': 'West Harbour (Hamilton)',
	'allandale-waterfront': 'Allandale Waterfront (Barrie)',
	'guelph-central': 'Guelph'
};

export function stopToSeoName(stop: Stop): string {
	const slug = stopToSlug(stop);
	return SEO_NAME_OVERRIDES[slug] ?? stopToDisplayName(stop);
}
