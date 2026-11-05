import { Layout } from '@pupitre/core/components/Layout';

import { useTour } from './tour-context';
import { TourProvider } from './TourProvider';

/** The shared layout, with the guided tour. */
function LayoutWithTour() {
  return <Layout onStartTour={useTour().startTour} />;
}

export function LearnerLayout() {
  return (
    <TourProvider>
      <LayoutWithTour />
    </TourProvider>
  );
}
