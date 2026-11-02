import type { PublicSettings, UserPublic } from '../lib/api-types';
import { db } from './db';

// Fixtures of the mock mode (`pnpm dev:mock`). Shapes follow the Go response
// structs, so a page built against them works unchanged with the real backend.

export type MockAccount = {
  user: UserPublic;
  /** Shown on the mock Keycloak page. */
  description: string;
};

const createdAt = '2026-09-01T09:00:00Z';

export const MOCK_ACCOUNTS: MockAccount[] = [
  {
    description: 'Apprenant',
    user: {
      id: '7a1c3e52-5f0b-4c2e-9d61-0b8f1a2c3d41',
      username: 'alice',
      email: 'alice@pupitre.test',
      role: 'student',
      avatarUrl: '🦊',
      bio: 'Apprenante en DevOps.',
      isActive: true,
      authProvider: 'oidc',
      createdAt,
    },
  },
  {
    description: 'Manager de groupe',
    user: {
      id: '2b9d4f61-8c3a-4e7b-a152-6d0e9f8a7b32',
      username: 'marc.manager',
      email: 'marc@pupitre.test',
      role: 'manager',
      avatarUrl: null,
      bio: null,
      isActive: true,
      authProvider: 'oidc',
      createdAt,
    },
  },
  {
    description: 'Administrateur',
    user: {
      id: '0f6e2d18-3b7c-4a95-8e24-c1d3b5a7f960',
      username: 'admin',
      email: 'admin@pupitre.test',
      role: 'admin',
      avatarUrl: '🦉',
      bio: null,
      isActive: true,
      authProvider: 'oidc',
      createdAt,
    },
  },
  {
    // Reproduces audit findings C2 and C4: these values must show as text and
    // never run (spec 003). An alert box means a regression.
    description: 'Compte piégé (test XSS)',
    user: {
      id: '9c8b7a65-4d3e-4f21-b0a9-8e7d6c5b4a39',
      username: '<img src=x onerror=alert("xss-username")>',
      email: 'piege@pupitre.test',
      role: 'student',
      avatarUrl: 'https://x" onerror="alert(\'xss-avatar\')',
      bio: '<script>alert("xss-bio")</script>',
      isActive: true,
      authProvider: 'oidc',
      createdAt,
    },
  },
];

export const MOCK_PUBLIC_SETTINGS: PublicSettings = {
  registration_enabled: 'false',
  sso_local_login_enabled: 'false',
  password_min_length: '8',
  password_require_uppercase: 'false',
  password_require_number: 'false',
  oidc_enabled: 'true',
  ldap_enabled: 'false',
};

/** The account, with the changes saved by PUT /api/auth/profile or an admin; none once deleted. */
export function findAccount(id: string | null | undefined): MockAccount | undefined {
  const account = MOCK_ACCOUNTS.find((candidate) => candidate.user.id === id);
  if (!account || db().deletedUsers.includes(account.user.id)) return undefined;
  const changes = db().profiles[account.user.id];
  return changes ? { ...account, user: { ...account.user, ...changes } } : account;
}

/** Every account that was not deleted, with its changes. */
export function accounts(): MockAccount[] {
  return MOCK_ACCOUNTS.flatMap((account) => findAccount(account.user.id) ?? []);
}
