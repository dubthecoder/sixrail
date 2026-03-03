// web/src/lib/stores/favorites.ts
import { browser } from '$app/environment';
import { writable } from 'svelte/store';

function createFavorites() {
	const initial = browser ? JSON.parse(localStorage.getItem('favorites') || '[]') : [];
	const { subscribe, set, update } = writable<string[]>(initial);

	return {
		subscribe,
		toggle(stopCode: string) {
			update((faves) => {
				const next = faves.includes(stopCode)
					? faves.filter((f) => f !== stopCode)
					: [...faves, stopCode];
				if (browser) localStorage.setItem('favorites', JSON.stringify(next));
				return next;
			});
		}
	};
}

export const favorites = createFavorites();

function createDefaultStation() {
	const initial = browser ? localStorage.getItem('defaultStation') || '' : '';
	const { subscribe, set } = writable<string>(initial);

	return {
		subscribe,
		set(code: string) {
			if (browser) localStorage.setItem('defaultStation', code);
			set(code);
		}
	};
}

export const defaultStation = createDefaultStation();
