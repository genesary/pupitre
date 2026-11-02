import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { toast } from 'sonner';

import { getMe, myBadges, updateProfile } from '@pupitre/core/lib/api';
import type { UserPublic } from '@pupitre/core/lib/api-types';
import { updateUser } from '@pupitre/core/lib/session';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /profile (docs/specs/021-profile.md), markup of pupitre's profile.astro.
// Keycloak owns the username, the e-mail and the password: only the bio and an
// emoji avatar (from an earned badge) can be changed. No avatar URL can be
// entered (spec 003 S3, issue genesary/pupitre#187).

const PROVIDERS: Record<string, string> = {
  oidc: 'Keycloak',
  github: 'GitHub',
  gitlab: 'GitLab',
  google: 'Google',
  ldap: 'LDAP / Active Directory',
};

type Section = 'compte' | 'securite';

function AvatarPicker({ user, onSaved }: { user: UserPublic; onSaved: (user: UserPublic) => void }) {
  const [open, setOpen] = useState(false);
  const [hover, setHover] = useState(false);
  const area = useRef<HTMLDivElement>(null);
  const badges = useQuery({ queryKey: ['my', 'badges'], queryFn: myBadges });
  const icons = [...new Set((badges.data?.badges ?? []).map((b) => b.icon || '🏅'))];

  const save = useMutation({
    mutationFn: (icon: string) => updateProfile({ avatarUrl: icon }),
    onSuccess: (updated) => {
      onSaved(updated);
      setOpen(false);
      toast.success('Photo de profil mise à jour');
    },
    onError: () => toast.error('Erreur lors de la mise à jour'),
  });

  // Closes the menu on a click outside, like the Astro page.
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (!area.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [open]);

  return (
    <div ref={area}>
      <button
        type="button"
        className="profile-avatar-wrapper block border-0 bg-transparent p-0"
        title="Changer la photo de profil"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
      >
        <div className="w-20 h-20">
          <UserAvatar
            username={user.username}
            avatarUrl={user.avatarUrl}
            imageClassName="avatar-circle"
            fallbackClassName="avatar-initial"
            emojiClassName="text-[2rem]"
          />
        </div>
        <div className="profile-avatar-overlay" style={{ display: hover ? 'flex' : 'none' }}>
          ✏️
          <br />
          Modifier
        </div>
      </button>
      {open && (
        <div className="mb-3">
          <div className="card-sm p-2 text-left" style={{ boxShadow: '0 4px 16px rgba(0,0,0,.12)' }}>
            {icons.length > 0 ? (
              <>
                <div className="text-[.72rem] text-ink-soft mt-2 mb-1 px-2">Emoji de badge</div>
                <div className="flex flex-wrap gap-1.5 justify-center px-1 pb-1">
                  {icons.map((icon) => (
                    <button
                      key={icon}
                      type="button"
                      className="avatar-pick-btn"
                      title={icon}
                      disabled={save.isPending}
                      onClick={() => save.mutate(icon)}
                    >
                      {icon}
                    </button>
                  ))}
                </div>
              </>
            ) : (
              <p className="text-[.8rem] text-ink-soft px-2 py-1.5">
                {badges.isPending ? 'Chargement…' : 'Obtenez des badges pour débloquer des avatars'}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function AccountSection({ user, onSaved }: { user: UserPublic; onSaved: (user: UserPublic) => void }) {
  const [editing, setEditing] = useState(false);
  const [bio, setBio] = useState(user.bio ?? '');
  const save = useMutation({
    mutationFn: () => updateProfile({ bio }),
    onSuccess: (updated) => {
      onSaved(updated);
      setEditing(false);
      toast.success('Profil mis à jour');
    },
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    save.mutate();
  };

  return (
    <div className="card">
      <div className="profile-section-header">
        <h2 className="text-[1.1rem]">Mon compte</h2>
        {!editing && (
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => {
              setBio(user.bio ?? '');
              save.reset();
              setEditing(true);
            }}
          >
            Modifier
          </button>
        )}
      </div>
      {editing ? (
        <form className="stack gap-[14px]" onSubmit={submit}>
          <div className="form-group">
            <label className="label" htmlFor="edit-bio">
              Bio
            </label>
            <textarea
              className="input"
              id="edit-bio"
              rows={3}
              placeholder="Dites-nous quelque chose sur vous…"
              value={bio}
              onChange={(event) => setBio(event.target.value)}
            />
          </div>
          <div className="flex gap-2.5">
            <button type="submit" className="btn btn-primary" disabled={save.isPending}>
              {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
            </button>
            <button type="button" className="btn btn-secondary" onClick={() => setEditing(false)}>
              Annuler
            </button>
          </div>
          {save.isError && <p className="text-bad text-[.85rem]">{save.error.message || 'Erreur lors de la sauvegarde'}</p>}
        </form>
      ) : (
        <div className="stack gap-[14px]">
          <div className="profile-field-row">
            <span className="text-soft text-sm pt-0.5">Nom d'utilisateur</span>
            <span className="font-medium text-[.9rem]">{user.username}</span>
          </div>
          <div className="profile-field-row">
            <span className="text-soft text-sm pt-0.5">Email</span>
            <span className="text-[.9rem]">{user.email}</span>
          </div>
          <div className="profile-field-row">
            <span className="text-soft text-sm pt-0.5">Bio</span>
            <span className="text-[.9rem] text-ink-soft whitespace-pre-line">{user.bio || '—'}</span>
          </div>
        </div>
      )}
    </div>
  );
}

export function Profile() {
  const queryClient = useQueryClient();
  const location = useLocation();
  const navigate = useNavigate();
  const me = useQuery({ queryKey: ['auth', 'me'], queryFn: getMe });
  const section: Section = location.hash === '#securite' ? 'securite' : 'compte';

  // The session user feeds the top bar: keep it in step with the server.
  useEffect(() => {
    if (me.data) updateUser(me.data);
  }, [me.data]);

  const onSaved = (user: UserPublic) => {
    queryClient.setQueryData(['auth', 'me'], user);
  };

  if (me.isPending) {
    return (
      <div className="page-content">
        <LoadingState />
      </div>
    );
  }
  if (me.isError) {
    return (
      <div className="page-content">
        <LoadError error={me.error} onRetry={() => me.refetch()} />
      </div>
    );
  }

  const user = me.data;
  const provider = user.authProvider && user.authProvider !== 'local' ? user.authProvider : null;
  const providerLabel = provider ? (PROVIDERS[provider] ?? provider) : null;
  const link = (name: Section, label: string) => (
    <a
      className={`sidebar-link${section === name ? ' active' : ''}`}
      href={`#${name}`}
      onClick={(event) => {
        event.preventDefault();
        navigate({ hash: name }, { replace: true });
      }}
    >
      {label}
    </a>
  );

  return (
    <div className="page-content">
      <div className="profile-layout">
        <aside className="sidebar">
          <div className="card-sm mb-4 text-center p-5">
            <AvatarPicker user={user} onSaved={onSaved} />
            <div className="font-bold mb-0.5">{user.username}</div>
            <div className="text-[.8rem] text-ink-soft">{user.email}</div>
            <div className="mt-2">
              {user.role === 'admin' && <span className="badge badge-red">Admin</span>}{' '}
              {providerLabel && <span className="badge badge-blue">{providerLabel}</span>}
            </div>
          </div>
          <nav className="sidebar-nav">
            {link('compte', '👤 Compte')}
            {link('securite', '🔒 Sécurité')}
          </nav>
        </aside>

        <div className="stack-lg">
          {section === 'compte' ? (
            <AccountSection key={user.id} user={user} onSaved={onSaved} />
          ) : (
            <div className="card">
              <h2 className="text-[1.1rem] mb-5">Sécurité</h2>
              <p className="text-soft text-sm">
                Votre compte est géré par <strong>Keycloak</strong>. La gestion du mot de passe se fait dans votre compte
                Keycloak.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
