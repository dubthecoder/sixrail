import { json } from '@sveltejs/kit';
import { getAlerts } from '$lib/api';

export async function GET() {
	try {
		const alerts = await getAlerts();
		return json(alerts);
	} catch {
		return json([], { status: 502 });
	}
}
