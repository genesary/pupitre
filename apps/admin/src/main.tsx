import './index.css';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';

import { getMe } from '@pupitre/core/lib/api';
import { ApiError, configureClient } from '@pupitre/core/lib/api-client';
import { loginPath } from '@pupitre/core/lib/auth-paths';
import { endSession, getSession, getToken, setSessionEndHandler, updateUser } from '@pupitre/core/lib/session';
import { SiteContext } from '@pupitre/core/lib/site';
import { Toaster } from '@pupitre/ui/components/ui/sonner';
import { TooltipProvider } from '@pupitre/ui/components/ui/tooltip';
import { createRouter } from '@/router';

// The admin app. It shares the session of the learner app (same origin,
// sessionStorage); signing in and out happen on the learner app's pages.

const isMock = import.meta.env.MODE === 'mock';

async function start() {
  if (isMock) {
    // MSW, with the same handlers and mock state as the learner app. The
    // worker script is the learner app's /mockServiceWorker.js (same origin).
    const { worker } = await import('@pupitre/core/mocks/browser');
    await worker.start({ onUnhandledFrame: 'bypass', quiet: true });
  }

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // Client errors (4xx) will not fix themselves on retry.
        retry: (count, error) =>
          !(error instanceof ApiError && error.status >= 400 && error.status < 500) && count < 2,
      },
    },
  });

  configureClient({
    getToken,
    onUnauthorized: () => endSession('unauthorized'),
  });

  // The sign-in page belongs to the learner app: a page load.
  setSessionEndHandler((reason) => {
    queryClient.clear();
    window.location.assign(reason === 'logout' ? '/login' : loginPath(window.location));
  });

  // Refresh role, avatar and username of the shared session (spec 001).
  if (getSession()) {
    getMe().then(updateUser).catch(() => {});
  }

  const container = document.getElementById('root');
  if (!container) throw new Error('#root element missing from index.html');

  createRoot(container).render(
    <StrictMode>
      <SiteContext.Provider value="admin">
        <QueryClientProvider client={queryClient}>
          <TooltipProvider>
            <RouterProvider router={createRouter()} />
            <Toaster />
          </TooltipProvider>
        </QueryClientProvider>
      </SiteContext.Provider>
    </StrictMode>,
  );
}

void start();
