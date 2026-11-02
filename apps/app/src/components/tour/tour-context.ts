import { createContext, useContext } from 'react';

// What the learner app's pages and layout reach of the guided tour
// (docs/specs/007-guided-tour.md). Provided by TourProvider.

export type TourContextValue = { startTour: () => void; running: boolean };

export const TourContext = createContext<TourContextValue>({ startTour: () => {}, running: false });

export function useTour() {
  return useContext(TourContext);
}
