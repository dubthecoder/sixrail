// web/src/lib/stores/commute.ts
import { browser } from '$app/environment';
import { writable } from 'svelte/store';
import { torontoHour } from '$lib/display';

export interface CommuteTrip {
	originCode: string;
	originName: string;
	destinationCode: string;
	destinationName: string;
}

export interface CommuteStore {
	toWork: CommuteTrip | null;
	toHome: CommuteTrip | null;
}

function safeParse<T>(key: string, fallback: T): T {
	try {
		const raw = localStorage.getItem(key);
		return raw ? JSON.parse(raw) : fallback;
	} catch {
		localStorage.removeItem(key);
		return fallback;
	}
}

function createCommuteStore() {
	const empty: CommuteStore = { toWork: null, toHome: null };
	const { subscribe, set, update } = writable<CommuteStore>(empty);

	return {
		subscribe,
		/** Call once from onMount to load saved state from localStorage */
		hydrate() {
			if (browser) {
				const saved = safeParse<CommuteStore | null>('commute', null);
				if (saved) set(saved);
			}
		},
		setTrip(direction: 'toWork' | 'toHome', trip: CommuteTrip) {
			update((s) => {
				const next = { ...s, [direction]: trip };
				if (browser) localStorage.setItem('commute', JSON.stringify(next));
				return next;
			});
		},
		clear() {
			if (browser) localStorage.removeItem('commute');
			set(empty);
		}
	};
}

export const commute = createCommuteStore();

/** Returns which direction to show based on time of day, respecting manual override */
export function getActiveDirection(
	override: 'toWork' | 'toHome' | null,
	commuteState?: CommuteStore
): 'toWork' | 'toHome' {
	if (override) return override;

	const preferWork = torontoHour() < 12;
	if (!commuteState?.toWork && !commuteState?.toHome) {
		return preferWork ? 'toWork' : 'toHome';
	}

	if (preferWork) {
		return commuteState?.toWork ? 'toWork' : 'toHome';
	}

	return commuteState?.toHome ? 'toHome' : 'toWork';
}
