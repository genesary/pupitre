import { useEffect, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import { oidcCallback } from '@pupitre/core/lib/api';
import { clearFlow, recallFlow } from '@/lib/oidc-flow';
import { landingPath } from '@pupitre/core/lib/auth-paths';
import { useSiteNavigate } from '@pupitre/core/lib/site';
import { openSession } from '@pupitre/core/lib/session';
import { safeRedirect } from '@pupitre/ui/lib/security';

// /auth/callback — Keycloak comes back here (docs/specs/001). The route must not
// change: it is the redirect URI registered in Keycloak.

function errorMessage(error: string): string {
  return error === 'access_denied'
    ? 'Accès refusé : vous avez annulé la connexion.'
    : `Erreur de connexion : ${error}`;
}

/** Checks done before any network call; null when the callback can proceed. */
function validate(params: URLSearchParams, expectedState: string | null): string | null {
  const providerError = params.get('error');
  if (providerError) return errorMessage(providerError);
  if (!params.get('code') || !params.get('state')) {
    return 'Paramètres de connexion manquants. Veuillez réessayer.';
  }
  // Refuse a callback this browser did not start (login CSRF, audit finding M3).
  if (!expectedState || expectedState !== params.get('state')) {
    return 'Session de connexion invalide, recommencez.';
  }
  return null;
}

export function AuthCallback() {
  const [params] = useSearchParams();
  // The destination may be in the admin app (an admin lands on /admin): a page load then.
  const navigate = useSiteNavigate();
  const [error, setError] = useState<string | null>(() => validate(params, recallFlow().state));
  // The authorization code is single-use, and React runs effects twice in
  // development: exchange it exactly once.
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    const flow = recallFlow();
    const code = params.get('code');
    const state = params.get('state');
    if (error || !code || !state) {
      clearFlow();
      return;
    }

    oidcCallback(code, state)
      .then((auth) => {
        clearFlow();
        openSession(auth);
        navigate(safeRedirect(flow.redirect, landingPath(auth.user)), { replace: true });
      })
      .catch((err: Error) => {
        clearFlow();
        setError(err.message || 'La connexion a échoué. Veuillez réessayer.');
      });
  }, [error, navigate, params]);

  // Markup of pupitre's pages/auth/callback.astro.
  return (
    <div className="min-h-screen bg-page flex items-center justify-center p-6">
      <div className="card w-full max-w-[380px] text-center px-10 py-12">
        {error ? (
          <div>
            <div className="text-[2.5rem] mb-4">✕</div>
            <h2 className="text-bad mb-2">Connexion échouée</h2>
            <p className="text-ink-soft text-[.875rem] mb-6">{error}</p>
            <Link to="/login" className="btn btn-primary">
              Retour à la connexion
            </Link>
          </div>
        ) : (
          <div>
            <div className="spinner w-12 h-12 border-[3px] mx-auto mb-5" />
            <h2 className="mb-2">Finalisation de la connexion…</h2>
            <p className="text-ink-soft text-[.875rem]">
              Veuillez patienter pendant la vérification de votre identité.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
