import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { Toaster } from 'sonner';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { getCourse, markPresence, sessionParticipants } from '@pupitre/core/lib/api';
import { endSession, openSession } from '@pupitre/core/lib/session';
import { MOCK_ACCOUNTS } from '@pupitre/core/mocks/data';
import { mockToken } from '@pupitre/core/mocks/jwt';
import { Attendance } from '@/routes/Attendance';

vi.mock('@pupitre/core/lib/api', () => ({ getCourse: vi.fn(), sessionParticipants: vi.fn(), markPresence: vi.fn() }));

function renderAs(role: 'admin' | 'manager') {
  const user = MOCK_ACCOUNTS.find((a) => a.user.role === role)!.user;
  openSession({ token: mockToken(user.id, role), user });
  vi.mocked(getCourse).mockResolvedValue({
    slug: 'atelier',
    title: 'Atelier Kubernetes',
    sessions: [{ id: 's1', title: 'Session de Paris', date: '2026-11-03T09:00:00Z' }],
  } as Awaited<ReturnType<typeof getCourse>>);
  vi.mocked(sessionParticipants).mockResolvedValue({
    participants: [{ userId: 'u1', username: 'alice', email: 'alice@pupitre.test', bookedAt: '2026-10-01T09:00:00Z', present: false }],
  });
  const router = createMemoryRouter([{ path: '/a/:slug/:sessionId', element: <Attendance /> }], {
    initialEntries: ['/a/atelier/s1'],
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
}

describe('attendance (spec 022)', () => {
  afterEach(() => {
    endSession('logout');
    vi.clearAllMocks();
  });

  it('reads the manager route for a manager, and counts the present participants', async () => {
    vi.mocked(markPresence).mockResolvedValue({ ok: true });
    renderAs('manager');
    const box = await screen.findByRole('checkbox', { name: 'alice présent' });
    expect(sessionParticipants).toHaveBeenCalledWith('manager', 'atelier', 's1');
    expect(screen.getByText('Session — Session de Paris')).toBeInTheDocument();

    await userEvent.click(box);
    expect(markPresence).toHaveBeenCalledWith('manager', 'atelier', 's1', 'u1', true);
    expect(box).toBeChecked();
    expect(screen.getByText('Présents').firstChild).toHaveTextContent('1');
  });

  it('puts the box back and shows a toast when the call fails', async () => {
    vi.mocked(markPresence).mockRejectedValue(new Error('offline'));
    renderAs('admin');
    const box = await screen.findByRole('checkbox', { name: 'alice présent' });
    expect(sessionParticipants).toHaveBeenCalledWith('admin', 'atelier', 's1');

    await userEvent.click(box);
    await waitFor(() => expect(box).not.toBeChecked());
    expect(await screen.findByText('Erreur lors de la mise à jour de la présence.')).toBeInTheDocument();
  });
});
