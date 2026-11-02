import type { ModuleSummary } from '@pupitre/core/lib/api-types';

// Module navigation rules shared by the course page, the lesson player and the
// quiz page (docs/specs/011, 012).

/** Page of a module: quizzes have their own page, everything else is a lesson. */
export function moduleHref(slug: string, module: Pick<ModuleSummary, 'type' | 'index'>): string {
  return module.type === 'quiz' ? `/courses/${slug}/quiz/${module.index}` : `/courses/${slug}/lessons/${module.index}`;
}

/** A module is done when its quiz is passed, or when it has been viewed. */
export function isModuleDone(module: ModuleSummary): boolean {
  return module.type === 'quiz' ? Boolean(module.passed) : module.viewed;
}

/** The lesson an inline quiz is embedded in: the last non-inline module before it. */
export function hostOf(modules: ModuleSummary[], quiz: ModuleSummary): ModuleSummary | undefined {
  return modules.filter((m) => m.index < quiz.index && !m.inline).at(-1);
}

/** Inline quizzes keyed by the index of their host lesson. */
export function inlineQuizzesByHost(modules: ModuleSummary[]): Map<number, ModuleSummary> {
  const map = new Map<number, ModuleSummary>();
  for (const quiz of modules.filter((m) => m.inline)) {
    const host = hostOf(modules, quiz);
    if (host) map.set(host.index, quiz);
  }
  return map;
}

export function moduleIcon(module: ModuleSummary): string | null {
  if (module.locked) return '🔒';
  if (module.type === 'video') return '🎬';
  if (module.type === 'image') return '🖼️';
  if (module.type === 'quiz') return '🧩';
  if (module.type === 'lab') return '🧪';
  return null;
}
