package fake

import (
	"context"
	"sync"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// BadgeRepository is an in-memory repository.BadgeRepository for tests.
type BadgeRepository struct {
	mu     sync.Mutex
	badges []badgeEntry
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
	// LeaderboardRows, when non-nil, is returned verbatim by Leaderboard so
	// tests can exercise the handler's ranking/enrichment loop.
	LeaderboardRows []repository.BadgeLeaderboardRow
}

// badgeEntry holds a single in-memory badge award.
type badgeEntry struct {
	userID     string
	courseSlug string
	earnedAt   time.Time
}

// NewBadgeRepository builds a fake BadgeRepository.
func NewBadgeRepository() *BadgeRepository {
	return &BadgeRepository{}
}

// Award grants a badge to userID for courseSlug. Idempotent.
func (f *BadgeRepository) Award(_ context.Context, userID, courseSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for _, b := range f.badges {
		if b.userID == userID && b.courseSlug == courseSlug {
			return nil
		}
	}

	f.badges = append(f.badges, badgeEntry{userID: userID, courseSlug: courseSlug, earnedAt: time.Now()})

	return nil
}

// MyBadges returns all badges earned by userID.
func (f *BadgeRepository) MyBadges(_ context.Context, userID string) ([]repository.BadgeRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var out []repository.BadgeRow

	for _, b := range f.badges {
		if b.userID == userID {
			out = append(out, repository.BadgeRow{CourseSlug: b.courseSlug, EarnedAt: b.earnedAt})
		}
	}

	return out, nil
}

// UserBadges returns all badges earned by userID (public view).
func (f *BadgeRepository) UserBadges(ctx context.Context, userID string) ([]repository.BadgeRow, error) {
	return f.MyBadges(ctx, userID)
}

// Leaderboard returns an empty leaderboard (stub).
func (f *BadgeRepository) Leaderboard(_ context.Context, _ int) ([]repository.BadgeLeaderboardRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	return f.LeaderboardRows, nil
}

// EarnedCount returns how many distinct users have earned the badge
// for courseSlug.
func (f *BadgeRepository) EarnedCount(_ context.Context, courseSlug string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	var count int64

	for _, b := range f.badges {
		if b.courseSlug == courseSlug {
			count++
		}
	}

	return count, nil
}

// EarnedCounts returns how many users have earned each of courseSlugs,
// keyed by course slug.
func (f *BadgeRepository) EarnedCounts(_ context.Context, courseSlugs []string) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wanted := make(map[string]bool, len(courseSlugs))
	for _, slug := range courseSlugs {
		wanted[slug] = true
	}

	counts := make(map[string]int64, len(courseSlugs))

	for _, b := range f.badges {
		if wanted[b.courseSlug] {
			counts[b.courseSlug]++
		}
	}

	return counts, nil
}
