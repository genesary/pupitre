import { beforeEach, describe, expect, it } from 'vitest';

import { setCourseReturn, takeCourseReturn } from '@/lib/course-return';

describe('takeCourseReturn', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('returns a same-site path once', () => {
    setCourseReturn('/my-paths?slug=devops');
    expect(takeCourseReturn()).toBe('/my-paths?slug=devops');
    expect(takeCourseReturn()).toBeNull();
  });

  it.each(['https://evil.test/', '//evil.test', '/\\evil.test', '/.//evil.test'])('refuses %s', (path) => {
    setCourseReturn(path);
    expect(takeCourseReturn()).toBeNull();
  });
});
