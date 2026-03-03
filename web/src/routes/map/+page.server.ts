// web/src/routes/map/+page.server.ts
import { getTrainPositions } from '$lib/api';

export async function load() {
	try {
		const positions = await getTrainPositions();
		return { positions };
	} catch {
		return { positions: null };
	}
}
