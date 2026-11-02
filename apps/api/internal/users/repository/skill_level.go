package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// SkillLevelRow is a single row returned by skill level listing queries.
type SkillLevelRow struct {
	Skill            string `gorm:"column:skill"             json:"skill"`
	Level            string `gorm:"column:level"             json:"level"`
	CompletedCourses int    `gorm:"column:completed_courses" json:"completedCourses"`
	TotalCourses     int    `gorm:"column:total_courses"     json:"totalCourses"`
	LevelOrder       int    `gorm:"column:levelorder"        json:"-"`
}

// SkillLevelRepository is the persistence boundary for user_skill_levels.
type SkillLevelRepository interface {
	// Upsert increments the completed-course count for userID/skill, refreshes
	// totalCourses, and promotes the level (maître when all courses are done).
	Upsert(ctx context.Context, userID, skill, level string, totalCourses int) error
	// UpsertAll atomically upserts skill levels for all skills in a single
	// transaction; if any upsert fails the whole batch is rolled back.
	UpsertAll(ctx context.Context, userID string, skills map[string]struct{}, difficulty string, totals map[string]int) error
	// ListByUser returns all skill levels for userID, ordered by skill name.
	ListByUser(ctx context.Context, userID string) ([]SkillLevelRow, error)
}

// Numeric ordinals for the difficulty levels used in the levelorder column.
const (
	difficultyOrderBeginner = iota + 1
	difficultyOrderIntermediate
	difficultyOrderAdvanced
	difficultyOrderMaitre
)

// difficultyOrder maps a course difficulty string to a comparable integer.
// "maître" is excluded — it is a derived state, not a course difficulty.
func difficultyOrder(d string) int {
	switch d {
	case "beginner":
		return difficultyOrderBeginner
	case "intermediate":
		return difficultyOrderIntermediate
	case "advanced":
		return difficultyOrderAdvanced
	default:
		return 0
	}
}

// gormSkillLevelRepository is the GORM-backed SkillLevelRepository.
type gormSkillLevelRepository struct {
	db *gorm.DB
}

// NewGormSkillLevelRepository builds a SkillLevelRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormSkillLevelRepository(db *gorm.DB) SkillLevelRepository {
	return &gormSkillLevelRepository{db: db}
}

// upsertSkillLevelTx executes the ON CONFLICT upsert on gormDB (plain DB or a
// transaction handle). order must be pre-validated (non-zero).
func upsertSkillLevelTx(gormDB *gorm.DB, userID, skill, level string, order, totalCourses int) error {
	// First-completion path: single-course skill awards maître immediately.
	insertLevel := level
	insertOrder := order

	if totalCourses > 0 && totalCourses == 1 {
		insertLevel = "maître"
		insertOrder = difficultyOrderMaitre
	}

	err := gormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: colUserID}, {Name: "skill"}},
		DoUpdates: clause.Assignments(map[string]any{
			"completed_courses": gorm.Expr("user_skill_levels.completed_courses + 1"),
			"total_courses":     gorm.Expr("EXCLUDED.total_courses"),
			"level": gorm.Expr(`CASE
				WHEN EXCLUDED.total_courses > 0
				     AND user_skill_levels.completed_courses + 1 >= EXCLUDED.total_courses
				     THEN 'maître'
				WHEN EXCLUDED.levelorder > user_skill_levels.levelorder
				     THEN EXCLUDED.level
				ELSE user_skill_levels.level
			END`),
			"levelorder": gorm.Expr(`CASE
				WHEN EXCLUDED.total_courses > 0
				     AND user_skill_levels.completed_courses + 1 >= EXCLUDED.total_courses
				     THEN 4
				WHEN EXCLUDED.levelorder > user_skill_levels.levelorder
				     THEN EXCLUDED.levelorder
				ELSE user_skill_levels.levelorder
			END`),
			"updatedat": gorm.Expr("NOW()"),
		}),
	}).Create(&models.UserSkillLevel{
		UserID:           userID,
		Skill:            skill,
		Level:            insertLevel,
		LevelOrder:       insertOrder,
		CompletedCourses: 1,
		TotalCourses:     totalCourses,
	}).Error
	if err != nil {
		return fmt.Errorf("upsert skill level: %w", err)
	}

	return nil
}

// Upsert atomically records a course completion for userID/skill.
//
// Level progression rules (applied inside a single SQL statement):
//   - completed_courses is incremented on every call.
//   - If totalCourses > 0 and completed_courses reaches totalCourses, the
//     learner is promoted to "maître" (levelorder 4).
//   - Otherwise the highest difficulty seen so far is kept (max semantics).
//
// On the first completion (INSERT path) the maître check uses completed=1,
// so a skill with a single course awards maître immediately.
func (r *gormSkillLevelRepository) Upsert(ctx context.Context, userID, skill, level string, totalCourses int) error {
	order := difficultyOrder(level)
	if order == 0 {
		return nil
	}

	return upsertSkillLevelTx(r.db.WithContext(ctx), userID, skill, level, order, totalCourses)
}

// UpsertAll atomically upserts skill levels for all skills in skills inside a
// single database transaction. If any individual upsert fails the whole batch
// is rolled back, preventing partial updates.
func (r *gormSkillLevelRepository) UpsertAll(ctx context.Context, userID string, skills map[string]struct{}, difficulty string, totals map[string]int) error {
	order := difficultyOrder(difficulty)
	if order == 0 || len(skills) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for skill := range skills {
			upsertErr := upsertSkillLevelTx(tx, userID, skill, difficulty, order, totals[skill])
			if upsertErr != nil {
				return upsertErr
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert skill levels batch: %w", err)
	}

	return nil
}

// ListByUser returns all skill levels for userID ordered alphabetically by
// skill name, including progress counters.
func (r *gormSkillLevelRepository) ListByUser(ctx context.Context, userID string) ([]SkillLevelRow, error) {
	var rows []SkillLevelRow

	err := r.db.WithContext(ctx).
		Model(&models.UserSkillLevel{}).
		Select("skill, level, levelorder, completed_courses, total_courses").
		Where("userid = ?::uuid", userID).
		Order("skill ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list skill levels: %w", err)
	}

	return rows, nil
}
