import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import { MOCK_ACCOUNTS } from '../mocks/data';
import { mockToken } from '../mocks/jwt';

import { RequireAuth, RequireRole } from './RequireAuth';
import { endSession, openSession } from '../lib/session';

function signIn(role: 'student' | 'manager' | 'admin') {
  const user = MOCK_ACCOUNTS.find((a) => a.user.role === role)!.user;
  openSession({ token: mockToken(user.id, role), user });
}

function renderAt(path: string) {
  const router = createMemoryRouter(
    [
      { path: '/login', element: <p>login</p> },
      { path: '/my-courses', element: <RequireAuth><p>mes cours</p></RequireAuth> },
      { path: '/admin/users', element: <RequireRole roles={['admin']}><p>utilisateurs</p></RequireRole> },
      {
        path: '/admin/exports',
        element: <RequireRole roles={['admin', 'manager']}><p>exports</p></RequireRole>,
      },
    ],
    { initialEntries: [path] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

describe('guards (spec 002)', () => {
  afterEach(() => endSession('logout'));

  it('sends a visitor to /login with the page to come back to', () => {
    const router = renderAt('/my-courses');
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toBe('?redirect=%2Fmy-courses');
  });

  it('shows "Accès refusé" to a learner on an admin page', () => {
    signIn('student');
    renderAt('/admin/users');
    expect(screen.getByText('Accès refusé')).toBeInTheDocument();
    expect(screen.queryByText('utilisateurs')).toBeNull();
  });

  it('lets a manager open the exports but not the users', () => {
    signIn('manager');
    renderAt('/admin/exports');
    expect(screen.getByText('exports')).toBeInTheDocument();
  });

  it('lets an admin in', () => {
    signIn('admin');
    renderAt('/admin/users');
    expect(screen.getByText('utilisateurs')).toBeInTheDocument();
  });
});
