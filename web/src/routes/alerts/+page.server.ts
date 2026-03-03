import { getAlerts } from '$lib/api';

export async function load() {
	try {
		const alerts = await getAlerts();
		return { alerts: Array.isArray(alerts) ? alerts : [] };
	} catch {
		return { alerts: [] };
	}
}
