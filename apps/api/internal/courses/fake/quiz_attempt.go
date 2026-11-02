package fake

import (
	"context"
	"sync"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// QuizAttemptRepository is an in-memory fake of
// repository.QuizAttemptRepository.
type QuizAttemptRepository struct {
	mu      sync.Mutex
	entries map[attemptEntryKey]*attemptEntry
}

// attemptEntryKey identifies one learner's state on one quiz question.
type attemptEntryKey struct {
	repository.AttemptKey

	QuestionID string
}

// attemptEntry is the recorded state behind one attemptEntryKey.
type attemptEntry struct {
	attempts      int
	cooldownUntil time.Time
}

// NewQuizAttemptRepository builds an empty fake.
func NewQuizAttemptRepository() *QuizAttemptRepository {
	return &QuizAttemptRepository{entries: make(map[attemptEntryKey]*attemptEntry)}
}

// States returns the recorded state of each question that has one.
func (f *QuizAttemptRepository) States(
	_ context.Context, key repository.AttemptKey, questionIDs []string,
) (map[string]repository.AttemptState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	states := make(map[string]repository.AttemptState, len(questionIDs))

	for _, questionID := range questionIDs {
		entry, ok := f.entries[attemptEntryKey{AttemptKey: key, QuestionID: questionID}]
		if !ok {
			continue
		}

		states[questionID] = repository.AttemptState{
			Attempts:  entry.attempts,
			Remaining: max(time.Until(entry.cooldownUntil), 0),
		}
	}

	return states, nil
}

// RecordFailures increments the attempt counter of each listed question
// and applies spec's backoff.
func (f *QuizAttemptRepository) RecordFailures(
	_ context.Context, key repository.AttemptKey, questionIDs []string,
	spec content.CooldownSpec, maxAttempts *int, lockOnMax bool,
) (map[string]repository.RecordResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	results := make(map[string]repository.RecordResult, len(questionIDs))

	for _, questionID := range questionIDs {
		entryKey := attemptEntryKey{AttemptKey: key, QuestionID: questionID}

		entry, ok := f.entries[entryKey]
		if !ok {
			entry = &attemptEntry{}
			f.entries[entryKey] = entry
		}

		entry.attempts++

		if maxAttempts != nil && lockOnMax && entry.attempts >= *maxAttempts {
			results[questionID] = repository.RecordResult{Attempts: entry.attempts, Locked: true}

			continue
		}

		remaining := content.ComputeCooldown(entry.attempts, spec)
		entry.cooldownUntil = time.Now().Add(remaining)

		results[questionID] = repository.RecordResult{Attempts: entry.attempts, Remaining: remaining}
	}

	return results, nil
}

// Clear forgets the recorded attempts for the listed questions.
func (f *QuizAttemptRepository) Clear(_ context.Context, key repository.AttemptKey, questionIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, questionID := range questionIDs {
		delete(f.entries, attemptEntryKey{AttemptKey: key, QuestionID: questionID})
	}

	return nil
}
