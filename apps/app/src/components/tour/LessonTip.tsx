import { useEffect, useState } from 'react';

import { useCurrentUser } from '@pupitre/core/lib/session';
import { TOUR_MESSAGES } from '@/lib/messages/tour';
import { recordLessonTip, readTourState, shouldShowLessonTip } from '@/lib/tour-storage';
import { LESSON_TIP_STEPS } from '@/lib/tour-steps';

import { useTour } from './tour-context';
import { TourRunner } from './TourRunner';

// The tip of the first lesson a user opens (docs/specs/007-guided-tour.md,
// "First lesson tip"). Mounted by the lesson page once its content shows.

export function LessonTip() {
  const user = useCurrentUser();
  const { running } = useTour();
  const [show, setShow] = useState(() => (user ? shouldShowLessonTip(readTourState(user.id)) : false));

  // Shown once, whether it is completed or closed.
  useEffect(() => {
    if (show && user) recordLessonTip(user.id);
  }, [show, user]);

  if (!show || running) return null;
  return <TourRunner steps={LESSON_TIP_STEPS} skipLabel={TOUR_MESSAGES.skipTip} onEnd={() => setShow(false)} />;
}
