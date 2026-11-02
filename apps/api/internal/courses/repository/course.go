package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// CourseFilter narrows a catalog listing. Every field is optional; the
// zero value lists the whole catalog.
type CourseFilter struct {
	// Category, Difficulty and Skill match exactly (case-insensitively).
	Category   string
	Difficulty string
	Skill      string
	// Search matches a substring of the title or description.
	Search string
	// PublicOnly restricts the result to courses in the public catalog.
	PublicOnly bool
	// Slugs, when non-empty, restricts the result to those exact courses.
	// It is what lets a caller resolve a set of course slugs in one query
	// instead of one request per slug.
	Slugs []string
}

// SkillModule identifies one module tagged with a given skill, together
// with the course it belongs to.
type SkillModule struct {
	Name        string
	Slug        string
	Index       int
	Type        string
	CourseSlug  string
	CourseTitle string
}

// CourseRepository reads and writes course definitions.
//
// Reads come in two depths on purpose: catalog listings never load module
// rows (they report ModuleCount instead), while the handlers that render a
// course load its modules in a single indexed query.
type CourseRepository interface {
	// List returns catalog entries matching filter, ordered by title.
	// Modules are not loaded; ModuleCount is populated instead.
	List(ctx context.Context, filter CourseFilter) ([]*content.Course, error)
	// Get returns a course with its modules, prerequisites and sessions,
	// or ErrNotFound.
	Get(ctx context.Context, slug string) (*content.Course, error)
	// Modules returns a course's modules in display order.
	Modules(ctx context.Context, slug string) ([]content.Module, error)
	// Upsert replaces a course definition wholesale, in one transaction.
	Upsert(ctx context.Context, course *content.Course) error
	// Create inserts a new course, returning ErrConflict if the slug is taken.
	Create(ctx context.Context, course *content.Course) error
	// Delete removes a course and every row hanging off it.
	Delete(ctx context.Context, slug string) error
	// SkillTotals counts how many courses teach each of the given skills.
	SkillTotals(ctx context.Context, skills []string) (map[string]int, error)
	// ModulesBySkills lists every module in the public catalog tagged with
	// any of skills, keyed by skill.
	ModulesBySkills(ctx context.Context, skills []string) (map[string][]SkillModule, error)
	// PutSession inserts or replaces one scheduled session of a course.
	PutSession(ctx context.Context, courseSlug string, session content.Session) error
	// DeleteSession removes one scheduled session, returning ErrNotFound
	// when the course or the session does not exist.
	DeleteSession(ctx context.Context, courseSlug, sessionID string) error
	// SessionExists reports whether a course has the given session.
	SessionExists(ctx context.Context, courseSlug, sessionID string) (bool, error)
}

// gormCourseRepository is the GORM-backed CourseRepository.
type gormCourseRepository struct {
	db *gorm.DB
}

// NewGormCourseRepository builds a CourseRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormCourseRepository(db *gorm.DB) CourseRepository {
	return &gormCourseRepository{db: db}
}

// courseListRow is a catalog row joined with its module count.
type courseListRow struct {
	models.Course

	ModuleCount int `gorm:"column:module_count"`
}

// List returns catalog entries matching filter, ordered by title. It runs
// three queries regardless of catalog size: the filtered course rows with
// their module counts, then the prerequisites and sessions of exactly
// those rows — never one query per course.
func (r *gormCourseRepository) List(ctx context.Context, filter CourseFilter) ([]*content.Course, error) {
	query := r.db.WithContext(ctx).
		Model(&models.Course{}).
		Select("courses.*, COUNT(course_modules.id) AS module_count").
		Joins("LEFT JOIN course_modules ON course_modules.course_slug = courses.slug").
		Group("courses.slug").
		Order("courses.title")

	query = applyCourseFilter(query, filter)

	var rows []courseListRow

	err := query.Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}

	courses := make([]*content.Course, 0, len(rows))
	slugs := make([]string, 0, len(rows))
	bySlug := make(map[string]*content.Course, len(rows))

	for i := range rows {
		course := courseFromModel(&rows[i].Course)
		course.ModuleCount = rows[i].ModuleCount

		courses = append(courses, course)
		slugs = append(slugs, course.Slug)
		bySlug[course.Slug] = course
	}

	err = r.attachPrerequisites(ctx, slugs, bySlug)
	if err != nil {
		return nil, err
	}

	err = r.attachSessions(ctx, slugs, bySlug)
	if err != nil {
		return nil, err
	}

	return courses, nil
}

// applyCourseFilter adds a WHERE clause for each active filter field.
func applyCourseFilter(query *gorm.DB, filter CourseFilter) *gorm.DB {
	if filter.PublicOnly {
		query = query.Where("courses.public = ?", true)
	}

	if len(filter.Slugs) > 0 {
		query = query.Where("courses.slug IN ?", filter.Slugs)
	}

	if filter.Category != "" {
		query = query.Where("LOWER(courses.category) = LOWER(?)", filter.Category)
	}

	if filter.Difficulty != "" {
		query = query.Where("LOWER(courses.difficulty) = LOWER(?)", filter.Difficulty)
	}

	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		query = query.Where("courses.title ILIKE ? OR courses.description ILIKE ?", pattern, pattern)
	}

	if filter.Skill != "" {
		// Compare against the lower-cased skill array so the filter stays
		// case-insensitive without giving up the array containment operator.
		query = query.Where("LOWER(?) = ANY (SELECT LOWER(s) FROM UNNEST(courses.skills) AS s)", filter.Skill)
	}

	return query
}

// attachChildren loads every row of type M belonging to the courses in
// slugs in a single query, ordered by orderBy, and hands each one to attach
// together with the course it belongs to.
//
// Batching by slug rather than looping per course is what keeps a catalogue
// listing at a fixed number of queries however large the catalogue is.
func attachChildren[M any](
	ctx context.Context, gdb *gorm.DB, slugs []string, orderBy, what string,
	courseSlugOf func(row M) string,
	bySlug map[string]*content.Course,
	attach func(course *content.Course, row M),
) error {
	if len(slugs) == 0 {
		return nil
	}

	var rows []M

	err := gdb.WithContext(ctx).
		Where("course_slug IN ?", slugs).
		Order(orderBy).
		Find(&rows).Error
	if err != nil {
		return fmt.Errorf("load %s: %w", what, err)
	}

	for _, row := range rows {
		course, ok := bySlug[courseSlugOf(row)]
		if !ok {
			continue
		}

		attach(course, row)
	}

	return nil
}

// Get returns a course with its modules, prerequisites and sessions.
func (r *gormCourseRepository) Get(ctx context.Context, slug string) (*content.Course, error) {
	var row models.Course

	err := r.db.WithContext(ctx).
		Preload("Modules", func(db *gorm.DB) *gorm.DB { return db.Order("course_modules.position") }).
		Preload("Prerequisites", func(db *gorm.DB) *gorm.DB { return db.Order("course_prerequisites.id") }).
		Preload("Sessions", func(db *gorm.DB) *gorm.DB { return db.Order("course_sessions.date, course_sessions.session_id") }).
		First(&row, "slug = ?", slug).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get course %s: %w", slug, err)
	}

	course := courseFromModel(&row)

	course.Modules = make([]content.Module, 0, len(row.Modules))

	for i := range row.Modules {
		module, convErr := moduleFromModel(&row.Modules[i])
		if convErr != nil {
			return nil, convErr
		}

		course.Modules = append(course.Modules, module)
	}

	course.ModuleCount = len(course.Modules)

	for _, prereq := range row.Prerequisites {
		course.Prerequisites = append(course.Prerequisites, prerequisiteFromModel(prereq))
	}

	for _, session := range row.Sessions {
		course.Sessions = append(course.Sessions, sessionFromModel(session))
	}

	return course, nil
}

// Modules returns a course's modules in display order.
func (r *gormCourseRepository) Modules(ctx context.Context, slug string) ([]content.Module, error) {
	var rows []models.CourseModule

	err := r.db.WithContext(ctx).
		Where("course_slug = ?", slug).
		Order("position").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list modules of %s: %w", slug, err)
	}

	modules := make([]content.Module, 0, len(rows))

	for i := range rows {
		module, convErr := moduleFromModel(&rows[i])
		if convErr != nil {
			return nil, convErr
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// Create inserts a new course definition, reporting ErrConflict when the
// slug is already taken.
func (r *gormCourseRepository) Create(ctx context.Context, course *content.Course) error {
	var existing int64

	err := r.db.WithContext(ctx).Model(&models.Course{}).Where("slug = ?", course.Slug).Count(&existing).Error
	if err != nil {
		return fmt.Errorf("check course %s: %w", course.Slug, err)
	}

	if existing > 0 {
		return ErrConflict
	}

	return r.Upsert(ctx, course)
}

// Upsert replaces a course definition wholesale. The course row and all of
// its children are written in one transaction, so a partially-applied
// course is never visible to a concurrent reader.
func (r *gormCourseRepository) Upsert(ctx context.Context, course *content.Course) error {
	row, children, err := courseToModel(course)
	if err != nil {
		return err
	}

	err = r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		upsertErr := transaction.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: colSlug}},
			DoUpdates: clause.AssignmentColumns([]string{
				colTitle, colDescription, "category", "difficulty", "public", "hidden",
				"scope", "xp_required", "badge_name", "badge_icon", "in_person",
				"skills", colUpdatedAt,
			}),
		}).Create(row).Error
		if upsertErr != nil {
			return fmt.Errorf("upsert course %s: %w", course.Slug, upsertErr)
		}

		return replaceCourseChildren(transaction, course.Slug, children)
	})
	if err != nil {
		return fmt.Errorf("write course %s: %w", course.Slug, err)
	}

	return nil
}

// courseChildren groups the rows that a course owns and that a write
// replaces wholesale.
type courseChildren struct {
	modules       []models.CourseModule
	prerequisites []models.CoursePrerequisite
	sessions      []models.CourseSession
}

// replaceCourseChildren deletes and re-inserts the module, prerequisite
// and session rows of a course. Sessions are upserted rather than deleted
// so that a course edit never silently drops bookings' schedule rows.
func replaceCourseChildren(transaction *gorm.DB, slug string, children courseChildren) error {
	err := transaction.Where("course_slug = ?", slug).Delete(&models.CourseModule{}).Error
	if err != nil {
		return fmt.Errorf("clear modules of %s: %w", slug, err)
	}

	err = transaction.Where("course_slug = ?", slug).Delete(&models.CoursePrerequisite{}).Error
	if err != nil {
		return fmt.Errorf("clear prerequisites of %s: %w", slug, err)
	}

	if len(children.modules) > 0 {
		err = transaction.Create(&children.modules).Error
		if err != nil {
			return fmt.Errorf("insert modules of %s: %w", slug, err)
		}
	}

	if len(children.prerequisites) > 0 {
		err = transaction.Create(&children.prerequisites).Error
		if err != nil {
			return fmt.Errorf("insert prerequisites of %s: %w", slug, err)
		}
	}

	if len(children.sessions) > 0 {
		err = transaction.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: colCourseSlug}, {Name: colSessionID}},
			DoUpdates: clause.AssignmentColumns([]string{colTitle, colDate, colLocation, colCapacity}),
		}).Create(&children.sessions).Error
		if err != nil {
			return fmt.Errorf("upsert sessions of %s: %w", slug, err)
		}
	}

	return nil
}

// Delete removes a course and every row hanging off it.
func (r *gormCourseRepository) Delete(ctx context.Context, slug string) error {
	result := r.db.WithContext(ctx).Delete(&models.Course{}, "slug = ?", slug)
	if result.Error != nil {
		return fmt.Errorf("delete course %s: %w", slug, result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// SkillTotals counts how many courses teach each of the given skills,
// resolving the whole set in a single grouped query.
func (r *gormCourseRepository) SkillTotals(ctx context.Context, skills []string) (map[string]int, error) {
	totals := make(map[string]int, len(skills))
	if len(skills) == 0 {
		return totals, nil
	}

	var rows []struct {
		Skill string `gorm:"column:skill"`
		Total int    `gorm:"column:total"`
	}

	err := r.db.WithContext(ctx).
		Table("courses").
		Select("s AS skill, COUNT(*) AS total").
		Joins("CROSS JOIN UNNEST(courses.skills) AS s").
		Where("s IN ?", skills).
		Group("s").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("count skill totals: %w", err)
	}

	for _, row := range rows {
		totals[row.Skill] = row.Total
	}

	return totals, nil
}

// ModulesBySkills lists every module in the public catalog tagged with any
// of skills, keyed by skill, in course-then-position order. Index is the
// module's position within its course.
//
// The whole set resolves in one query: the intersection of the requested
// skills with each module's own skill array is unnested, so a module
// tagged with several of them is attributed to each without the caller
// having to ask once per skill.
func (r *gormCourseRepository) ModulesBySkills(ctx context.Context, skills []string) (map[string][]SkillModule, error) {
	bySkill := make(map[string][]SkillModule, len(skills))
	if len(skills) == 0 {
		return bySkill, nil
	}

	var rows []struct {
		Skill       string `gorm:"column:skill"`
		Name        string `gorm:"column:name"`
		Slug        string `gorm:"column:slug"`
		Position    int    `gorm:"column:position"`
		Type        string `gorm:"column:type"`
		CourseSlug  string `gorm:"column:course_slug"`
		CourseTitle string `gorm:"column:course_title"`
	}

	err := r.db.WithContext(ctx).
		Table("course_modules").
		Select(`matched.skill, course_modules.name, course_modules.slug, course_modules.position,
			course_modules.type, course_modules.course_slug, courses.title AS course_title`).
		Joins("JOIN courses ON courses.slug = course_modules.course_slug").
		Joins("CROSS JOIN LATERAL UNNEST(course_modules.skills) AS matched(skill)").
		Where("courses.public = ?", true).
		Where("matched.skill = ANY(?)", pq.StringArray(skills)).
		Order("matched.skill, courses.title, course_modules.position").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list modules for %d skills: %w", len(skills), err)
	}

	for _, row := range rows {
		bySkill[row.Skill] = append(bySkill[row.Skill], SkillModule{
			Name:        row.Name,
			Slug:        row.Slug,
			Index:       row.Position,
			Type:        row.Type,
			CourseSlug:  row.CourseSlug,
			CourseTitle: row.CourseTitle,
		})
	}

	return bySkill, nil
}

// PutSession inserts or replaces one scheduled session of a course.
// Writing by (course_slug, session_id) makes a retried request idempotent:
// it overwrites the same row instead of appending a duplicate.
func (r *gormCourseRepository) PutSession(ctx context.Context, courseSlug string, session content.Session) error {
	exists, err := r.courseExists(ctx, courseSlug)
	if err != nil {
		return err
	}

	if !exists {
		return ErrNotFound
	}

	row := models.CourseSession{
		CourseSlug: courseSlug,
		SessionID:  session.ID,
		Title:      session.Title,
		Date:       session.Date,
		Location:   session.Location,
		Capacity:   session.Capacity,
	}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colCourseSlug}, {Name: colSessionID}},
		DoUpdates: clause.AssignmentColumns([]string{colTitle, colDate, colLocation, colCapacity}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("put session %s of %s: %w", session.ID, courseSlug, err)
	}

	return nil
}

// DeleteSession removes one scheduled session of a course.
func (r *gormCourseRepository) DeleteSession(ctx context.Context, courseSlug, sessionID string) error {
	result := r.db.WithContext(ctx).
		Delete(&models.CourseSession{}, "course_slug = ? AND session_id = ?", courseSlug, sessionID)
	if result.Error != nil {
		return fmt.Errorf("delete session %s of %s: %w", sessionID, courseSlug, result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// SessionExists reports whether a course has the given session.
func (r *gormCourseRepository) SessionExists(ctx context.Context, courseSlug, sessionID string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&models.CourseSession{}).
		Where("course_slug = ? AND session_id = ?", courseSlug, sessionID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check session %s of %s: %w", sessionID, courseSlug, err)
	}

	return count > 0, nil
}

// courseExists reports whether a course row exists for slug.
func (r *gormCourseRepository) courseExists(ctx context.Context, slug string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.Course{}).Where("slug = ?", slug).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check course %s: %w", slug, err)
	}

	return count > 0, nil
}

// attachPrerequisites loads the prerequisites of every course in slugs and
// attaches them to the matching entry of bySlug.
func (r *gormCourseRepository) attachPrerequisites(ctx context.Context, slugs []string, bySlug map[string]*content.Course) error {
	return attachChildren(ctx, r.db, slugs, "course_slug, id", "course prerequisites",
		func(row models.CoursePrerequisite) string { return row.CourseSlug },
		bySlug,
		func(course *content.Course, row models.CoursePrerequisite) {
			course.Prerequisites = append(course.Prerequisites, prerequisiteFromModel(row))
		})
}

// attachSessions loads the sessions of every course in slugs and attaches
// them to the matching entry of bySlug.
func (r *gormCourseRepository) attachSessions(ctx context.Context, slugs []string, bySlug map[string]*content.Course) error {
	return attachChildren(ctx, r.db, slugs, "course_slug, date, session_id", "course sessions",
		func(row models.CourseSession) string { return row.CourseSlug },
		bySlug,
		func(course *content.Course, row models.CourseSession) {
			course.Sessions = append(course.Sessions, sessionFromModel(row))
		})
}
