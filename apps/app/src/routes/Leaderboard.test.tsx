import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { leaderboard } from '@pupitre/core/lib/api';
import { Leaderboard } from '@/routes/Leaderboard';

vi.mock('@pupitre/core/lib/api', () => ({ leaderboard: vi.fn() }));

const TRAP_NAME = '<img src=x onerror=alert("xss-username")>';

describe('leaderboard (spec 020)', () => {
  it('shows a malicious username and avatar as text, never as HTML', async () => {
    vi.mocked(leaderboard).mockResolvedValue({
      leaderboard: [
        { rank: 1, userId: 'u1', username: 'admin', avatarUrl: '🦉', count: 2, icons: ['🐧', '🐳'] },
        { rank: 2, userId: 'u2', username: TRAP_NAME, avatarUrl: 'https://x" onerror="alert(1)', count: 1, icons: ['🐧'] },
        { rank: 4, userId: 'u4', username: 'zoe', avatarUrl: null, count: 1, icons: Array(12).fill('🏅') },
      ],
    });
    const router = createMemoryRouter([{ path: '/leaderboard', element: <Leaderboard /> }], {
      initialEntries: ['/leaderboard'],
    });
    const { container } = render(
      <QueryClientProvider client={new QueryClient()}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    expect(await screen.findByText(TRAP_NAME)).toBeInTheDocument();
    expect(container.querySelector('[onerror]')).toBeNull();
    expect(container.querySelector('img[src="x"]')).toBeNull();
    // An avatar URL that is not a clean https URL falls back to the initial.
    expect(screen.getByText('<')).toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
    expect(screen.getByText('Autres participants')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /zoe/ })).toHaveAttribute('href', '/users/u4');
  });
});
