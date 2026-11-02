import { describe, expect, it } from 'vitest';

import { ALL_LEVELS, matchesDifficulty, rangeLabel } from '@/lib/difficulty-range';

const course = (difficulty: string) => ({ difficulty }) as Parameters<typeof matchesDifficulty>[0];

describe('catalogue difficulty range (spec 010)', () => {
  it('keeps every course on the full range, those without a difficulty included', () => {
    expect(matchesDifficulty(course(''), ALL_LEVELS)).toBe(true);
    expect(matchesDifficulty(course('expert'), ALL_LEVELS)).toBe(true);
    expect(matchesDifficulty(course('advanced'), ALL_LEVELS)).toBe(true);
  });

  it('drops a course without a known difficulty from a narrower range', () => {
    expect(matchesDifficulty(course(''), [2, 3])).toBe(false);
    expect(matchesDifficulty(course('expert'), [1, 2])).toBe(false);
  });

  it('keeps the levels inside the range only', () => {
    expect(matchesDifficulty(course('intermediate'), [2, 2])).toBe(true);
    expect(matchesDifficulty(course('beginner'), [2, 2])).toBe(false);
    expect(matchesDifficulty(course('advanced'), [2, 2])).toBe(false);
    expect(matchesDifficulty(course('beginner'), [1, 2])).toBe(true);
    expect(matchesDifficulty(course('advanced'), [2, 3])).toBe(true);
  });

  it('names the range', () => {
    expect(rangeLabel([1, 3])).toBe('Toutes');
    expect(rangeLabel([2, 2])).toBe('Intermédiaire');
    expect(rangeLabel([2, 3])).toBe('Intermédiaire → Avancé');
    expect(rangeLabel([1, 2])).toBe('Débutant → Intermédiaire');
  });
});
