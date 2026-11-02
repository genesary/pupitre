// XP levels of user-service/internal/handlers/xp.go, duplicated as the Astro
// my-xp.astro page did: they must stay identical to the backend (defect A11,
// docs/specs/020-learner-space.md).

export type XpLevel = { level: number; name: string; icon: string; threshold: number };

export const XP_LEVELS: XpLevel[] = [
  { level: 1, name: 'Novice', icon: '🌱', threshold: 0 },
  { level: 2, name: 'Apprenant', icon: '📘', threshold: 200 },
  { level: 3, name: 'Confirmé', icon: '⚙️', threshold: 600 },
  { level: 4, name: 'Expert', icon: '🔥', threshold: 1500 },
  { level: 5, name: 'Maître', icon: '🏆', threshold: 3500 },
];

/** The level of a total, as xpLevel() computes it on the server. */
export function levelOf(total: number): XpLevel {
  return [...XP_LEVELS].reverse().find((level) => total >= level.threshold) ?? XP_LEVELS[0]!;
}

export type XpProgress = {
  /** Percentage of the way to the next level, 100 at the last level. */
  percent: number;
  next: XpLevel | null;
  /** XP still needed for the next level. */
  missing: number;
};

/** Progress towards the level after `level` (the server's value). */
export function xpProgress(total: number, level: number): XpProgress {
  const current = XP_LEVELS[level - 1] ?? levelOf(total);
  const next = XP_LEVELS[current.level] ?? null;
  if (!next) return { percent: 100, next: null, missing: 0 };
  const percent = Math.min(100, Math.max(0, Math.round(((total - current.threshold) / (next.threshold - current.threshold)) * 100)));
  return { percent, next, missing: next.threshold - total };
}

export const XP_SOURCE_ICONS: Record<string, string> = { lesson: '📖', module: '🎯', course: '🎓' };
export const XP_SOURCE_LABELS: Record<string, string> = { lesson: 'Leçon', module: 'Module', course: 'Cours' };
