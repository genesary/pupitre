import { readItem, writeItem } from '@pupitre/ui/lib/storage';

import { TOUR_VERSION } from '@/lib/tour-steps';

// What the browser remembers of the guided tour, per user
// (docs/specs/007-guided-tour.md, "Persistence"). No backend route stores it.

export type TourStatus = 'offered' | 'started' | 'completed' | 'dismissed';

export type TourState = {
  version: number;
  status?: TourStatus;
  lessonTip?: 'seen';
};

const STATUSES: TourStatus[] = ['offered', 'started', 'completed', 'dismissed'];

const keyOf = (userId: string) => `pupitre.tour.${userId}`;

/** The stored state, or null when absent or unreadable. */
export function readTourState(userId: string): TourState | null {
  const raw = readItem('local', keyOf(userId));
  if (!raw) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (typeof value !== 'object' || value === null) return null;
    const { version, status, lessonTip } = value as Record<string, unknown>;
    if (typeof version !== 'number') return null;
    return {
      version,
      status: STATUSES.includes(status as TourStatus) ? (status as TourStatus) : undefined,
      lessonTip: lessonTip === 'seen' ? 'seen' : undefined,
    };
  } catch {
    return null;
  }
}

function write(userId: string, patch: Partial<TourState>) {
  const current = readTourState(userId);
  writeItem('local', keyOf(userId), JSON.stringify({ ...current, ...patch }));
}

export function recordTourStatus(userId: string, status: TourStatus) {
  write(userId, { version: TOUR_VERSION, status });
}

export function recordLessonTip(userId: string) {
  write(userId, { version: readTourState(userId)?.version ?? 0, lessonTip: 'seen' });
}

/** The tour is offered when it was never offered, or when it changed since. */
export function shouldOfferTour(state: TourState | null): boolean {
  return !state?.status || state.version < TOUR_VERSION;
}

export function shouldShowLessonTip(state: TourState | null): boolean {
  return state?.lessonTip !== 'seen';
}
