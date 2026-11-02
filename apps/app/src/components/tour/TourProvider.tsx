import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { useLocation } from 'react-router';
import { toast } from 'sonner';

import { useCurrentUser } from '@pupitre/core/lib/session';
import { TOUR_MESSAGES } from '@/lib/messages/tour';
import { recordTourStatus, readTourState, shouldOfferTour } from '@/lib/tour-storage';
import { isReadingPage, tourSteps } from '@/lib/tour-steps';

import { TourContext } from './tour-context';
import { TourBubble } from './TourBubble';
import { TourRunner } from './TourRunner';

// The guided tour of the learner app (docs/specs/007-guided-tour.md): the
// first-visit offer, `startTour()` for the side menu, and the running tour.

function TourOffer({ username, onStart, onLater }: { username: string; onStart: () => void; onLater: () => void }) {
  return (
    <TourBubble
      offer
      rect={null}
      stepKey="offer"
      title={TOUR_MESSAGES.welcomeTitle(username)}
      onClose={onLater}
      closeLabel={TOUR_MESSAGES.close}
      actions={
        <>
          <button type="button" className="btn btn-primary btn-sm" onClick={onStart}>
            {TOUR_MESSAGES.start}
          </button>
          <button type="button" className="btn btn-ghost btn-sm" onClick={onLater}>
            {TOUR_MESSAGES.later}
          </button>
        </>
      }
    >
      {TOUR_MESSAGES.welcomeText}
    </TourBubble>
  );
}

export function TourProvider({ children }: { children: ReactNode }) {
  const user = useCurrentUser();
  const { pathname } = useLocation();
  // The user the tour runs for: a sign-out or another sign-in stops it.
  const [runningFor, setRunningFor] = useState<string | null>(null);
  const [run, setRun] = useState(0);
  // Bumped after writing the stored state, so it is read again.
  const [stored, setStored] = useState(0);

  const userId = user?.id;
  const role = user?.role;
  const steps = useMemo(() => (role ? tourSteps(role) : []), [role]);
  const running = Boolean(userId) && runningFor === userId;

  const record = useCallback(
    (status: Parameters<typeof recordTourStatus>[1]) => {
      if (!userId) return;
      recordTourStatus(userId, status);
      setStored((n) => n + 1);
    },
    [userId],
  );

  const startTour = useCallback(() => {
    if (!userId) return;
    record('started');
    // Lets the side menu close and hand focus back before the tour takes it.
    requestAnimationFrame(() => {
      setRun((n) => n + 1);
      setRunningFor(userId);
    });
  }, [record, userId]);

  const end = useCallback(
    (completed: boolean) => {
      setRunningFor(null);
      record(completed ? 'completed' : 'dismissed');
      if (!completed) toast.info(TOUR_MESSAGES.skippedToast);
    },
    [record],
  );

  // `stored` is the trigger for a fresh read after a write.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const state = useMemo(() => (userId ? readTourState(userId) : null), [userId, stored]);
  const offer = user && !running && !isReadingPage(pathname) && shouldOfferTour(state) ? user : null;

  const value = useMemo(() => ({ startTour, running }), [startTour, running]);

  return (
    <TourContext.Provider value={value}>
      {children}
      {running && <TourRunner key={run} steps={steps} onEnd={end} returnFocusTo="nav-menu" />}
      {offer && (
        <TourOffer
          username={offer.username}
          onStart={startTour}
          onLater={() => {
            record('offered');
            toast.info(TOUR_MESSAGES.laterToast);
          }}
        />
      )}
    </TourContext.Provider>
  );
}
