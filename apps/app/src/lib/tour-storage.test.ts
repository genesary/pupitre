import { describe, expect, it } from 'vitest';

import {
  readTourState,
  recordLessonTip,
  recordTourStatus,
  shouldOfferTour,
  shouldShowLessonTip,
} from '@/lib/tour-storage';
import { TOUR_VERSION } from '@/lib/tour-steps';

const USER = 'u1';
const KEY = `pupitre.tour.${USER}`;

describe('guided tour persistence (spec 007)', () => {
  it('offers the tour when nothing is stored', () => {
    expect(readTourState(USER)).toBeNull();
    expect(shouldOfferTour(readTourState(USER))).toBe(true);
    expect(shouldShowLessonTip(readTourState(USER))).toBe(true);
  });

  it('treats invalid storage as never offered', () => {
    window.localStorage.setItem(KEY, '{not json');
    expect(shouldOfferTour(readTourState(USER))).toBe(true);
    window.localStorage.setItem(KEY, '"text"');
    expect(shouldOfferTour(readTourState(USER))).toBe(true);
  });

  it('does not offer the tour again once answered', () => {
    for (const status of ['offered', 'started', 'completed', 'dismissed'] as const) {
      recordTourStatus(USER, status);
      expect(shouldOfferTour(readTourState(USER))).toBe(false);
    }
  });

  it('offers the tour again when it changed since', () => {
    window.localStorage.setItem(KEY, JSON.stringify({ version: TOUR_VERSION - 1, status: 'completed' }));
    expect(shouldOfferTour(readTourState(USER))).toBe(true);
  });

  it('keeps the lesson tip and the tour apart', () => {
    recordLessonTip(USER);
    expect(shouldShowLessonTip(readTourState(USER))).toBe(false);
    expect(shouldOfferTour(readTourState(USER))).toBe(true);

    recordTourStatus(USER, 'completed');
    expect(shouldShowLessonTip(readTourState(USER))).toBe(false);
  });

  it('keeps one state per user', () => {
    recordTourStatus(USER, 'completed');
    expect(shouldOfferTour(readTourState('u2'))).toBe(true);
  });
});
