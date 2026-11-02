package fake

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// patternGlobalScope mirrors repository's unexported scope constant.
const patternGlobalScope = "global"

// PatternRepository is an in-memory repository.PatternRepository for tests.
type PatternRepository struct {
	mu       sync.Mutex
	patterns []models.MarkdownPattern

	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewPatternRepository builds a fake PatternRepository seeded with patterns.
func NewPatternRepository(seed ...models.MarkdownPattern) *PatternRepository {
	return &PatternRepository{patterns: append([]models.MarkdownPattern{}, seed...)}
}

// ListGlobal returns every pattern with global scope.
func (f *PatternRepository) ListGlobal(_ context.Context) ([]models.MarkdownPattern, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]models.MarkdownPattern, 0)

	for _, p := range f.patterns {
		if p.Scope == patternGlobalScope {
			list = append(list, p)
		}
	}

	return list, nil
}

// ListForCourse returns global patterns plus patterns scoped to courseSlug.
func (f *PatternRepository) ListForCourse(_ context.Context, courseSlug string) ([]models.MarkdownPattern, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]models.MarkdownPattern, 0)

	for _, p := range f.patterns {
		if p.Scope == patternGlobalScope || p.Scope == courseSlug {
			list = append(list, p)
		}
	}

	return list, nil
}

// Get returns the pattern with the given id.
func (f *PatternRepository) Get(_ context.Context, patternID uuid.UUID) (*models.MarkdownPattern, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	i := f.findIndexByID(patternID)
	if i < 0 {
		return nil, repository.ErrPatternNotFound
	}

	cp := f.patterns[i]

	return &cp, nil
}

// Create adds a new pattern and returns it.
func (f *PatternRepository) Create(
	_ context.Context, name, label, description, parameter, html, css, jsCode, scope, createdBy string,
) (*models.MarkdownPattern, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	if f.findIndexByNameScope(name, scope) >= 0 {
		return nil, errors.New("duplicate pattern name in scope")
	}

	now := time.Now()

	createdByID, err := uuid.Parse(createdBy)

	var createdByPtr *uuid.UUID
	if err == nil {
		createdByPtr = &createdByID
	}

	pattern := models.MarkdownPattern{
		ID: uuid.New(), Name: name, Label: label, Description: description, Parameter: parameter,
		HTML: html, CSS: css, JS: jsCode, Scope: scope, CreatedBy: createdByPtr,
		CreatedAt: now, UpdatedAt: now,
	}

	f.patterns = append(f.patterns, pattern)

	return &pattern, nil
}

// UpdateByName replaces the pattern named oldName and returns the update.
func (f *PatternRepository) UpdateByName(
	_ context.Context, oldName, name, label, description, parameter, html, css, jsCode, scope string,
) (*models.MarkdownPattern, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	idx := f.findIndexByName(oldName)
	if idx < 0 {
		return nil, repository.ErrPatternNotFound
	}

	f.patterns[idx].Name = name
	f.patterns[idx].Label = label
	f.patterns[idx].Description = description
	f.patterns[idx].Parameter = parameter
	f.patterns[idx].HTML = html
	f.patterns[idx].CSS = css
	f.patterns[idx].JS = jsCode
	f.patterns[idx].Scope = scope
	f.patterns[idx].FromConfig = false
	f.patterns[idx].UpdatedAt = time.Now()

	cp := f.patterns[idx]

	return &cp, nil
}

// DeleteByName removes the pattern with the given name.
func (f *PatternRepository) DeleteByName(_ context.Context, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	i := f.findIndexByName(name)
	if i < 0 {
		return false, nil
	}

	f.patterns = append(f.patterns[:i], f.patterns[i+1:]...)

	return true, nil
}

// DeleteByIDAndScope removes the pattern with the given id and scope.
func (f *PatternRepository) DeleteByIDAndScope(_ context.Context, patternID uuid.UUID, scope string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	i := f.findIndexByID(patternID)
	if i < 0 || f.patterns[i].Scope != scope {
		return false, nil
	}

	f.patterns = append(f.patterns[:i], f.patterns[i+1:]...)

	return true, nil
}

// UpsertFromConfig inserts or updates a pattern sourced from config.
func (f *PatternRepository) UpsertFromConfig(
	_ context.Context, name, label, description, parameter, html, css, jsCode, scope string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	now := time.Now()

	idx := f.findIndexByNameScope(name, scope)
	if idx >= 0 {
		f.patterns[idx].Label = label
		f.patterns[idx].Description = description
		f.patterns[idx].Parameter = parameter
		f.patterns[idx].HTML = html
		f.patterns[idx].CSS = css
		f.patterns[idx].JS = jsCode
		f.patterns[idx].FromConfig = true
		f.patterns[idx].UpdatedAt = now

		return nil
	}

	f.patterns = append(f.patterns, models.MarkdownPattern{
		ID: uuid.New(), Name: name, Label: label, Description: description, Parameter: parameter,
		HTML: html, CSS: css, JS: jsCode, Scope: scope, FromConfig: true, CreatedAt: now, UpdatedAt: now,
	})

	return nil
}

// findIndexByID returns the index of the pattern with the given id, or -1.
func (f *PatternRepository) findIndexByID(id uuid.UUID) int {
	for i := range f.patterns {
		if f.patterns[i].ID == id {
			return i
		}
	}

	return -1
}

// findIndexByName returns the index of the pattern with the given name, or -1.
func (f *PatternRepository) findIndexByName(name string) int {
	for i := range f.patterns {
		if f.patterns[i].Name == name {
			return i
		}
	}

	return -1
}

// findIndexByNameScope returns the index of the pattern with the given name
// and scope, or -1.
func (f *PatternRepository) findIndexByNameScope(name, scope string) int {
	for i := range f.patterns {
		if f.patterns[i].Name == name && f.patterns[i].Scope == scope {
			return i
		}
	}

	return -1
}
