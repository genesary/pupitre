package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// AttemptKey identifies the quiz module a set of question attempts
// belongs to.
type AttemptKey struct {
	Username    string
	CourseSlug  string
	ModuleIndex int
}

// AttemptState is a learner's recorded retry state for one question.
type AttemptState struct {
	// Attempts is how many times the question has been answered wrongly.
	Attempts int
	// Remaining is how long until the question may be retried; zero when
	// no cooldown is pending.
	Remaining time.Duration
}

// RecordResult is the outcome of recording one wrong answer.
type RecordResult struct {
	Attempts int
	// Remaining is the cooldown the learner must now wait out.
	Remaining time.Duration
	// Locked reports that the question hit its attempt cap on a module
	// configured to lock rather than back off.
	Locked bool
}

// QuizAttemptRepository persists per-question quiz retry state.
//
// Every method takes the whole set of question IDs a request touches and
// resolves it in a fixed number of statements, so submitting a 30-question
// quiz costs three round trips rather than ninety.
type QuizAttemptRepository interface {
	// States returns the recorded state of each of questionIDs that has
	// any. Questions with no recorded attempt are absent from the result.
	States(ctx context.Context, key AttemptKey, questionIDs []string) (map[string]AttemptState, error)
	// RecordFailures increments the attempt counter of every question in
	// questionIDs, applies spec's backoff, and returns the resulting state
	// per question.
	RecordFailures(
		ctx context.Context, key AttemptKey, questionIDs []string,
		spec content.CooldownSpec, maxAttempts *int, lockOnMax bool,
	) (map[string]RecordResult, error)
	// Clear forgets the recorded attempts for questionIDs, which is what
	// answering them correctly does.
	Clear(ctx context.Context, key AttemptKey, questionIDs []string) error
}

// gormQuizAttemptRepository is the GORM-backed QuizAttemptRepository.
type gormQuizAttemptRepository struct {
	db *gorm.DB
}

// NewGormQuizAttemptRepository builds a QuizAttemptRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormQuizAttemptRepository(db *gorm.DB) QuizAttemptRepository {
	return &gormQuizAttemptRepository{db: db}
}

// States returns the recorded attempt state for each question that has one.
func (r *gormQuizAttemptRepository) States(ctx context.Context, key AttemptKey, questionIDs []string) (map[string]AttemptState, error) {
	states := make(map[string]AttemptState, len(questionIDs))
	if len(questionIDs) == 0 {
		return states, nil
	}

	var rows []models.QuizQuestionAttempt

	err := r.db.WithContext(ctx).
		Where("username = ? AND course_slug = ? AND module_index = ?", key.Username, key.CourseSlug, key.ModuleIndex).
		Where("question_id IN ?", questionIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load quiz attempts: %w", err)
	}

	now := time.Now()

	for _, row := range rows {
		states[row.QuestionID] = AttemptState{
			Attempts:  row.Attempts,
			Remaining: remainingUntil(row.CooldownUntil, now),
		}
	}

	return states, nil
}

// remainingUntil returns how long until deadline, clamped at zero and
// treating a nil deadline as "no cooldown".
func remainingUntil(deadline *time.Time, now time.Time) time.Duration {
	if deadline == nil {
		return 0
	}

	return max(deadline.Sub(now), 0)
}

// RecordFailures increments the attempt counter of every listed question
// and applies the module's backoff.
//
// The increment is a single multi-row upsert so concurrent submissions
// cannot lose a count to a read-modify-write race, and the resulting
// deadlines are written back in one more statement.
func (r *gormQuizAttemptRepository) RecordFailures(
	ctx context.Context, key AttemptKey, questionIDs []string,
	spec content.CooldownSpec, maxAttempts *int, lockOnMax bool,
) (map[string]RecordResult, error) {
	results := make(map[string]RecordResult, len(questionIDs))
	if len(questionIDs) == 0 {
		return results, nil
	}

	attempts, err := r.incrementAttempts(ctx, key, questionIDs)
	if err != nil {
		return nil, err
	}

	deadlines := make(map[string]time.Time, len(attempts))
	now := time.Now()

	for questionID, count := range attempts {
		if maxAttempts != nil && lockOnMax && count >= *maxAttempts {
			results[questionID] = RecordResult{Attempts: count, Locked: true}

			continue
		}

		remaining := content.ComputeCooldown(count, spec)
		results[questionID] = RecordResult{Attempts: count, Remaining: remaining}

		if remaining > 0 {
			deadlines[questionID] = now.Add(remaining)
		}
	}

	err = r.writeDeadlines(ctx, key, deadlines)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// dedupe returns values with duplicates and empty strings removed, keeping
// first-seen order. The multi-row upsert below cannot touch the same
// conflict target twice in one statement, so a quiz that declares the same
// question ID twice must not turn into a SQL error.
func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}

		if _, dup := seen[value]; dup {
			continue
		}

		seen[value] = struct{}{}

		out = append(out, value)
	}

	return out
}

// Clear forgets the recorded attempts for the listed questions.
func (r *gormQuizAttemptRepository) Clear(ctx context.Context, key AttemptKey, questionIDs []string) error {
	if len(questionIDs) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).
		Where("username = ? AND course_slug = ? AND module_index = ?", key.Username, key.CourseSlug, key.ModuleIndex).
		Where("question_id IN ?", questionIDs).
		Delete(&models.QuizQuestionAttempt{}).Error
	if err != nil {
		return fmt.Errorf("clear quiz attempts: %w", err)
	}

	return nil
}

// incrementAttempts bumps the attempt counter of every listed question in
// one statement and returns the new counts.
func (r *gormQuizAttemptRepository) incrementAttempts(ctx context.Context, key AttemptKey, questionIDs []string) (map[string]int, error) {
	questionIDs = dedupe(questionIDs)
	if len(questionIDs) == 0 {
		return map[string]int{}, nil
	}

	placeholders := make([]string, 0, len(questionIDs))
	args := make([]any, 0, len(questionIDs)*4) //nolint:mnd // four bound columns per row, see the VALUES tuple below

	for _, questionID := range questionIDs {
		placeholders = append(placeholders, "(?, ?, ?, ?, 1, NOW())")
		args = append(args, key.Username, key.CourseSlug, key.ModuleIndex, questionID)
	}

	query := `
		INSERT INTO quiz_question_attempts
			(username, course_slug, module_index, question_id, attempts, updated_at)
		VALUES ` + strings.Join(placeholders, ", ") + `
		ON CONFLICT (username, course_slug, module_index, question_id) DO UPDATE
			SET attempts = quiz_question_attempts.attempts + 1, updated_at = NOW()
		RETURNING question_id, attempts`

	var rows []struct {
		QuestionID string `gorm:"column:question_id"`
		Attempts   int    `gorm:"column:attempts"`
	}

	err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("record quiz attempts: %w", err)
	}

	attempts := make(map[string]int, len(rows))
	for _, row := range rows {
		attempts[row.QuestionID] = row.Attempts
	}

	return attempts, nil
}

// writeDeadlines stores the computed cooldown deadline of every question
// that has one, in a single statement.
func (r *gormQuizAttemptRepository) writeDeadlines(ctx context.Context, key AttemptKey, deadlines map[string]time.Time) error {
	if len(deadlines) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(deadlines))
	args := make([]any, 0, len(deadlines)*2) //nolint:mnd // two bound columns per row, see the VALUES tuple below

	for questionID, deadline := range deadlines {
		placeholders = append(placeholders, "(?, ?::timestamptz)")
		args = append(args, questionID, deadline)
	}

	query := `
		UPDATE quiz_question_attempts AS a
		SET cooldown_until = v.deadline
		FROM (VALUES ` + strings.Join(placeholders, ", ") + `) AS v(question_id, deadline)
		WHERE a.username = ? AND a.course_slug = ? AND a.module_index = ?
		  AND a.question_id = v.question_id`

	args = append(args, key.Username, key.CourseSlug, key.ModuleIndex)

	err := r.db.WithContext(ctx).Exec(query, args...).Error
	if err != nil {
		return fmt.Errorf("write quiz cooldowns: %w", err)
	}

	return nil
}
