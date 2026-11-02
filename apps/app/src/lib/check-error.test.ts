import { describe, expect, it } from 'vitest';

import { ApiError } from '@pupitre/core/lib/api-client';

import { checkErrorMessage } from '@/lib/check-error';

describe('checkErrorMessage', () => {
  it('tells a learner over the lab check limit how long to wait', () => {
    expect(checkErrorMessage(new ApiError(429, 'Too many lab checks, retry in 60 s'))).toBe(
      "Trop de vérifications d'affilée : réessayez dans 60 s.",
    );
    expect(checkErrorMessage(new ApiError(429, 'Too Many Requests'))).toBe(
      "Trop de vérifications d'affilée : réessayez dans un instant.",
    );
  });

  it('shows any other error as it is', () => {
    expect(checkErrorMessage(new ApiError(502, 'GITLAB_TOKEN not configured'))).toBe('GITLAB_TOKEN not configured');
    expect(checkErrorMessage(null)).toBeUndefined();
  });
});
