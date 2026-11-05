import { describe, expect, it } from 'vitest';

import { appOf } from './site';

describe('appOf', () => {
  it('sends /admin and /manager to the admin app', () => {
    expect(appOf('/admin')).toBe('admin');
    expect(appOf('/admin/users?x=1')).toBe('admin');
    expect(appOf('/manager/users')).toBe('admin');
  });

  it('keeps every other path in the learner app', () => {
    expect(appOf('/courses')).toBe('learner');
    expect(appOf('/administration')).toBe('learner');
    expect(appOf('/my-courses#top')).toBe('learner');
    expect(appOf('/login?redirect=%2Fadmin')).toBe('learner');
  });
});
