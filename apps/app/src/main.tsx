import './index.css';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider, type RouteObject } from 'react-router';
import { toast } from 'sonner';

import { getMe } from '@pupitre/core/lib/api';
import { ApiError, configureClient } from '@pupitre/core/lib/api-client';
import { loginPath } from '@pupitre/core/lib/auth-paths';
import { endSession, getSession, getToken, setSessionEndHandler, updateUser } from '@pupitre/core/lib/session';
import { Toaster } from '@pupitre/ui/components/ui/sonner';
import { TooltipProvider } from '@pupitre/ui/components/ui/tooltip';
import { createRouter } from '@/router';

const isMock = import.meta.env.MODE === 'mock';

async function start() {
  const extraRoutes: RouteObject[] = [];

  if (isMock) {
    // Mock mode (`pnpm dev:mock`): MSW answers /api/* in the browser and a fake
    // Keycloak page stands in for the real one. Nothing of it ships otherwise.
    const [{ worker }, { MockKeycloakPage }] = await Promise.all([
      import('@pupitre/core/mocks/browser'),
      import('@pupitre/core/mocks/mock-keycloak-page'),
    ]);
    await worker.start({ onUnhandledFrame: 'bypass', quiet: true });
    extraRoutes.push({ path: '/mock-keycloak', element: <MockKeycloakPage /> });
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

  const router = createRouter(extraRoutes);

  configureClient({
    getToken,
    onUnauthorized: () => endSession('unauthorized'),
  });

  setSessionEndHandler((reason) => {
    queryClient.clear();
    if (reason === 'logout') {
      void router.navigate('/login');
      return;
    }
    toast.error('Session expirée, reconnectez-vous.');
    void router.navigate(loginPath(window.location));
  });

  // Refresh role, avatar and username of a restored session (spec 001). A 401
  // ends the session through the client's handler.
  if (getSession()) {
    getMe().then(updateUser).catch(() => {});
  }

  const container = document.getElementById('root');
  if (!container) throw new Error('#root element missing from index.html');

  createRoot(container).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <RouterProvider router={router} />
          <Toaster />
        </TooltipProvider>
      </QueryClientProvider>
    </StrictMode>,
  );
}

void start();
