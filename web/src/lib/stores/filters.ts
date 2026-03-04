import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export interface FilterState {
	showTrains: boolean;
	showBuses: boolean;
	activeRoutes: string[];
	activeStatuses: string[];
}

const defaultFilters: FilterState = {
	showTrains: true,
	showBuses: true,
	activeRoutes: [],
	activeStatuses: []
};

function createFilters() {
	const initial: FilterState = browser
		? { ...defaultFilters, ...JSON.parse(localStorage.getItem('filters') || '{}') }
		: defaultFilters;
	const { subscribe, set, update } = writable<FilterState>(initial);

	function persist(state: FilterState) {
		if (browser) localStorage.setItem('filters', JSON.stringify(state));
	}

	return {
		subscribe,
		toggleTrains() {
			update((s) => {
				const next = { ...s, showTrains: !s.showTrains };
				persist(next);
				return next;
			});
		},
		toggleBuses() {
			update((s) => {
				const next = { ...s, showBuses: !s.showBuses };
				persist(next);
				return next;
			});
		},
		toggleRoute(routeName: string) {
			update((s) => {
				const routes = s.activeRoutes.includes(routeName)
					? s.activeRoutes.filter((r) => r !== routeName)
					: [...s.activeRoutes, routeName];
				const next = { ...s, activeRoutes: routes };
				persist(next);
				return next;
			});
		},
		setAllRoutes(routeNames: string[]) {
			update((s) => {
				const next = { ...s, activeRoutes: routeNames };
				persist(next);
				return next;
			});
		},
		clearRoutes() {
			update((s) => {
				const next = { ...s, activeRoutes: [] };
				persist(next);
				return next;
			});
		},
		toggleStatus(status: string) {
			update((s) => {
				const statuses = s.activeStatuses.includes(status)
					? s.activeStatuses.filter((st) => st !== status)
					: [...s.activeStatuses, status];
				const next = { ...s, activeStatuses: statuses };
				persist(next);
				return next;
			});
		},
		reset() {
			persist(defaultFilters);
			set(defaultFilters);
		}
	};
}

export const filters = createFilters();
