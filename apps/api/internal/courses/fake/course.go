package fake

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// CourseRepository is an in-memory fake of repository.CourseRepository.
type CourseRepository struct {
	mu      sync.RWMutex
	courses map[string]*content.Course
	// Err, when set, is returned by every method (simulates a DB failure).
	Err error
}

// NewCourseRepository builds a fake seeded with the given courses.
func NewCourseRepository(seed ...*content.Course) *CourseRepository {
	repo := &CourseRepository{courses: make(map[string]*content.Course, len(seed))}
	for _, course := range seed {
		repo.put(course)
	}

	return repo
}

// List returns catalog entries matching filter, ordered by title.
func (f *CourseRepository) List(_ context.Context, filter repository.CourseFilter) ([]*content.Course, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return nil, f.Err
	}

	out := make([]*content.Course, 0, len(f.courses))

	for _, course := range f.courses {
		if matchesCourseFilter(course, filter) {
			listed := *course
			listed.Modules = nil
			out = append(out, &listed)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })

	return out, nil
}

// matchesCourseFilter reports whether course satisfies every active filter
// field, mirroring the WHERE clauses the SQL repository builds.
func matchesCourseFilter(course *content.Course, filter repository.CourseFilter) bool {
	return (!filter.PublicOnly || course.IsPublic) &&
		matchesOptional(filter.Category, course.Category) &&
		matchesOptional(filter.Difficulty, course.Difficulty) &&
		matchesSearch(filter.Search, course) &&
		matchesSkill(filter.Skill, course.Skills)
}

// matchesOptional reports whether an inactive (empty) filter passes, or an
// active one equals actual case-insensitively.
func matchesOptional(wanted, actual string) bool {
	return wanted == "" || strings.EqualFold(actual, wanted)
}

// matchesSearch reports whether search is empty or occurs in the course's
// title or description.
func matchesSearch(search string, course *content.Course) bool {
	if search == "" {
		return true
	}

	needle := strings.ToLower(search)

	return strings.Contains(strings.ToLower(course.Title), needle) ||
		strings.Contains(strings.ToLower(course.Description), needle)
}

// matchesSkill reports whether skill is empty or present in skills.
func matchesSkill(skill string, skills []string) bool {
	if skill == "" {
		return true
	}

	return slices.ContainsFunc(skills, func(candidate string) bool {
		return strings.EqualFold(candidate, skill)
	})
}

// Get returns a course with its modules, or repository.ErrNotFound.
func (f *CourseRepository) Get(_ context.Context, slug string) (*content.Course, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return nil, f.Err
	}

	course, ok := f.courses[slug]
	if !ok {
		return nil, repository.ErrNotFound
	}

	copied := *course

	return &copied, nil
}

// Modules returns a course's modules in display order.
func (f *CourseRepository) Modules(_ context.Context, slug string) ([]content.Module, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return nil, f.Err
	}

	course, ok := f.courses[slug]
	if !ok {
		return nil, repository.ErrNotFound
	}

	return slices.Clone(course.Modules), nil
}

// Upsert replaces a course definition wholesale.
func (f *CourseRepository) Upsert(_ context.Context, course *content.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.put(course)

	return nil
}

// Create inserts a new course, reporting repository.ErrConflict when the
// slug is taken.
func (f *CourseRepository) Create(_ context.Context, course *content.Course) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	if _, exists := f.courses[course.Slug]; exists {
		return repository.ErrConflict
	}

	f.put(course)

	return nil
}

// Delete removes a course.
func (f *CourseRepository) Delete(_ context.Context, slug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	if _, exists := f.courses[slug]; !exists {
		return repository.ErrNotFound
	}

	delete(f.courses, slug)

	return nil
}

// SkillTotals counts how many courses teach each of the given skills.
func (f *CourseRepository) SkillTotals(_ context.Context, skills []string) (map[string]int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wanted := make(map[string]struct{}, len(skills))
	for _, skill := range skills {
		wanted[skill] = struct{}{}
	}

	totals := make(map[string]int, len(skills))

	for _, course := range f.courses {
		for _, skill := range course.Skills {
			if _, ok := wanted[skill]; ok {
				totals[skill]++
			}
		}
	}

	return totals, nil
}

// ModulesBySkills lists every module of a public course tagged with any of
// skills, keyed by skill.
func (f *CourseRepository) ModulesBySkills(_ context.Context, skills []string) (map[string][]repository.SkillModule, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return nil, f.Err
	}

	bySkill := make(map[string][]repository.SkillModule, len(skills))

	for _, skill := range skills {
		var out []repository.SkillModule

		for _, course := range f.courses {
			if !course.IsPublic {
				continue
			}

			for idx, mod := range course.Modules {
				if !slices.Contains(mod.Skills, skill) {
					continue
				}

				out = append(out, repository.SkillModule{
					Name:        mod.Name,
					Slug:        mod.Slug(),
					Index:       idx,
					Type:        mod.Type,
					CourseSlug:  course.Slug,
					CourseTitle: course.Title,
				})
			}
		}

		sort.Slice(out, func(i, j int) bool {
			if out[i].CourseTitle != out[j].CourseTitle {
				return out[i].CourseTitle < out[j].CourseTitle
			}

			return out[i].Index < out[j].Index
		})

		if len(out) > 0 {
			bySkill[skill] = out
		}
	}

	return bySkill, nil
}

// PutSession inserts or replaces one scheduled session of a course.
func (f *CourseRepository) PutSession(_ context.Context, courseSlug string, session content.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	course, ok := f.courses[courseSlug]
	if !ok {
		return repository.ErrNotFound
	}

	sessions := slices.Clone(course.Sessions)

	replaced := false

	for i := range sessions {
		if sessions[i].ID == session.ID {
			sessions[i] = session
			replaced = true

			break
		}
	}

	if !replaced {
		sessions = append(sessions, session)
	}

	updated := *course
	updated.Sessions = sessions
	f.courses[courseSlug] = &updated

	return nil
}

// DeleteSession removes one scheduled session of a course.
func (f *CourseRepository) DeleteSession(_ context.Context, courseSlug, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	course, ok := f.courses[courseSlug]
	if !ok {
		return repository.ErrNotFound
	}

	remaining := make([]content.Session, 0, len(course.Sessions))

	for _, session := range course.Sessions {
		if session.ID != sessionID {
			remaining = append(remaining, session)
		}
	}

	if len(remaining) == len(course.Sessions) {
		return repository.ErrNotFound
	}

	updated := *course
	updated.Sessions = remaining
	f.courses[courseSlug] = &updated

	return nil
}

// SessionExists reports whether a course has the given session.
func (f *CourseRepository) SessionExists(_ context.Context, courseSlug, sessionID string) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.Err != nil {
		return false, f.Err
	}

	course, ok := f.courses[courseSlug]
	if !ok {
		return false, nil
	}

	return slices.ContainsFunc(course.Sessions, func(s content.Session) bool {
		return s.ID == sessionID
	}), nil
}

// put stores a deep-enough copy of course, deriving the fields the real
// repository computes on write.
func (f *CourseRepository) put(course *content.Course) {
	stored := *course
	stored.ModuleCount = len(course.Modules)

	if stored.Skills == nil {
		stored.Skills = content.AggregateSkills(course.Modules)
	}

	f.courses[stored.Slug] = &stored
}
