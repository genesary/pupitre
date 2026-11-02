import { useSyncExternalStore } from 'react';

import type { AuthResponse, UserPublic } from './api-types';
import { readItem, removeItem, writeItem } from '@pupitre/ui/lib/storage';

// The session of docs/specs/001-keycloak-authentication.md. This module is the
// only code that knows where the token lives: moving to an HttpOnly cookie later
// only changes this file.
//
// Interim storage is sessionStorage: the session ends with the tab and is not
// shared between tabs, unlike the localStorage of the Astro frontend.

const STORAGE_KEY = 'pupitre.session';

export type Session = {
  token: string;
  user: UserPublic;
  /** Token expiry, in milliseconds since the epoch, or null if unknown. */
  expiresAt: number | null;
};

type SessionEndReason = 'expired' | 'unauthorized' | 'logout';
type SessionEndHandler = (reason: SessionEndReason) => void;

const listeners = new Set<() => void>();
let session: Session | null = load();
let expiryTimer: ReturnType<typeof setTimeout> | undefined;
let onSessionEnd: SessionEndHandler = () => {};

/** Reads `exp` from a JWT payload. No signature check: the server does that. */
export function tokenExpiry(token: string): number | null {
  const payload = token.split('.')[1];
  if (!payload) return null;
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
    const claims = JSON.parse(atob(padded)) as { exp?: unknown };
    return typeof claims.exp === 'number' ? claims.exp * 1000 : null;
  } catch {
    return null;
  }
}

function isExpired(expiresAt: number | null, now = Date.now()): boolean {
  return expiresAt !== null && expiresAt <= now;
}

function load(): Session | null {
  const raw = readItem('session', STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Session;
    if (typeof parsed?.token !== 'string' || !parsed.user) return null;
    if (isExpired(parsed.expiresAt)) {
      removeItem('session', STORAGE_KEY);
      return null;
    }
    return parsed;
  } catch {
    removeItem('session', STORAGE_KEY);
    return null;
  }
}

function emit() {
  listeners.forEach((listener) => listener());
}

function scheduleExpiry() {
  clearTimeout(expiryTimer);
  if (!session?.expiresAt) return;
  // setTimeout overflows above ~24.8 days; tokens last hours, but stay safe.
  const delay = Math.min(session.expiresAt - Date.now(), 2 ** 31 - 1);
  expiryTimer = setTimeout(() => endSession('expired'), Math.max(delay, 0));
}

function persist(next: Session | null) {
  session = next;
  if (next) writeItem('session', STORAGE_KEY, JSON.stringify(next));
  else removeItem('session', STORAGE_KEY);
  scheduleExpiry();
  emit();
}

export function getSession(): Session | null {
  return session;
}

export function getToken(): string | null {
  return session?.token ?? null;
}

export function openSession(auth: AuthResponse) {
  persist({ token: auth.token, user: auth.user, expiresAt: tokenExpiry(auth.token) });
}

/** Replaces the stored user, e.g. after GET /api/auth/me or a profile update. */
export function updateUser(user: UserPublic) {
  if (!session) return;
  persist({ ...session, user });
}

export function endSession(reason: SessionEndReason) {
  if (!session) return;
  persist(null);
  onSessionEnd(reason);
}

/** Called once at start-up with the navigation and toast logic of the app. */
export function setSessionEndHandler(handler: SessionEndHandler) {
  onSessionEnd = handler;
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useSession(): Session | null {
  return useSyncExternalStore(subscribe, getSession, getSession);
}

export function useCurrentUser(): UserPublic | null {
  return useSession()?.user ?? null;
}

// The expiry timer of a session restored from storage.
scheduleExpiry();
