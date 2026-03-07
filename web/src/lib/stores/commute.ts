// web/src/lib/stores/commute.ts
import { browser } from '$app/environment';
import { writable } from 'svelte/store';

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

export interface NotificationPrefs {
	enabled: boolean;
	thresholdMinutes: 5 | 10 | 15;
}

function createCommuteStore() {
	const initial: CommuteStore = browser
		? (JSON.parse(localStorage.getItem('commute') || 'null') ?? { toWork: null, toHome: null })
		: { toWork: null, toHome: null };

	const { subscribe, set, update } = writable<CommuteStore>(initial);

	return {
		subscribe,
		setTrip(direction: 'toWork' | 'toHome', trip: CommuteTrip) {
			update((s) => {
				const next = { ...s, [direction]: trip };
				if (browser) localStorage.setItem('commute', JSON.stringify(next));
				return next;
			});
		},
		clear() {
			const empty = { toWork: null, toHome: null };
			if (browser) localStorage.removeItem('commute');
			set(empty);
		}
	};
}

function createNotificationStore() {
	const initial: NotificationPrefs = browser
		? (JSON.parse(localStorage.getItem('notificationPrefs') || 'null') ?? {
				enabled: false,
				thresholdMinutes: 5
			})
		: { enabled: false, thresholdMinutes: 5 };

	const { subscribe, update } = writable<NotificationPrefs>(initial);

	return {
		subscribe,
		setEnabled(enabled: boolean) {
			update((s) => {
				const next = { ...s, enabled };
				if (browser) localStorage.setItem('notificationPrefs', JSON.stringify(next));
				return next;
			});
		},
		setThreshold(thresholdMinutes: 5 | 10 | 15) {
			update((s) => {
				const next = { ...s, thresholdMinutes };
				if (browser) localStorage.setItem('notificationPrefs', JSON.stringify(next));
				return next;
			});
		}
	};
}

export const commute = createCommuteStore();
export const notificationPrefs = createNotificationStore();

/** Returns which direction to show based on time of day, respecting manual override */
export function getActiveDirection(override: 'toWork' | 'toHome' | null): 'toWork' | 'toHome' {
	if (override) return override;
	return new Date().getHours() < 12 ? 'toWork' : 'toHome';
}
