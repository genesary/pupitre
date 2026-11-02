package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
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
