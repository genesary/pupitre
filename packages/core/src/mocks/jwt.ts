import type { Role } from '../lib/api-types';

// Unsigned JWTs for the mock mode. The frontend never verifies signatures (the
// backend does), it only reads `exp`, so this is enough to exercise the session.

const TOKEN_LIFETIME_SECONDS = 8 * 60 * 60;

function base64url(value: object): string {
  const json = JSON.stringify(value);
  const bytes = new TextEncoder().encode(json);
  let binary = '';
  bytes.forEach((byte) => (binary += String.fromCharCode(byte)));
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function mockToken(userId: string, role: Role, lifetimeSeconds = TOKEN_LIFETIME_SECONDS) {
  const now = Math.floor(Date.now() / 1000);
  const header = base64url({ alg: 'none', typ: 'JWT' });
  const payload = base64url({
    sub: userId,
    role,
    iss: 'user-service',
    aud: ['pupitre-api'],
    iat: now,
    exp: now + lifetimeSeconds,
  });
  return `${header}.${payload}.mock`;
}

export function readMockToken(authorization: string | null): { sub: string; exp: number } | null {
  const token = authorization?.replace(/^Bearer /, '');
  const payload = token?.split('.')[1];
  if (!payload) return null;
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const claims = JSON.parse(atob(base64 + '='.repeat((4 - (base64.length % 4)) % 4)));
    return typeof claims.sub === 'string' && typeof claims.exp === 'number' ? claims : null;
  } catch {
    return null;
  }
}
