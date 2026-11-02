import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { leaderboard } from '@pupitre/core/lib/api';
import type { LeaderboardEntry } from '@pupitre/core/lib/api-types';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /leaderboard (docs/specs/020-learner-space.md), markup of pupitre's leaderboard.astro.
// Usernames and avatars come from other users: rendered as text or through
// <UserAvatar>, never as HTML (spec 003 S3).

const MEDALS: Record<number, string> = { 1: '🥇', 2: '🥈', 3: '🥉' };

const AVATAR_SIZES: Record<number, string> = {
  1: 'w-[88px] h-[88px] text-[1.8rem] border-[#fcd34d] bg-gradient-to-br from-[#fefce8] to-[#fef08a] text-[#a16207]',
  2: 'w-[72px] h-[72px] text-[1.4rem] border-[#c7d2fe] bg-gradient-to-br from-[#f0f4ff] to-[#e0e7ff] text-[#4361ee]',
  3: 'w-[68px] h-[68px] text-[1.3rem] border-[#fdba74] bg-gradient-to-br from-[#fff7ed] to-[#ffedd5] text-[#c2410c]',
};

const BLOCKS: Record<number, string> = {
  1: 'h-20 bg-gradient-to-b from-[#fde68a] to-[#fcd34d] text-[#a16207]',
  2: 'h-14 bg-gradient-to-b from-[#c7d2fe] to-[#a5b4fc] text-[#3730a3]',
  3: 'h-10 bg-gradient-to-b from-[#fdba74] to-[#fb923c] text-[#7c2d12]',
};

const ORDER: Record<number, string> = { 1: 'order-2', 2: 'order-1', 3: 'order-3' };

const badges = (count: number) => `${count} badge${count > 1 ? 's' : ''}`;

function Avatar({ entry }: { entry: LeaderboardEntry }) {
  return (
    <UserAvatar
      username={entry.username}
      avatarUrl={entry.avatarUrl}
      imageClassName="w-full h-full object-cover"
      fallbackClassName=""
      emojiClassName=""
    />
  );
}

function PodiumCard({ entry }: { entry: LeaderboardEntry }) {
  const first = entry.rank === 1;
  return (
    <Link to={`/users/${entry.userId}`} className={`lb-podium-card ${ORDER[entry.rank] ?? ''}`}>
      <div className={`lb-podium-avatar ${AVATAR_SIZES[entry.rank] ?? 'w-16 h-16 text-lg border-line bg-fill'}`}>
        <Avatar entry={entry} />
      </div>
      <div className={`lb-podium-medal ${first ? 'text-[2.2rem]' : 'text-[1.8rem]'}`}>
        {MEDALS[entry.rank] ?? `#${entry.rank}`}
      </div>
      <div className={`lb-podium-name ${first ? 'text-base' : 'text-[.9rem]'}`}>{entry.username}</div>
      <div className="lb-podium-count">{badges(entry.count)}</div>
      <div className={`lb-podium-block ${BLOCKS[entry.rank] ?? 'h-8 bg-fill text-ink-soft'}`}>{entry.rank}</div>
    </Link>
  );
}

function ListRow({ entry }: { entry: LeaderboardEntry }) {
  const icons = entry.icons ?? [];
  return (
    <Link to={`/users/${entry.userId}`} className="lb-row">
      <div className="lb-rank">#{entry.rank}</div>
      <div className="lb-avatar">
        <Avatar entry={entry} />
      </div>
      <div className="lb-info">
        <div className="lb-username">{entry.username}</div>
        <div className="lb-count">{badges(entry.count)}</div>
      </div>
      <div className="lb-icons sm:hidden">
        {icons.slice(0, 10).join(' ')}
        {icons.length > 10 && <span className="text-[.75rem] text-ink-soft">+{icons.length - 10}</span>}
      </div>
    </Link>
  );
}

export function Leaderboard() {
  const board = useQuery({ queryKey: ['leaderboard'], queryFn: leaderboard });
  const entries = board.data?.leaderboard ?? [];
  const top = entries.filter((e) => e.rank <= 3);
  const rest = entries.filter((e) => e.rank > 3);

  return (
    <div className="page-outer">
      <div className="page-inner">
        <div className="text-center mb-12">
          <h1 data-tour="page-title" className="page-title mb-1.5">Classement</h1>
          <p className="text-[.9rem] text-ink-soft">badges obtenus</p>
        </div>

        {board.isPending ? (
          <LoadingState />
        ) : board.isError ? (
          <LoadError error={board.error} onRetry={() => board.refetch()} />
        ) : entries.length === 0 ? (
          <div className="text-center py-20 px-6 text-ink-soft">
            <div className="text-[3rem] mb-3">🏅</div>
            <p>Aucun badge décerné pour l'instant.</p>
          </div>
        ) : (
          <>
            {top.length > 0 && (
              <div className="flex justify-center items-end gap-2 mb-10 sm:gap-4">
                {top.map((entry) => (
                  <PodiumCard key={entry.userId} entry={entry} />
                ))}
              </div>
            )}
            {rest.length > 0 && (
              <>
                <div className="divider-text mb-4">Autres participants</div>
                <div className="flex flex-col gap-2">
                  {rest.map((entry) => (
                    <ListRow key={entry.userId} entry={entry} />
                  ))}
                </div>
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
}
