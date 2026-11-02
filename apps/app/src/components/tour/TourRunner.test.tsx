import { act, fireEvent, render, screen } from '@testing-library/react';
import { useEffect, useState, type ReactNode } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { TourStep } from '@/lib/tour-steps';

import { TourRunner } from './TourRunner';

// jsdom lays nothing out: every target has an empty box, so a step either
// falls back to a centred bubble or, when optional, is skipped.

const STEPS: TourStep[] = [
  { id: 'one', targets: ['one'], title: 'Un', text: 'Premier' },
  { id: 'two', targets: ['two'], optional: true, title: 'Deux', text: 'Deuxième' },
  { id: 'three', title: 'Trois', text: 'Troisième' },
];

function renderTour(onEnd = vi.fn()) {
  const router = createMemoryRouter(
    [
      {
        path: '/',
        element: (
          <>
            <button>Avant</button>
            <span data-tour="one">cible</span>
            <span data-tour="two">cachée</span>
            <TourRunner steps={STEPS} onEnd={onEnd} />
          </>
        ),
      },
    ],
    { initialEntries: ['/'] },
  );
  render(<RouterProvider router={router} />);
  return onEnd;
}

function renderSteps(steps: TourStep[], page: ReactNode) {
  const router = createMemoryRouter(
    [{ path: '/', element: <>{page}<TourRunner steps={steps} onEnd={vi.fn()} /></> }],
    { initialEntries: ['/'] },
  );
  render(<RouterProvider router={router} />);
}

const key = (name: string) => act(() => void fireEvent.keyDown(document, { key: name }));

describe('tour runner (spec 007)', () => {
  it('goes forward and back with the keyboard, skipping a hidden optional step', async () => {
    const onEnd = renderTour();
    expect(await screen.findByText('Un')).toBeInTheDocument();
    expect(screen.getByText('1 / 3')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Suivant' })).toHaveFocus();

    key('ArrowRight');
    expect(await screen.findByText('Trois')).toBeInTheDocument();
    expect(screen.getByText('2 / 2')).toBeInTheDocument();

    key('ArrowLeft');
    expect(await screen.findByText('Un')).toBeInTheDocument();
    expect(onEnd).not.toHaveBeenCalled();
  });

  it('ends with Escape before the last step', async () => {
    const onEnd = renderTour();
    await screen.findByText('Un');
    key('Escape');
    expect(onEnd).toHaveBeenCalledWith(false);
  });

  it('completes on « Terminer »', async () => {
    const onEnd = renderTour();
    await screen.findByText('Un');
    fireEvent.click(screen.getByRole('button', { name: 'Suivant' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Terminer' }));
    expect(onEnd).toHaveBeenCalledWith(true);
    expect(screen.queryByText('Passer la visite')).toBeNull();
  });

  it('announces each step', async () => {
    renderTour();
    expect(await screen.findByText('Étape 1 sur 3 : Un')).toBeInTheDocument();
  });

  // Steps of the `loaded` shortcut (spec 007, "Navigation between pages").
  const LOADING: TourStep[] = [
    { id: 'card', targets: ['absent'], optional: true, loaded: 'page-loaded', title: 'Carte', text: '' },
    { id: 'end', title: 'Fin', text: '' },
  ];

  /** A page that shows its `loaded` marker after `delay` ms, or never. */
  function LoadingPage({ delay }: { delay?: number }) {
    const [loaded, setLoaded] = useState(false);
    useEffect(() => {
      if (delay === undefined) return;
      const timer = setTimeout(() => setLoaded(true), delay);
      return () => clearTimeout(timer);
    }, [delay]);
    return loaded ? <span data-tour="page-loaded" /> : null;
  }

  it('skips an optional step at once when its page says it loaded without the target', async () => {
    renderSteps(LOADING, <span data-tour="page-loaded" />);
    expect(await screen.findByText('Fin')).toBeInTheDocument();
    expect(screen.getByText('1 / 1')).toBeInTheDocument();
  });

  describe('while the page loads', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    const advance = (ms: number) => act(() => void vi.advanceTimersByTime(ms));

    it('waits, then skips the step after 3 s when the page never says it loaded', () => {
      renderSteps(LOADING, <LoadingPage />);
      advance(2900);
      expect(screen.getByText('…')).toBeInTheDocument();
      expect(screen.queryByText('Fin')).toBeNull();
      advance(200);
      expect(screen.getByText('Fin')).toBeInTheDocument();
    });

    it('skips the step as soon as the page says it loaded, without waiting 3 s', () => {
      renderSteps(LOADING, <LoadingPage delay={500} />);
      advance(400);
      expect(screen.getByText('…')).toBeInTheDocument();
      // The marker renders at 500 ms; the next poll (every 100 ms) skips the step.
      advance(150);
      advance(100);
      expect(screen.getByText('Fin')).toBeInTheDocument();
    });
  });
});
