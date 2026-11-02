import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { Link, useParams } from 'react-router';

import { getPublicUser, userBadges } from '@pupitre/core/lib/api';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';
import { LoadingState } from '@pupitre/core/components/PageStates';

// /users/:id (docs/specs/020-learner-space.md), markup of pupitre's users/[id].astro.

export function PublicProfile() {
  const { id = '' } = useParams();
  const user = useQuery({ queryKey: ['users', id], queryFn: () => getPublicUser(id), retry: false });
  const badges = useQuery({ queryKey: ['users', id, 'badges'], queryFn: () => userBadges(id), retry: false });

  const username = user.data?.username || 'Utilisateur';
  const list = badges.data?.badges ?? [];

  useEffect(() => {
    if (!user.data) return;
    const previous = document.title;
    document.title = `${user.data.username} — pupitre`;
    return () => {
      document.title = previous;
    };
  }, [user.data]);

  return (
    <div className="up-outer">
      <div className="page-inner">
        <Link to="/leaderboard" className="up-back">
          ← Retour au classement
        </Link>

        {user.isPending || badges.isPending ? (
          <LoadingState />
        ) : user.isError ? (
          <p className="text-center text-ink-soft p-10">Profil introuvable.</p>
        ) : (
          <div>
            <div className="up-header">
              <div className="up-avatar">
                <UserAvatar
                  username={username}
                  avatarUrl={user.data.avatarUrl}
                  imageClassName="w-full h-full object-cover"
                  fallbackClassName=""
                  emojiClassName="text-[2.2rem]"
                />
              </div>
              <div>
                <div className="up-username">{username}</div>
                <div className="up-subtitle">
                  {list.length > 0
                    ? `${list.length} badge${list.length > 1 ? 's' : ''} obtenu${list.length > 1 ? 's' : ''}`
                    : "Aucun badge pour l'instant"}
                </div>
              </div>
            </div>

            {list.length === 0 ? (
              <div className="up-empty">Aucun badge pour l'instant.</div>
            ) : (
              <>
                <div className="up-section-title">Badges obtenus</div>
                <div className="up-grid">
                  {list.map((badge) => (
                    <Link key={badge.courseSlug} to={`/courses/${badge.courseSlug}`} className="up-card">
                      <div className="up-card-icon">{badge.icon || '🏅'}</div>
                      <div className="up-card-name">{badge.name || badge.courseSlug}</div>
                      <div className="up-card-course">{badge.courseTitle || badge.courseSlug}</div>
                      <div className="up-card-earners">
                        {badge.earnedBy} apprenant{badge.earnedBy > 1 ? 's' : ''}
                      </div>
                    </Link>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
