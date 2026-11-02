import { describe, expect, it } from 'vitest';

import { levelOf, xpProgress } from '@/lib/xp';

describe('xp levels', () => {
  it('matches the thresholds of xp.go', () => {
    expect(levelOf(0).name).toBe('Novice');
    expect(levelOf(199).level).toBe(1);
    expect(levelOf(200).name).toBe('Apprenant');
    expect(levelOf(650).name).toBe('Confirmé');
    expect(levelOf(3500).name).toBe('Maître');
  });

  it('shows 850 XP to Expert at 650 XP (spec 020)', () => {
    const progress = xpProgress(650, 3);
    expect(progress.next?.level).toBe(4);
    expect(progress.next?.name).toBe('Expert');
    expect(progress.missing).toBe(850);
    expect(progress.percent).toBe(6);
  });

  it('stops at the last level', () => {
    expect(xpProgress(5000, 5)).toEqual({ percent: 100, next: null, missing: 0 });
  });
});
