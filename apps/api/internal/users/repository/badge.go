package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// BadgeRow is a single row returned by badge listing queries.
type BadgeRow struct {
	CourseSlug string
	EarnedAt   time.Time
}

// BadgeLeaderboardRow is one entry in the badge leaderboard.
type BadgeLeaderboardRow struct {
	UserID    string
	Username  string
	AvatarURL *string
	Count     int64
	Slugs     []string
}

// BadgeRepository is the persistence boundary for the user_badges table.
type BadgeRepository interface {
	// Award grants courseSlug's badge to userID. Idempotent: re-awarding an
	// already-earned badge is a no-op.
	Award(ctx context.Context, userID, courseSlug string) error
	// MyBadges returns all badges earned by userID, most recently earned first.
	MyBadges(ctx context.Context, userID string) ([]BadgeRow, error)
	// UserBadges returns all badges earned by the given userID (public view).
	UserBadges(ctx context.Context, userID string) ([]BadgeRow, error)
	// EarnedCount returns how many distinct users have earned the badge for courseSlug.
	EarnedCount(ctx context.Context, courseSlug string) (int64, error)
	// EarnedCounts returns how many users have earned each of the given
	// badges, keyed by course slug, in one grouped query. Listing a
	// learner's badges needs every count at once, and asking for them one
	// at a time made that list cost a query per badge.
	EarnedCounts(ctx context.Context, courseSlugs []string) (map[string]int64, error)
	// Leaderboard returns users ranked by badge count, most badges first.
	Leaderboard(ctx context.Context, limit int) ([]BadgeLeaderboardRow, error)
}

// gormBadgeRepository is the GORM-backed BadgeRepository.
type gormBadgeRepository struct {
	db *gorm.DB
}

// NewGormBadgeRepository builds a BadgeRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormBadgeRepository(db *gorm.DB) BadgeRepository {
	return &gormBadgeRepository{db: db}
}

// Award inserts a user_badge row, doing nothing if one already exists.
func (r *gormBadgeRepository) Award(ctx context.Context, userID, courseSlug string) error {
	badge := models.UserBadge{UserID: userID, CourseSlug: courseSlug}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&badge).Error
	if err != nil {
		return fmt.Errorf("award badge: %w", err)
	}

	return nil
}

// MyBadges returns all badges earned by userID, most recently earned first.
func (r *gormBadgeRepository) MyBadges(ctx context.Context, userID string) ([]BadgeRow, error) {
	var rows []BadgeRow

	err := r.db.WithContext(ctx).Model(&models.UserBadge{}).
		Select("courseslug AS course_slug, earnedat AS earned_at").
		Where("userid = ?::uuid", userID).
		Order("earnedat DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list my badges: %w", err)
	}

	return rows, nil
}

// UserBadges returns all badges earned by userID (public view).
func (r *gormBadgeRepository) UserBadges(ctx context.Context, userID string) ([]BadgeRow, error) {
	return r.MyBadges(ctx, userID)
}

// EarnedCount returns how many distinct users have earned the badge
// for courseSlug.
func (r *gormBadgeRepository) EarnedCount(ctx context.Context, courseSlug string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.UserBadge{}).
		Where("courseslug = ?", courseSlug).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count badge earners: %w", err)
	}

	return count, nil
}

// EarnedCounts returns how many users have earned each of courseSlugs,
// keyed by course slug. Slugs nobody has earned are absent.
func (r *gormBadgeRepository) EarnedCounts(ctx context.Context, courseSlugs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(courseSlugs))
	if len(courseSlugs) == 0 {
		return counts, nil
	}

	var rows []struct {
		CourseSlug string `gorm:"column:courseslug"`
		Total      int64  `gorm:"column:total"`
	}

	err := r.db.WithContext(ctx).Model(&models.UserBadge{}).
		Select("courseslug, COUNT(*) AS total").
		Where("courseslug = ANY(?)", pq.StringArray(courseSlugs)).
		Group("courseslug").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("count badge earners by course: %w", err)
	}

	for _, row := range rows {
		counts[row.CourseSlug] = row.Total
	}

	return counts, nil
}

// Leaderboard returns up to limit users ranked by badge count descending,
// with the list of earned course slugs for each user.
func (r *gormBadgeRepository) Leaderboard(ctx context.Context, limit int) ([]BadgeLeaderboardRow, error) {
	type rawRow struct {
		UserID    string
		Username  string
		AvatarURL *string
		Count     int64
		Slugs     string // comma-separated
	}

	var raw []rawRow

	err := r.db.WithContext(ctx).
		Model(&models.UserBadge{}).
		Joins("JOIN users u ON u.id = user_badges.userid").
		Select("user_badges.userid AS user_id, u.username, u.avatarurl AS avatar_url, COUNT(*) AS count, STRING_AGG(user_badges.courseslug, ',' ORDER BY user_badges.earnedat DESC) AS slugs").
		Group("user_badges.userid, u.username, u.avatarurl").
		Order("count DESC, u.username ASC").
		Limit(limit).
		Scan(&raw).Error
	if err != nil {
		return nil, fmt.Errorf("leaderboard query: %w", err)
	}

	rows := make([]BadgeLeaderboardRow, 0, len(raw))

	for _, entry := range raw {
		var slugs []string
		if entry.Slugs != "" {
			slugs = splitCSV(entry.Slugs)
		}

		rows = append(rows, BadgeLeaderboardRow{
			UserID:    entry.UserID,
			Username:  entry.Username,
			AvatarURL: entry.AvatarURL,
			Count:     entry.Count,
			Slugs:     slugs,
		})
	}

	return rows, nil
}

// splitCSV splits a comma-separated string into a slice of non-empty parts.
func splitCSV(csv string) []string {
	if csv == "" {
		return nil
	}

	parts := make([]string, 0)
	start := 0

	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			parts = append(parts, csv[start:i])
			start = i + 1
		}
	}

	return parts
}
