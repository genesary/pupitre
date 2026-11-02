import './tour.css';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router';

import { TOUR_MESSAGES } from '@/lib/messages/tour';
import { stepPath, type TourStep } from '@/lib/tour-steps';

import { TourBubble } from './TourBubble';
import { TourSpotlight } from './TourSpotlight';

// Runs a list of steps (docs/specs/007-guided-tour.md): goes to the page of
// each step, waits for its target, highlights it and shows the bird's bubble.
// Used by the guided tour and by the first lesson tip.

/** How long a step waits for its target after a navigation. */
const TARGET_WAIT_MS = 3000;
const POLL_MS = 100;

type Found = { element: Element } | 'hidden' | null;

function isVisible(element: Element): boolean {
  const rect = element.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0 && rect.right > 0 && rect.left < window.innerWidth;
}

function findTarget(targets: string[]): Found {
  let hidden = false;
  for (const target of targets) {
    for (const element of document.querySelectorAll(`[data-tour="${target}"]`)) {
      if (isVisible(element)) return { element };
      hidden = true;
    }
  }
  return hidden ? 'hidden' : null;
}

/** The slug of the course a `course-card` target links to. */
function courseOf(element: Element): string | null {
  const href = element.querySelector('a[href^="/courses/"]')?.getAttribute('href') ?? '';
  return /^\/courses\/([^/?#]+)$/.exec(href)?.[1] ?? null;
}

/**
 * The target's box, read again when the page scrolls or resizes, or when the
 * target or the page body changes size (content loading above it).
 */
function useTrackedRect(element: Element | null): DOMRect | null {
  const [tracked, setTracked] = useState<{ element: Element; rect: DOMRect } | null>(null);
  useEffect(() => {
    if (!element) return;
    let frame = 0;
    const measure = () => {
      frame = 0;
      setTracked({ element, rect: element.getBoundingClientRect() });
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(measure);
    };
    schedule();
    window.addEventListener('scroll', schedule, { capture: true, passive: true });
    window.addEventListener('resize', schedule);
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule);
    observer?.observe(element);
    observer?.observe(document.body);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('scroll', schedule, { capture: true });
      window.removeEventListener('resize', schedule);
      observer?.disconnect();
    };
  }, [element]);
  return tracked && tracked.element === element ? tracked.rect : null;
}

type Phase = { kind: 'waiting' } | { kind: 'shown'; element: Element | null };

type TourRunnerProps = {
  steps: TourStep[];
  /** Called when the tour stops; `completed` after its last step. */
  onEnd: (completed: boolean) => void;
  skipLabel?: string;
  /** `data-tour` of the element to focus at the end when the one focused at the start is gone. */
  returnFocusTo?: string;
};

export function TourRunner({ steps, onEnd, skipLabel = TOUR_MESSAGES.skip, returnFocusTo }: TourRunnerProps) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [index, setIndex] = useState(0);
  const [skipped, setSkipped] = useState<ReadonlySet<string>>(new Set());
  const [phase, setPhase] = useState<Phase>({ kind: 'waiting' });
  const courseSlug = useRef<string | null>(null);
  const direction = useRef<1 | -1>(1);
  // The step the tour last navigated for: a path the router writes differently
  // (encoded characters) must not make it navigate again and again.
  const navigatedFor = useRef<number | null>(null);
  const nextRef = useRef<HTMLButtonElement>(null);

  const step = steps[index]!;
  const last = index === steps.length - 1;
  const rect = useTrackedRect(phase.kind === 'shown' ? phase.element : null);

  // Focus goes back where it was when the tour ends: the « Visite guidée »
  // entry is gone with the side menu, so its button stands in for it. Read at
  // the first render, before the bubble takes focus.
  const [focusedBefore] = useState(() => document.activeElement);
  useEffect(() => {
    return () => {
      const fallback = returnFocusTo ? document.querySelector(`[data-tour="${returnFocusTo}"]`) : null;
      const kept =
        focusedBefore instanceof HTMLElement && focusedBefore !== document.body && focusedBefore.isConnected;
      const target = kept ? focusedBefore : fallback;
      if (target instanceof HTMLElement) target.focus();
    };
  }, [focusedBefore, returnFocusTo]);

  const move = useCallback(
    (to: 1 | -1, skippedNow: ReadonlySet<string>) => {
      direction.current = to;
      let target = index + to;
      while (target >= 0 && target < steps.length && skippedNow.has(steps[target]!.id)) target += to;
      if (target < 0) return;
      if (target >= steps.length) {
        onEnd(true);
        return;
      }
      navigatedFor.current = null;
      setPhase({ kind: 'waiting' });
      setIndex(target);
    },
    [index, onEnd, steps],
  );

  const skipStep = useCallback(
    (current: TourStep) => {
      const next = new Set(skipped).add(current.id);
      // Without the course of the card, the steps of the course page go too.
      if (current.id === 'course-card') for (const s of steps) if (s.needsCourse) next.add(s.id);
      setSkipped(next);
      move(direction.current, next);
    },
    [move, skipped, steps],
  );

  useEffect(() => {
    if (step.needsCourse && !courseSlug.current) {
      skipStep(step);
      return;
    }
    const path = stepPath(step, courseSlug.current);
    if (path && pathname !== path && navigatedFor.current !== index) {
      navigatedFor.current = index;
      navigate(path);
      return;
    }
    const targets = step.targets;
    const startedAt = performance.now();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const look = () => {
      if (!targets) {
        setPhase({ kind: 'shown', element: null });
        return;
      }
      const found = findTarget(targets);
      if (found && found !== 'hidden') {
        const { element } = found;
        if (step.id === 'course-card') courseSlug.current = courseOf(element);
        const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
        element.scrollIntoView?.({ block: 'center', behavior: reduced ? 'auto' : 'smooth' });
        setPhase({ kind: 'shown', element });
      } else if (
        found === 'hidden' ||
        // An optional step goes as soon as its page says it loaded without it.
        (step.optional && step.loaded && document.querySelector(`[data-tour="${step.loaded}"]`)) ||
        performance.now() - startedAt >= TARGET_WAIT_MS
      ) {
        if (step.optional) skipStep(step);
        else setPhase({ kind: 'shown', element: null });
      } else {
        timer = setTimeout(look, POLL_MS);
      }
    };
    look();
    return () => clearTimeout(timer);
    // skipStep changes with `skipped`, which this effect sets itself.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, index, pathname, navigate]);

  const next = useCallback(() => move(1, skipped), [move, skipped]);
  const previous = useCallback(() => move(-1, skipped), [move, skipped]);
  const stop = useCallback(() => onEnd(last), [last, onEnd]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        stop();
      } else if (phase.kind !== 'shown') {
        return;
      } else if (event.key === 'ArrowRight') {
        event.preventDefault();
        next();
      } else if (event.key === 'ArrowLeft') {
        event.preventDefault();
        previous();
      } else if (event.key === 'Enter' && !(event.target instanceof HTMLButtonElement)) {
        event.preventDefault();
        next();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [next, phase.kind, previous, stop]);

  const visible = steps.filter((s) => !skipped.has(s.id));
  const position = visible.indexOf(step) + 1;
  const counter = `${position} / ${visible.length}`;
  const shown = phase.kind === 'shown';
  const hasPrevious = steps.slice(0, index).some((s) => !skipped.has(s.id));

  return (
    <>
      <TourSpotlight rect={shown ? rect : null} />
      <TourBubble
        rect={shown ? rect : null}
        stepKey={`${step.id}:${phase.kind}`}
        title={shown ? step.title : TOUR_MESSAGES.waiting}
        counter={shown ? counter : undefined}
        focusRef={nextRef}
        actions={
          <>
            {!last && (
              <button type="button" className="tour-skip" onClick={stop}>
                {skipLabel}
              </button>
            )}
            <span className="tour-spacer" />
            {shown && hasPrevious && (
              <button type="button" className="btn btn-ghost btn-sm" onClick={previous}>
                {TOUR_MESSAGES.previous}
              </button>
            )}
            {shown && (
              <button type="button" ref={nextRef} className="btn btn-primary btn-sm" onClick={last ? stop : next}>
                {last ? TOUR_MESSAGES.finish : TOUR_MESSAGES.next}
              </button>
            )}
          </>
        }
      >
        {shown ? step.text : null}
      </TourBubble>
      <div className="sr-only" aria-live="polite">
        {shown ? TOUR_MESSAGES.announce(position, visible.length, step.title) : ''}
      </div>
    </>
  );
}
