import type { Departure } from '$lib/api-client';

const dayMs = 24 * 60 * 60 * 1000;
const hourMs = 60 * 60 * 1000;
const minuteMs = 60 * 1000;
const overnightStartHour = 18;
const overnightRollHour = 6;

const torontoFmt = new Intl.DateTimeFormat('en-CA', {
	timeZone: 'America/Toronto',
	hour: 'numeric',
	minute: 'numeric',
	second: 'numeric',
	hour12: false
});

/** Current hour (0-23) in America/Toronto timezone. */
export function torontoHour(): number {
	const parts = torontoFmt.formatToParts(new Date());
	return Number(parts.find((p) => p.type === 'hour')!.value);
}

/**
 * Returns the current time-of-day in America/Toronto as total milliseconds since midnight,
 * plus a helper to build a "today in Toronto" Date for a given HH:MM schedule time.
 */
export function torontoNow(): { ms: number; todayAt: (h: number, m: number) => number } {
	const parts = torontoFmt.formatToParts(new Date());
	const get = (t: string) => Number(parts.find((p) => p.type === t)!.value);
	const hour = get('hour');
	const minute = get('minute');
	const second = get('second');
	const ms = (hour * 3600 + minute * 60 + second) * 1000;
	return {
		ms,
		todayAt: (h: number, m: number) => (h * 3600 + m * 60) * 1000
	};
}

function parseHHMM(hhmm: string): { hour: number; minute: number } | null {
	const [hour, minute] = hhmm.split(':').map(Number);
	if (
		Number.isNaN(hour) ||
		Number.isNaN(minute) ||
		hour < 0 ||
		hour > 23 ||
		minute < 0 ||
		minute > 59
	) {
		return null;
	}
	return { hour, minute };
}

export function shiftHHMM(hhmm: string, minutes: number): string {
	const parsed = parseHHMM(hhmm);
	if (!parsed) return hhmm;

	const totalMinutes = parsed.hour * 60 + parsed.minute + minutes;
	const normalized = ((totalMinutes % (24 * 60)) + 24 * 60) % (24 * 60);
	const hour = Math.floor(normalized / 60);
	const minute = normalized % 60;

	return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

export function departureDisplayTime(d: Departure): string {
	if (d.actualTime) return d.actualTime;
	if (d.delayMinutes && d.delayMinutes > 0) {
		return shiftHHMM(d.scheduledTime, d.delayMinutes);
	}
	return d.scheduledTime;
}

export function departureTargetMs(
	hhmm: string,
	now: ReturnType<typeof torontoNow> = torontoNow()
): number {
	const parsed = parseHHMM(hhmm);
	if (!parsed) return now.ms;

	let targetMs = now.todayAt(parsed.hour, parsed.minute);
	const nowHour = Math.floor(now.ms / hourMs);

	if (targetMs <= now.ms && nowHour >= overnightStartHour && parsed.hour < overnightRollHour) {
		targetMs += dayMs;
	}

	return targetMs;
}

export function isUpcomingDeparture(
	departure: Departure,
	now: ReturnType<typeof torontoNow> = torontoNow()
): boolean {
	return departureTargetMs(departureDisplayTime(departure), now) > now.ms;
}

export function formatCountdown(
	hhmm: string,
	now: ReturnType<typeof torontoNow> = torontoNow()
): string {
	const diffMs = Math.max(0, departureTargetMs(hhmm, now) - now.ms);
	const totalMins = Math.floor(diffMs / minuteMs);
	const secs = Math.floor((diffMs % minuteMs) / 1000);
	if (totalMins >= 60) {
		const hrs = Math.floor(totalMins / 60);
		const mins = totalMins % 60;
		return `${hrs}h ${String(mins).padStart(2, '0')}m`;
	}
	return `${String(totalMins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

export function padRight(str: string, len: number): string {
	return str.toUpperCase().padEnd(len, ' ').slice(0, len);
}

/** Compact platform strings like "11 & 12" → "11&12" to fit display width. */
export function compactPlatform(p: string): string {
	return p.replace(/\s*&\s*/g, '&');
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
