import type { AdminLeaderboardEntry } from '@pupitre/core/lib/api-types';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';

// Rows of the admin score leaderboard (docs/specs/040-admin-dashboard.md),
// markup of pupitre's admin/leaderboard and monitoring pages.

const MEDALS = ['🥇', '🥈', '🥉'];

export function ScoreLeaderboardRows({ entries, showEmail = false }: { entries: AdminLeaderboardEntry[]; showEmail?: boolean }) {
  return entries.map((entry, index) => (
    <div key={entry.id} className="lb-row-padded">
      <span className={`lb-rank-lg${index < 3 ? ' top' : ''}`}>{index < 3 ? MEDALS[index] : index + 1}</span>
      <div className="lb-avatar-lg lb-avatar-user">
        <UserAvatar
          username={entry.username}
          avatarUrl={entry.avatarUrl}
          imageClassName="w-full h-full object-cover rounded-full"
          fallbackClassName=""
          emojiClassName=""
        />
      </div>
      <div className="lb-info">
        <div className="lb-name">{entry.username}</div>
        {showEmail && <div className="lb-sub">{entry.email}</div>}
        <div className="lb-stats">
          {entry.enrolledCourses ?? 0} cours · {entry.passedModules ?? 0} modules validés
        </div>
      </div>
      <div className="lb-score-col">
        <div className="lb-score-main">{entry.totalScore ?? 0} pts</div>
      </div>
    </div>
  ));
}
