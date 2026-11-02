import { act, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { UserPublic } from '@pupitre/core/lib/api-types';
import { useCurrentUser } from '@pupitre/core/lib/session';

import { TourProvider } from './TourProvider';

vi.mock('@pupitre/core/lib/session', () => ({ useCurrentUser: vi.fn() }));

const user = (id: string, username: string): UserPublic => ({
  id,
  username,
  email: `${username}@pupitre.local`,
  role: 'student',
  avatarUrl: null,
  bio: null,
  isActive: true,
  authProvider: 'oidc',
  createdAt: '2026-10-01T00:00:00Z',
});

/** Re-renders the provider on « Rafraîchir », as a session change would. */
function Harness() {
  const [renders, setRenders] = useState(0);
  return (
    <TourProvider>
      <button onClick={() => setRenders(renders + 1)}>Rafraîchir</button>
    </TourProvider>
  );
}

function renderProvider(path = '/courses') {
  const router = createMemoryRouter([{ path: '*', element: <Harness /> }], {
    initialEntries: [path],
  });
  return render(<RouterProvider router={router} />);
}

const startTour = async () => {
  fireEvent.click(screen.getByRole('button', { name: "C'est parti !" }));
  // startTour waits one frame for the side menu to close.
  await act(() => new Promise((resolve) => requestAnimationFrame(resolve)));
};

describe('tour provider (spec 007)', () => {
  beforeEach(() => vi.mocked(useCurrentUser).mockReturnValue(user('u1', 'alice')));

  it('offers the tour to a signed-in user, with the username as text', () => {
    vi.mocked(useCurrentUser).mockReturnValue(user('u1', '<img src=x onerror=alert(1)>'));
    const { container } = renderProvider();
    expect(screen.getByText('Bonjour <img src=x onerror=alert(1)> !')).toBeInTheDocument();
    expect(container.ownerDocument.querySelector('img[src="x"]')).toBeNull();
  });

  it('offers nothing to a visitor, nor over a lesson', () => {
    vi.mocked(useCurrentUser).mockReturnValue(null);
    renderProvider();
    expect(screen.queryByText(/Bonjour/)).toBeNull();
  });

  it('waits for another page than a lesson', () => {
    renderProvider('/courses/docker/lessons/1');
    expect(screen.queryByText(/Bonjour/)).toBeNull();
  });

  it('stops the tour when another user signs in', { timeout: 8000 }, async () => {
    renderProvider();
    await startTour();
    // No top bar here: the step waits 3 s for its target, then shows centred.
    expect(await screen.findByText('Le catalogue', undefined, { timeout: 4000 })).toBeInTheDocument();

    vi.mocked(useCurrentUser).mockReturnValue(user('u2', 'bob'));
    fireEvent.click(screen.getByRole('button', { name: 'Rafraîchir', hidden: true }));
    expect(screen.queryByText('Le catalogue')).toBeNull();
    // bob has never been offered the tour.
    expect(screen.getByText('Bonjour bob !')).toBeInTheDocument();
  });
});
