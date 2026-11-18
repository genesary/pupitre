import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { listCourses, myCourses, mySkills } from '@pupitre/core/lib/api';
import type { Course, MyCourse } from '@pupitre/core/lib/api-types';
import { Catalogue } from '@/routes/Catalogue';

vi.mock('@pupitre/core/lib/api', () => ({ listCourses: vi.fn(), myCourses: vi.fn(), mySkills: vi.fn() }));
vi.mock('@pupitre/core/lib/session', () => ({ useSession: () => ({ token: 't', user: { username: 'alice' } }) }));
// The difficulty slider needs ResizeObserver, which jsdom lacks.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);

function course(slug: string, title: string): Course {
  return {
    slug,
    id: slug,
    title,
    description: '',
    category: 'devops',
    difficulty: 'beginner',
    isPublic: true,
    moduleCount: 4,
    labCount: 4,
    enrollmentCount: 1,
  };
}

function enrolled(slug: string, title: string, completedLabs: number, lastActivity: string | null = null): MyCourse {
  return {
    slug,
    id: slug,
    title,
    description: '',
    category: 'devops',
    difficulty: 'beginner',
    isPublic: true,
    labCount: 4,
    enrollmentCount: 1,
    completedLabs,
    totalScore: 0,
    lastActivity,
    completed: completedLabs === 4,
  };
}

function renderCatalogue(mine: MyCourse[]) {
  vi.mocked(listCourses).mockResolvedValue({
    courses: [
      course('done', 'Cours terminé'),
      course('todo', 'Cours à commencer'),
      course('old', 'Cours arrêté'),
      course('new', 'Cours commencé'),
    ],
  } as Awaited<ReturnType<typeof listCourses>>);
  vi.mocked(myCourses).mockResolvedValue({ courses: mine } as Awaited<ReturnType<typeof myCourses>>);
  vi.mocked(mySkills).mockResolvedValue({ skills: [] } as unknown as Awaited<ReturnType<typeof mySkills>>);
  const router = createMemoryRouter([{ path: '/courses', element: <Catalogue /> }], { initialEntries: ['/courses'] });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe('catalogue « Reprendre » hero', () => {
  it('skips a completed course for an unfinished one', async () => {
    renderCatalogue([enrolled('done', 'Cours terminé', 4), enrolled('todo', 'Cours à commencer', 0)]);
    expect(await screen.findByText('Reprendre')).toBeInTheDocument();
    expect(screen.getByText('0/4 modules · 0%')).toBeInTheDocument();
  });

  it('shows the unfinished course worked on last', async () => {
    renderCatalogue([
      enrolled('done', 'Cours terminé', 4, '2026-11-16 09:00:00+00'),
      enrolled('todo', 'Cours à commencer', 0),
      enrolled('old', 'Cours arrêté', 2, '2026-11-10 09:00:00+00'),
      enrolled('new', 'Cours commencé', 0, '2026-11-15 09:00:00+00'),
    ]);
    const hero = (await screen.findByText('Reprendre')).closest('.continue-hero')!;
    expect(hero).toHaveTextContent('Cours commencé');
  });

  it('is hidden when every enrolled course is completed', async () => {
    renderCatalogue([enrolled('done', 'Cours terminé', 4)]);
    expect(await screen.findAllByText('Cours terminé')).toHaveLength(1);
    expect(screen.queryByText('Reprendre')).toBeNull();
  });
});
