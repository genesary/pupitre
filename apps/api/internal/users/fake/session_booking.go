package fake

import (
	"context"
	"sync"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// sessionBookingEntry holds an in-memory booking with its owner and
// presence flag.
type sessionBookingEntry struct {
	row     repository.SessionBookingRow
	userID  string
	present bool
}

// SessionBookingRepository is an in-memory session booking repository
// for tests.
type SessionBookingRepository struct {
	mu       sync.Mutex
	bookings map[string]sessionBookingEntry // key: userID+"/"+courseSlug+"/"+sessionID
	Err      error
}

// NewSessionBookingRepository builds a fake SessionBookingRepository.
func NewSessionBookingRepository() *SessionBookingRepository {
	return &SessionBookingRepository{bookings: make(map[string]sessionBookingEntry)}
}

// bookingKey builds the map key for a booking.
func bookingKey(userID, courseSlug, sessionID string) string {
	return userID + "/" + courseSlug + "/" + sessionID
}

// Book records a booking if none exists yet.
func (f *SessionBookingRepository) Book(_ context.Context, userID, courseSlug, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	key := bookingKey(userID, courseSlug, sessionID)
	if _, exists := f.bookings[key]; !exists {
		f.bookings[key] = sessionBookingEntry{
			row:    repository.SessionBookingRow{CourseSlug: courseSlug, SessionID: sessionID, BookedAt: time.Now()},
			userID: userID,
		}
	}

	return nil
}

// Unbook removes a booking.
func (f *SessionBookingRepository) Unbook(_ context.Context, userID, courseSlug, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	delete(f.bookings, bookingKey(userID, courseSlug, sessionID))

	return nil
}

// ListByUser returns all bookings owned by the given user.
func (f *SessionBookingRepository) ListByUser(_ context.Context, userID string) ([]repository.SessionBookingRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	prefix := userID + "/"

	var out []repository.SessionBookingRow

	for key, entry := range f.bookings {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, entry.row)
		}
	}

	return out, nil
}

// ListBySession returns all participants for a given session.
func (f *SessionBookingRepository) ListBySession(_ context.Context, courseSlug, sessionID string) ([]repository.SessionParticipantRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	suffix := "/" + courseSlug + "/" + sessionID

	var out []repository.SessionParticipantRow

	for key, entry := range f.bookings {
		if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
			out = append(out, repository.SessionParticipantRow{
				UserID:   entry.userID,
				Username: entry.userID,
				BookedAt: entry.row.BookedAt,
				Present:  entry.present,
			})
		}
	}

	return out, nil
}

// CountBySession returns the number of bookings for a session.
func (f *SessionBookingRepository) CountBySession(_ context.Context, courseSlug, sessionID string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	suffix := "/" + courseSlug + "/" + sessionID

	var count int64

	for key := range f.bookings {
		if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
			count++
		}
	}

	return count, nil
}

// MarkPresent sets the presence flag for a booking.
func (f *SessionBookingRepository) MarkPresent(_ context.Context, userID, courseSlug, sessionID string, present bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	key := bookingKey(userID, courseSlug, sessionID)
	if entry, exists := f.bookings[key]; exists {
		entry.present = present
		f.bookings[key] = entry
	}

	return nil
}

// ExistsByUser reports whether a booking exists for the given user and session.
func (f *SessionBookingRepository) ExistsByUser(_ context.Context, userID, courseSlug, sessionID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	_, exists := f.bookings[bookingKey(userID, courseSlug, sessionID)]

	return exists, nil
}
