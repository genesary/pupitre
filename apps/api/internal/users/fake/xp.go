package fake

import (
	"context"
	"sync"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// xpEntry holds an in-memory XP event.
type xpEntry struct {
	userID     string
	source     string
	sourceSlug string
	amount     int
	earnedAt   time.Time
}

// XPRepository is an in-memory XPRepository for tests.
type XPRepository struct {
	mu     sync.Mutex
	events []xpEntry
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewXPRepository builds a fake XPRepository.
func NewXPRepository() *XPRepository {
	return &XPRepository{}
}

// Award records an XP event, idempotent per (userID, source, sourceSlug).
func (f *XPRepository) Award(_ context.Context, userID, source, sourceSlug string, amount int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for _, e := range f.events {
		if e.userID == userID && e.source == source && e.sourceSlug == sourceSlug {
			return nil
		}
	}

	f.events = append(f.events, xpEntry{
		userID:     userID,
		source:     source,
		sourceSlug: sourceSlug,
		amount:     amount,
		earnedAt:   time.Now(),
	})

	return nil
}

// Total returns the sum of XP for userID.
func (f *XPRepository) Total(_ context.Context, userID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	var total int

	for _, e := range f.events {
		if e.userID == userID {
			total += e.amount
		}
	}

	return total, nil
}

// History returns XP events for userID, most recent first.
func (f *XPRepository) History(_ context.Context, userID string) ([]repository.XPEventRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var out []repository.XPEventRow

	for i := len(f.events) - 1; i >= 0; i-- { //nolint:modernize // slices.Backward unavailable in this Go version
		entry := f.events[i]
		if entry.userID == userID {
			out = append(out, repository.XPEventRow{
				Source:     entry.source,
				SourceSlug: entry.sourceSlug,
				Amount:     entry.amount,
				EarnedAt:   entry.earnedAt,
			})
		}
	}

	return out, nil
}
