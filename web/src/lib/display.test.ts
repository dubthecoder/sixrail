import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Departure } from './api-client';
import {
	compactPlatform,
	padCenter,
	padRight,
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
