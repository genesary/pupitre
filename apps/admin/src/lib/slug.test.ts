import { describe, expect, it } from 'vitest';

import { slugError } from '@/lib/slug';

describe('slug rule (spec 042)', () => {
  it('accepts lowercase words joined by hyphens', () => {
    expect(slugError('test-cours')).toBeNull();
    expect(slugError('k8s-101')).toBeNull();
  });

  it('refuses spaces, capitals, stray hyphens and reserved segments', () => {
    expect(slugError('Test Cours')).toMatch(/minuscules/);
    expect(slugError('-cours')).not.toBeNull();
    expect(slugError('cours--test')).not.toBeNull();
    expect(slugError('batch')).toMatch(/réservé/);
    expect(slugError('')).toMatch(/requis/);
  });
});
