import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Departure } from './api-client';
import {
	departureDisplayTime,
	departureTargetMs,
	formatCountdown,
	isUpcomingDeparture,
	compactPlatform,
	padCenter,
	padRight,
	shiftHHMM,
	statusClass,
	statusText,
	torontoHour,
	torontoNow
} from './display';

describe('display helpers', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('reads the current Toronto hour and milliseconds since midnight', () => {
		vi.setSystemTime(new Date('2025-01-15T13:05:06Z'));

		expect(torontoHour()).toBe(8);

		const now = torontoNow();
		expect(now.ms).toBe((8 * 3600 + 5 * 60 + 6) * 1000);
		expect(now.todayAt(9, 30)).toBe((9 * 3600 + 30 * 60) * 1000);
	});

	it('pads and compacts board text for split-flap rendering', () => {
		expect(padRight('lw', 5)).toBe('LW   ');
		expect(compactPlatform('11 & 12')).toBe('11&12');
		expect(padCenter('go', 6)).toBe('  GO  ');
	});

	it('uses actual or delayed departure times and handles overnight rollovers', () => {
		const delayed = {
			line: 'LW',
			scheduledTime: '23:55',
			delayMinutes: 20,
			status: 'Delayed +20m'
		} satisfies Departure;
		const actual = { ...delayed, actualTime: '00:07' };

		expect(shiftHHMM('23:55', 20)).toBe('00:15');
		expect(departureDisplayTime(delayed)).toBe('00:15');
		expect(departureDisplayTime(actual)).toBe('00:07');

		const lateNightNow = {
			ms: (23 * 3600 + 50 * 60) * 1000,
			todayAt: (h: number, m: number) => (h * 3600 + m * 60) * 1000
		};
		expect(departureTargetMs('00:15', lateNightNow)).toBe((24 * 3600 + 15 * 60) * 1000);
		expect(isUpcomingDeparture(delayed, lateNightNow)).toBe(true);
		expect(formatCountdown('00:15', lateNightNow)).toBe('25:00');
	});

	it('does not wrap ordinary past departures to the next day', () => {
		const past = {
			line: 'LW',
			scheduledTime: '08:00',
			status: 'On Time'
		} satisfies Departure;
		const morningNow = {
			ms: (8 * 3600 + 30 * 60) * 1000,
			todayAt: (h: number, m: number) => (h * 3600 + m * 60) * 1000
		};

		expect(departureTargetMs('08:00', morningNow)).toBe(8 * 3600 * 1000);
		expect(isUpcomingDeparture(past, morningNow)).toBe(false);
		expect(formatCountdown('08:00', morningNow)).toBe('00:00');
	});

	it('returns cancel and delay states with the expected priority', () => {
		const onTime = {
			line: 'LW',
			scheduledTime: '08:15',
			status: 'On Time'
		} satisfies Departure;
		const delayed = { ...onTime, delayMinutes: 7 };
		const cancelled = { ...delayed, isCancelled: true };

		expect(statusText(onTime)).toBe('ON TIME');
		expect(statusClass(onTime)).toBe('text-green-400');
		expect(statusText(delayed)).toBe('+7M');
		expect(statusClass(delayed)).toBe('text-amber-400');
		expect(statusText(cancelled)).toBe('CANCEL');
		expect(statusClass(cancelled)).toBe('text-red-500');
	});
});
