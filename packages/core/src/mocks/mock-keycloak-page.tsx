import { useSearchParams } from 'react-router';

import { UserAvatar } from '@pupitre/ui/components/user-avatar';

import { MOCK_ACCOUNTS } from './data';

// Stand-in for the Keycloak sign-in page, mounted only by `pnpm dev:mock`. It
// returns to /auth/callback exactly like Keycloak would, so the real callback
// code (state check included) is exercised. Styled like the login card.

function callbackUrl(params: Record<string, string>) {
  return `/auth/callback?${new URLSearchParams(params).toString()}`;
}

export function MockKeycloakPage() {
  const [params] = useSearchParams();
  const state = params.get('state') ?? '';

  return (
    <div className="min-h-screen bg-fill flex items-start justify-center px-4 pt-10 pb-10 sm:items-center sm:pt-0">
      <div className="w-full max-w-[430px] bg-paper border-[1.5px] border-line rounded-[18px] px-5 pt-7 pb-6 flex flex-col gap-[18px] shadow-[0_20px_50px_-28px_rgba(30,60,120,.45)] sm:px-9 sm:pt-9 sm:pb-8">
        <div className="text-center">
          <h2>🧪 Keycloak simulé</h2>
          <p className="text-ink-soft text-[.875rem] mt-1">
            Mode mock : choisissez le compte avec lequel vous connecter.
          </p>
        </div>
        {!state && (
          <div className="bg-bad-soft border border-bad rounded-lg px-3.5 py-2.5 text-[.875rem] text-bad">
            Paramètre « state » manquant : repartez de la page de connexion.
          </div>
        )}
        <div className="flex flex-col gap-3">
          {MOCK_ACCOUNTS.map(({ user, description }) => (
            <button
              key={user.id}
              type="button"
              className="sso-btn !justify-start text-left"
              disabled={!state}
              onClick={() => window.location.assign(callbackUrl({ code: user.id, state }))}
            >
              <UserAvatar username={user.username} avatarUrl={user.avatarUrl} />
              <span className="flex-1 min-w-0">
                <span className="block font-semibold truncate">{user.username}</span>
                <span className="block text-[.78rem] text-ink-soft truncate">{description}</span>
              </span>
            </button>
          ))}
        </div>
        <div className="divider !my-0" />
        <button
          type="button"
          className="btn btn-ghost w-full"
          onClick={() => window.location.assign(callbackUrl({ error: 'access_denied' }))}
        >
          Annuler la connexion
        </button>
      </div>
    </div>
  );
}
