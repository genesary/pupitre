package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/genesary/pupitre/apps/api/internal/users/models"
)

// SessionBookingRow is the read model returned by the repository.
type SessionBookingRow struct {
	CourseSlug string    `json:"courseSlug"`
	SessionID  string    `json:"sessionId"`
	BookedAt   time.Time `json:"bookedAt"`
}

// SessionParticipantRow is the read model for admin/manager participant lists.
type SessionParticipantRow struct {
	UserID   string    `json:"userId"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	BookedAt time.Time `json:"bookedAt"`
	Present  bool      `json:"present"`
}

// SessionBookingRepository manages seat reservations for in-person sessions.
type SessionBookingRepository interface {
	Book(ctx context.Context, userID, courseSlug, sessionID string) error
	// BookSeat books within capacity, 0 meaning no limit, and reports whether
	// the user holds a seat afterwards.
	BookSeat(ctx context.Context, userID, courseSlug, sessionID string, capacity int) (bool, error)
	Unbook(ctx context.Context, userID, courseSlug, sessionID string) error
	ListByUser(ctx context.Context, userID string) ([]SessionBookingRow, error)
	ListBySession(ctx context.Context, courseSlug, sessionID string) ([]SessionParticipantRow, error)
	CountBySession(ctx context.Context, courseSlug, sessionID string) (int64, error)
	ExistsByUser(ctx context.Context, userID, courseSlug, sessionID string) (bool, error)
	MarkPresent(ctx context.Context, userID, courseSlug, sessionID string, present bool) error
}

// gormSessionBookingRepository is the GORM-backed implementation.
type gormSessionBookingRepository struct {
	db *gorm.DB
}

// NewGormSessionBookingRepository builds a GORM-backed
// SessionBookingRepository.
//
//nolint:ireturn // returning the interface is intentional for DI.
func NewGormSessionBookingRepository(db *gorm.DB) SessionBookingRepository {
	return &gormSessionBookingRepository{db: db}
}

// Book creates a booking if one does not already exist.
func (r *gormSessionBookingRepository) Book(ctx context.Context, userID, courseSlug, sessionID string) error {
	booking := models.SessionBooking{
		UserID:     userID,
		CourseSlug: courseSlug,
		SessionID:  sessionID,
	}

	return r.db.WithContext(ctx).
		Where(models.SessionBooking{UserID: userID, CourseSlug: courseSlug, SessionID: sessionID}).
		FirstOrCreate(&booking).Error
}

// BookSeat implements SessionBookingRepository. The lock serializes the
// bookings of one session: two learners counting apart would both see the
// last seat free. A learner who already holds a seat keeps it, full or not.
func (r *gormSessionBookingRepository) BookSeat(ctx context.Context, userID, courseSlug, sessionID string, capacity int) (bool, error) {
	if capacity <= 0 {
		return true, r.Book(ctx, userID, courseSlug, sessionID)
	}

	seated := false

	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		lockErr := txDB.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", courseSlug+"/"+sessionID).Error
		if lockErr != nil {
			return fmt.Errorf("lock session seats: %w", lockErr)
		}

		txRepo := &gormSessionBookingRepository{db: txDB}

		held, existsErr := txRepo.ExistsByUser(ctx, userID, courseSlug, sessionID)
		if existsErr != nil {
			return fmt.Errorf("check own booking: %w", existsErr)
		}

		if held {
			seated = true

			return nil
		}

		seats, countErr := txRepo.CountBySession(ctx, courseSlug, sessionID)
		if countErr != nil {
			return fmt.Errorf("count session seats: %w", countErr)
		}

		if seats >= int64(capacity) {
			return nil
		}

		seated = true

		return txRepo.Book(ctx, userID, courseSlug, sessionID)
	})
	if err != nil {
		return false, fmt.Errorf("book seat: %w", err)
	}

	return seated, nil
}

// Unbook removes a booking.
func (r *gormSessionBookingRepository) Unbook(ctx context.Context, userID, courseSlug, sessionID string) error {
	return r.db.WithContext(ctx).
		Where("userid = ? AND courseslug = ? AND sessionid = ?", userID, courseSlug, sessionID).
		Delete(&models.SessionBooking{}).Error
}

// ListByUser returns all bookings for a given user.
func (r *gormSessionBookingRepository) ListByUser(ctx context.Context, userID string) ([]SessionBookingRow, error) {
	var rows []models.SessionBooking

	err := r.db.WithContext(ctx).
		Where("userid = ?", userID).
		Order("bookedat ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]SessionBookingRow, 0, len(rows))
	for _, b := range rows {
		out = append(out, SessionBookingRow{
			CourseSlug: b.CourseSlug,
			SessionID:  b.SessionID,
			BookedAt:   b.BookedAt,
		})
	}

	return out, nil
}

// ListBySession returns all participants for a given session, joining
// user info.
func (r *gormSessionBookingRepository) ListBySession(ctx context.Context, courseSlug, sessionID string) ([]SessionParticipantRow, error) {
	type scanRow struct {
		UserID   string    `gorm:"column:userid"`
		Username string    `gorm:"column:username"`
		Email    string    `gorm:"column:email"`
		BookedAt time.Time `gorm:"column:bookedat"`
		Present  bool      `gorm:"column:present"`
	}

	var rows []scanRow

	err := r.db.WithContext(ctx).
		Table("session_bookings sb").
		Select("sb.userid, u.username, u.email, sb.bookedat, sb.present").
		Joins("JOIN users u ON u.id = sb.userid::uuid").
		Where("sb.courseslug = ? AND sb.sessionid = ?", courseSlug, sessionID).
		Order("sb.bookedat ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]SessionParticipantRow, 0, len(rows))
	for _, scanned := range rows {
		out = append(out, SessionParticipantRow(scanned))
	}

	return out, nil
}

// CountBySession returns the number of bookings for a session.
func (r *gormSessionBookingRepository) CountBySession(ctx context.Context, courseSlug, sessionID string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.SessionBooking{}).
		Where("courseslug = ? AND sessionid = ?", courseSlug, sessionID).
		Count(&count).Error

	return count, err
}

// MarkPresent sets the presence flag for a booking.
func (r *gormSessionBookingRepository) MarkPresent(ctx context.Context, userID, courseSlug, sessionID string, present bool) error {
	return r.db.WithContext(ctx).
		Model(&models.SessionBooking{}).
		Where("userid = ? AND courseslug = ? AND sessionid = ?", userID, courseSlug, sessionID).
		Update("present", present).Error
}

// ExistsByUser reports whether a booking exists for the given user and session.
func (r *gormSessionBookingRepository) ExistsByUser(ctx context.Context, userID, courseSlug, sessionID string) (bool, error) {
	var booking models.SessionBooking

	err := r.db.WithContext(ctx).
		Where("userid = ? AND courseslug = ? AND sessionid = ?", userID, courseSlug, sessionID).
		First(&booking).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	return err == nil, err
}
