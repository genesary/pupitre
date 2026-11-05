import { describe, expect, it } from 'vitest';

import { dayKey, monthGrid, toLocalInput } from '@/lib/calendar';

describe('agenda calendar', () => {
  it('starts weeks on Monday and pads with the next month', () => {
    // November 2026 starts on a Sunday.
    const grid = monthGrid(2026, 10);
    expect(grid).toHaveLength(42);
    expect(grid[0]).toEqual({ key: '2026-10-26', day: 26, otherMonth: true });
    expect(grid[6]).toEqual({ key: '2026-11-01', day: 1, otherMonth: false });
    expect(grid.at(-1)).toEqual({ key: '2026-12-06', day: 6, otherMonth: true });
  });

  it('has no padding when the month starts on Monday', () => {
    // February 2027 starts on a Monday and has 28 days.
    const grid = monthGrid(2027, 1);
    expect(grid).toHaveLength(28);
    expect(grid[0]!.key).toBe('2027-02-01');
  });

  it('round-trips a session date through the form in local time', () => {
    const local = new Date(2026, 10, 3, 9, 30);
    expect(toLocalInput(local.toISOString())).toBe('2026-11-03T09:30');
    expect(new Date('2026-11-03T09:30').toISOString()).toBe(local.toISOString());
    expect(dayKey(local)).toBe('2026-11-03');
    expect(toLocalInput('not a date')).toBe('');
  });
});
