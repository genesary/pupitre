import { describe, expect, it } from 'vitest';

import { isReadingPage, LESSON_TIP_STEPS, stepPath, tourSteps } from '@/lib/tour-steps';

const ids = (role: Parameters<typeof tourSteps>[0]) => tourSteps(role).map((step) => step.id);

describe('guided tour steps (spec 007)', () => {
  it('shows the manager and admin steps to their role only', () => {
    expect(ids('student')).not.toContain('manager');
    expect(ids('student')).not.toContain('admin');
    expect(ids('student')).toHaveLength(19);
    expect(ids('manager')).toContain('manager');
    expect(ids('manager')).not.toContain('admin');
    expect(ids('admin')).toContain('admin');
    expect(ids('admin')).not.toContain('manager');
  });

  it('visits every page of the learner space, in order', () => {
    const paths = [...new Set(tourSteps('student').map((step) => stepPath(step, 'docker')).filter(Boolean))];
    expect(paths).toEqual([
      '/courses',
      '/courses/docker',
      '/my-courses',
      '/my-paths',
      '/my-groups',
      '/my-badges',
      '/my-xp',
      '/leaderboard',
      '/my-skills',
      '/agenda',
    ]);
  });

  it('gives every step a title and a text', () => {
    for (const step of [...tourSteps('admin'), ...tourSteps('manager'), ...LESSON_TIP_STEPS]) {
      expect(step.title, step.id).toBeTruthy();
      expect(step.text, step.id).toBeTruthy();
    }
  });

  it('is not offered over a lesson or a quiz', () => {
    expect(isReadingPage('/courses/docker/lessons/2')).toBe(true);
    expect(isReadingPage('/courses/docker/quiz/3')).toBe(true);
    expect(isReadingPage('/courses/docker')).toBe(false);
    expect(isReadingPage('/my-courses')).toBe(false);
  });
});
