import { useSyncExternalStore } from 'react';

import { writeItem } from './storage';

// Light / dark mode of docs/specs/005-ui-theme-navigation.md. The initial class
// is set before React starts by public/theme-init.js, which reads the same key.

const STORAGE_KEY = 'theme';

type Theme = 'light' | 'dark';

const listeners = new Set<() => void>();

function current(): Theme {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light';
}

export function setTheme(theme: Theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark');
  writeItem('local', STORAGE_KEY, theme);
  listeners.forEach((listener) => listener());
}

export function toggleTheme() {
  setTheme(current() === 'dark' ? 'light' : 'dark');
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useTheme(): Theme {
  return useSyncExternalStore(subscribe, current, () => 'light');
}
