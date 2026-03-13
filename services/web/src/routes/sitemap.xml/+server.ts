import type { RequestHandler } from './$types';

import { getAllStops } from '$lib/api';
import { stopToSlug } from '$lib/stations';

const UNION_CODE = 'UN';

export const GET: RequestHandler = async () => {
	const BASE = 'https://railsix.com';

	let stationUrls = '';
	let commuteUrls = '';
	try {
		const stops = await getAllStops();
		if (Array.isArray(stops)) {
			const slugs = new Set<string>();
			for (const stop of stops) {
				// Departure board URLs
				const slug = stopToSlug(stop);
				if (slug && !slugs.has(slug)) {
					slugs.add(slug);
					stationUrls += `
  <url>
    <loc>${BASE}/departures/${slug}</loc>
    <changefreq>daily</changefreq>
    <priority>0.7</priority>
  </url>`;
				}

				// Commute URLs: every station paired with Union in both directions
				const code = stop.code || stop.id;
				if (code && code !== UNION_CODE) {
					commuteUrls += `
  <url>
    <loc>${BASE}/?from=${UNION_CODE}&amp;to=${encodeURIComponent(code)}&amp;dir=toWork</loc>
    <changefreq>daily</changefreq>
    <priority>0.6</priority>
  </url>
  <url>
    <loc>${BASE}/?from=${encodeURIComponent(code)}&amp;to=${UNION_CODE}&amp;dir=toHome</loc>
    <changefreq>daily</changefreq>
    <priority>0.6</priority>
  </url>`;
				}
			}
		}
	} catch {
		// If API is down, return sitemap without station URLs
	}

	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>${BASE}/</loc>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>${BASE}/departures</loc>
    <changefreq>daily</changefreq>
    <priority>0.9</priority>
  </url>${stationUrls}${commuteUrls}
</urlset>`;

	return new Response(xml, {
		headers: {
			'Content-Type': 'application/xml',
			'Cache-Control': 'max-age=3600'
		}
	});
};
