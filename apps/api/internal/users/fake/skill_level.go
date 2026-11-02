package fake

import (
	"context"
	"sync"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// SkillLevelRepository is an in-memory skill level repository for tests.
type SkillLevelRepository struct {
	mu     sync.Mutex
	levels map[string]repository.SkillLevelRow // key: userID+"/"+skill
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewSkillLevelRepository builds a fake SkillLevelRepository.
func NewSkillLevelRepository() *SkillLevelRepository {
	return &SkillLevelRepository{levels: make(map[string]repository.SkillLevelRow)}
}

// Numeric ordinals for the four difficulty levels (mirrors repository).
const (
	difficultyOrderBeginner = iota + 1
	difficultyOrderIntermediate
	difficultyOrderAdvanced
	difficultyOrderMaitre
)

// difficultyOrder maps a difficulty string to a comparable integer.
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

// Upsert increments the completed-course counter and promotes to maître when
// all courses for the skill are done.
func (f *SkillLevelRepository) Upsert(_ context.Context, userID, skill, level string, totalCourses int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	incomingOrder := difficultyOrder(level)
	if incomingOrder == 0 {
		return nil
	}

	key := userID + "/" + skill
	current, exists := f.levels[key]

	newCompleted := 1
	if exists {
		newCompleted = current.CompletedCourses + 1
	}

	isMaitre := totalCourses > 0 && newCompleted >= totalCourses

	var newLevel string

	var newOrder int

	switch {
	case isMaitre:
		newLevel = "maître"
		newOrder = difficultyOrderMaitre
	case exists && current.LevelOrder > incomingOrder:
		newLevel = current.Level
		newOrder = current.LevelOrder
	default:
		newLevel = level
		newOrder = incomingOrder
	}

	f.levels[key] = repository.SkillLevelRow{
		Skill:            skill,
		Level:            newLevel,
		CompletedCourses: newCompleted,
		TotalCourses:     totalCourses,
		LevelOrder:       newOrder,
	}

	return nil
}

// UpsertAll runs Upsert for each skill in skills; returns on first error.
func (f *SkillLevelRepository) UpsertAll(ctx context.Context, userID string, skills map[string]struct{}, difficulty string, totals map[string]int) error {
	for skill := range skills {
		err := f.Upsert(ctx, userID, skill, difficulty, totals[skill])
		if err != nil {
			return err
		}
	}

	return nil
}

// ListByUser returns all skill levels for userID.
func (f *SkillLevelRepository) ListByUser(_ context.Context, userID string) ([]repository.SkillLevelRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	prefix := userID + "/"

	var out []repository.SkillLevelRow

	for key, row := range f.levels {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, row)
		}
	}

	return out, nil
}
