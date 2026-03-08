import type { Departure } from '$lib/api-client';

export function padRight(str: string, len: number): string {
	return str.toUpperCase().padEnd(len, ' ').slice(0, len);
}

export function padCenter(str: string, len: number): string {
	const s = str.toUpperCase().slice(0, len);
	const left = Math.floor((len - s.length) / 2);
	return s.padStart(s.length + left, ' ').padEnd(len, ' ');
}

export function statusText(d: Departure): string {
	if (d.isCancelled) return 'CANCEL';
	if (d.delayMinutes && d.delayMinutes > 0) return `+${d.delayMinutes}M`;
	return 'ON TIME';
}

export function statusClass(d: Departure): string {
	if (d.isCancelled) return 'text-red-500';
	if (d.delayMinutes && d.delayMinutes > 0) return 'text-amber-400';
	return 'text-green-400';
}

export function occupancyIcon(status: string | undefined): string {
	if (!status) return '';
	switch (status) {
		case 'MANY_SEATS_AVAILABLE':
			return '\u25CB';
		case 'FEW_SEATS_AVAILABLE':
			return '\u25D1';
		case 'STANDING_ROOM_ONLY':
		case 'CRUSHED_STANDING_ROOM_ONLY':
		case 'FULL':
			return '\u25CF';
		default:
			return '';
	}
}

export function occupancyClass(status: string | undefined): string {
	if (!status) return '';
	switch (status) {
		case 'MANY_SEATS_AVAILABLE':
			return 'text-green-400';
		case 'FEW_SEATS_AVAILABLE':
			return 'text-amber-400';
		case 'STANDING_ROOM_ONLY':
		case 'CRUSHED_STANDING_ROOM_ONLY':
		case 'FULL':
			return 'text-red-400';
		default:
			return '';
	}
}

export function occupancyLabel(status: string | undefined): { text: string; cls: string } {
	if (!status) return { text: '', cls: '' };
	switch (status) {
		case 'MANY_SEATS_AVAILABLE':
			return { text: '▪ SEATS AVAIL', cls: 'text-green-400' };
		case 'FEW_SEATS_AVAILABLE':
			return { text: '▪▪ FEW SEATS', cls: 'text-amber-400' };
		case 'STANDING_ROOM_ONLY':
			return { text: '▪▪▪ STANDING', cls: 'text-amber-400' };
		case 'CRUSHED_STANDING_ROOM_ONLY':
		case 'FULL':
			return { text: '▪▪▪ FULL', cls: 'text-red-400' };
		case 'NOT_ACCEPTING_PASSENGERS':
			return { text: '✕ NOT BOARDING', cls: 'text-red-500' };
		default:
			return { text: '', cls: '' };
	}
}
