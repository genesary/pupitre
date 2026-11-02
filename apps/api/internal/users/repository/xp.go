package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// XP source identifiers used when recording XP events.
const (
	XPSourceLesson = "lesson"
	XPSourceModule = "module"
	XPSourceCourse = "course"
)

// XP amounts awarded per source type.
const (
	XPAmountLesson = 10
	XPAmountModule = 25
	XPAmountCourse = 200
)

// XPEventRow is a single row returned by XP history queries.
type XPEventRow struct {
	Source     string    `gorm:"column:source"      json:"source"`
	SourceSlug string    `gorm:"column:source_slug" json:"sourceSlug"`
	Amount     int       `gorm:"column:amount"      json:"amount"`
	EarnedAt   time.Time `gorm:"column:earned_at"   json:"earnedAt"`
}

// XPRepository is the persistence boundary for user_xp_events.
type XPRepository interface {
	// Award records an XP gain for userID. Idempotent per (userID, source, sourceSlug).
	Award(ctx context.Context, userID, source, sourceSlug string, amount int) error
	// Total returns the total XP earned by userID.
	Total(ctx context.Context, userID string) (int, error)
	// History returns XP events for userID, most recent first.
	History(ctx context.Context, userID string) ([]XPEventRow, error)
}

// gormXPRepository is the GORM-backed XPRepository.
type gormXPRepository struct {
	db *gorm.DB
}

// NewGormXPRepository builds an XPRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormXPRepository(db *gorm.DB) XPRepository {
	return &gormXPRepository{db: db}
}

// Award inserts an XP event, doing nothing if one already exists for the same
// (userid, source, source_slug) tuple, so replaying completions is safe.
func (r *gormXPRepository) Award(ctx context.Context, userID, source, sourceSlug string, amount int) error {
	event := models.UserXPEvent{
		UserID:     userID,
		Source:     source,
		SourceSlug: sourceSlug,
		Amount:     amount,
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: colUserID}, {Name: colSource}, {Name: "source_slug"}},
			DoNothing: true,
		}).
		Create(&event).Error
	if err != nil {
		return fmt.Errorf("award xp: %w", err)
	}

	return nil
}

// Total sums all XP events for userID.
func (r *gormXPRepository) Total(ctx context.Context, userID string) (int, error) {
	var total int

	err := r.db.WithContext(ctx).
		Model(&models.UserXPEvent{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("userid = ?::uuid", userID).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("sum xp: %w", err)
	}

	return total, nil
}

// History returns XP events for userID ordered by earned_at DESC.
func (r *gormXPRepository) History(ctx context.Context, userID string) ([]XPEventRow, error) {
	var rows []XPEventRow

	err := r.db.WithContext(ctx).
		Model(&models.UserXPEvent{}).
		Select("source, source_slug, amount, earned_at").
		Where("userid = ?::uuid", userID).
		Order("earned_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list xp events: %w", err)
	}

	return rows, nil
}
