import { Outlet } from 'react-router';

import { TopBar } from './TopBar';

// layouts/App.astro: the top bar, then the page, which brings its own container.
// `onStartTour` adds « Visite guidée » to the side menu (learner app only,
// docs/specs/007-guided-tour.md).
export function Layout({ onStartTour }: { onStartTour?: () => void }) {
  return (
    <>
      <TopBar onStartTour={onStartTour} />
      <Outlet />
    </>
  );
}
