import type { Course } from '@pupitre/core/lib/api-types';
import { DIFFICULTY_LABELS, LEVEL_ORDER } from '@pupitre/core/lib/course-meta';

// The difficulty range slider of the catalogue (docs/specs/010-catalogue.md,
// "Difficulty"): levels 1 to 3, in LEVEL_ORDER.

export const LEVELS = ['beginner', 'intermediate', 'advanced'] as const;

export type DifficultyRange = [number, number];

export const ALL_LEVELS: DifficultyRange = [1, LEVELS.length];

export function isFullRange([low, high]: DifficultyRange): boolean {
  return low === 1 && high === LEVELS.length;
}

export function levelLabel(order: number): string {
  const level = LEVELS[order - 1];
  return level ? DIFFICULTY_LABELS[level]! : '';
}

export function rangeLabel(range: DifficultyRange): string {
  const [low, high] = range;
  if (isFullRange(range)) return 'Toutes';
  if (low === high) return levelLabel(low);
  return `${levelLabel(low)} → ${levelLabel(high)}`;
}

/** The full range keeps every course, those without a known difficulty included. */
export function matchesDifficulty(course: Pick<Course, 'difficulty'>, range: DifficultyRange): boolean {
  if (isFullRange(range)) return true;
  const order = LEVEL_ORDER[course.difficulty] ?? 0;
  return order >= range[0] && order <= range[1];
}
