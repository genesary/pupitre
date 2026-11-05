import { Outlet } from 'react-router';

import { TopBar } from './TopBar';

// layouts/App.astro: the top bar, then the page, which brings its own container.
// `onStartTour` adds « Visite guidée » to the side menu (learner app only).
export function Layout({ onStartTour }: { onStartTour?: () => void }) {
  return (
    <>
      <TopBar onStartTour={onStartTour} />
      <Outlet />
    </>
  );
}
