import { describe, expect, it } from 'vitest';

import type { ModuleSummary } from '@pupitre/core/lib/api-types';

import { hostOf, inlineQuizzesByHost, isModuleDone, moduleHref } from '@/lib/module-links';

const mod = (index: number, type: ModuleSummary['type'], extra: Partial<ModuleSummary> = {}): ModuleSummary => ({
  index,
  name: `m${index}`,
  slug: `m${index}`,
  type,
  viewed: false,
  hidden: false,
  locked: false,
  ...extra,
});

describe('module links (spec 011)', () => {
  const modules = [mod(0, 'text'), mod(1, 'text'), mod(2, 'quiz', { inline: true }), mod(3, 'quiz')];

  it('sends quizzes to the quiz page and the rest to the lesson player', () => {
    expect(moduleHref('k8s', modules[3]!)).toBe('/courses/k8s/quiz/3');
    expect(moduleHref('k8s', modules[1]!)).toBe('/courses/k8s/lessons/1');
  });

  it('finds the host lesson of an inline quiz', () => {
    expect(hostOf(modules, modules[2]!)?.index).toBe(1);
    expect(inlineQuizzesByHost(modules).get(1)?.index).toBe(2);
  });

  it('counts a quiz as done when passed, a lesson when viewed', () => {
    expect(isModuleDone(mod(0, 'quiz', { viewed: true }))).toBe(false);
    expect(isModuleDone(mod(0, 'quiz', { passed: true }))).toBe(true);
    expect(isModuleDone(mod(0, 'text', { viewed: true }))).toBe(true);
  });
});
