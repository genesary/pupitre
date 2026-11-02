import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { createSession } from '@pupitre/core/lib/api';
import type { Course } from '@pupitre/core/lib/api-types';
import { SessionDialog } from '@/components/agenda/SessionDialog';

vi.mock('@pupitre/core/lib/api', () => ({ createSession: vi.fn(), updateSession: vi.fn() }));

const course = { slug: 'atelier', title: 'Atelier Kubernetes', inPerson: true } as Course;

describe('session dialog (spec 022)', () => {
  it('shows a missing location below its field and sends nothing', async () => {
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SessionDialog open onOpenChange={() => {}} courses={[course]} editing={null} onSaved={() => {}} />
      </QueryClientProvider>,
    );

    expect(screen.getByRole('heading', { name: 'Nouvelle session' })).toBeInTheDocument();
    await user.type(screen.getByLabelText('Titre'), 'Session de Paris');
    await user.type(screen.getByLabelText('Date et heure'), '2026-11-03T09:30');
    await user.click(screen.getByRole('button', { name: 'Créer' }));

    expect(screen.getByText('Le lieu est requis.')).toBeInTheDocument();
    expect(screen.getByLabelText('Lieu')).toHaveAttribute('aria-invalid', 'true');
    expect(createSession).not.toHaveBeenCalled();
  });

  it('sends the session once it is complete', async () => {
    const user = userEvent.setup();
    vi.mocked(createSession).mockResolvedValue({ id: 'sess-1' });
    const onSaved = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SessionDialog open onOpenChange={() => {}} courses={[course]} editing={null} onSaved={onSaved} />
      </QueryClientProvider>,
    );

    await user.type(screen.getByLabelText('Titre'), 'Session de Paris');
    await user.type(screen.getByLabelText('Date et heure'), '2026-11-03T09:30');
    await user.type(screen.getByLabelText('Lieu'), 'Paris');
    await user.click(screen.getByRole('button', { name: 'Créer' }));

    expect(createSession).toHaveBeenCalledWith('atelier', {
      title: 'Session de Paris',
      date: new Date('2026-11-03T09:30').toISOString(),
      location: 'Paris',
      capacity: 0,
    });
    expect(onSaved).toHaveBeenCalled();
  });
});
