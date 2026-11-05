import { describe, expect, it, vi } from 'vitest';

import type { UserPublic } from './api-types';
import { mockToken } from '../mocks/jwt';

import { endSession, getSession, openSession, setSessionEndHandler, tokenExpiry } from './session';

const user: UserPublic = {
  id: 'u1',
  username: 'alice',
  email: 'alice@pupitre.test',
  role: 'student',
  avatarUrl: null,
  bio: null,
  isActive: true,
  authProvider: 'oidc',
  createdAt: '2026-01-01T00:00:00Z',
};

describe('tokenExpiry', () => {
  it('reads exp in milliseconds', () => {
    const before = Date.now();
    const expiry = tokenExpiry(mockToken('u1', 'student', 60));
    expect(expiry).toBeGreaterThanOrEqual(before + 59_000);
    expect(expiry).toBeLessThanOrEqual(before + 61_000);
  });

  it.each(['', 'not-a-jwt', 'a.!!!.c'])('returns null for %j', (token) => {
    expect(tokenExpiry(token)).toBeNull();
  });
});

describe('session', () => {
  it('keeps the session in sessionStorage, never localStorage', () => {
    openSession({ token: mockToken('u1', 'student'), user });
    expect(getSession()?.user.username).toBe('alice');
    expect(window.sessionStorage.getItem('pupitre.session')).toContain('alice');
    expect(window.localStorage.length).toBe(0);
    endSession('logout');
  });

  it('ends itself when the token expires', () => {
    vi.useFakeTimers();
    const handler = vi.fn();
    setSessionEndHandler(handler);
    openSession({ token: mockToken('u1', 'student', 5), user });
    vi.advanceTimersByTime(6_000);
    expect(getSession()).toBeNull();
    expect(handler).toHaveBeenCalledWith('expired');
    vi.useRealTimers();
  });
});
