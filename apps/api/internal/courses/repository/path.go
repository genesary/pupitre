package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// pathKindCourse is the default path kind: an ordered list of courses.
const pathKindCourse = "course"

// PathRepository reads and writes learning path definitions.
type PathRepository interface {
	// List returns paths ordered by title, with their members loaded.
	// A limit of zero or less means unlimited.
	List(ctx context.Context, limit, offset int) ([]*content.Path, error)
	// Get returns one path with its members, or ErrNotFound.
	Get(ctx context.Context, slug string) (*content.Path, error)
	// ListBySlugs returns the named paths with their members loaded, in
	// title order, resolving the whole set in a fixed number of queries.
	// Slugs that match no path are simply absent from the result.
	ListBySlugs(ctx context.Context, slugs []string) ([]*content.Path, error)
	// SlugsContainingCourse returns the slugs of every path that includes
	// courseSlug. This replaces the in-memory reverse index that used to
	// answer the same question.
	SlugsContainingCourse(ctx context.Context, courseSlug string) ([]string, error)
	// SkillsOfCourses returns the deduplicated union of the skills taught
	// by the given courses, in a single query.
	SkillsOfCourses(ctx context.Context, courseSlugs []string) ([]string, error)
	// SkillsByCourse returns the skills taught by each of the given
	// courses, keyed by course slug, in a single query. A batch listing
	// uses it to attribute skills back to the path each course belongs to.
	SkillsByCourse(ctx context.Context, courseSlugs []string) (map[string][]string, error)
	// Upsert replaces a path definition wholesale, in one transaction.
	Upsert(ctx context.Context, path *content.Path) error
	// Create inserts a new path, returning ErrConflict if the slug is taken.
	Create(ctx context.Context, path *content.Path) error
	// Delete removes a path and its members.
	Delete(ctx context.Context, slug string) error
}

// gormPathRepository is the GORM-backed PathRepository.
type gormPathRepository struct {
	db *gorm.DB
}

// NewGormPathRepository builds a PathRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormPathRepository(db *gorm.DB) PathRepository {
	return &gormPathRepository{db: db}
}

// List returns paths ordered by title, paginated by limit/offset. Members
// are loaded for the returned page only, in two further queries — never
// one per path.
func (r *gormPathRepository) List(ctx context.Context, limit, offset int) ([]*content.Path, error) {
	query := r.db.WithContext(ctx).Order("title")

	if offset > 0 {
		query = query.Offset(offset)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	return r.collectPaths(ctx, query, "list paths")
}

// ListBySlugs returns the named paths with their members loaded.
func (r *gormPathRepository) ListBySlugs(ctx context.Context, slugs []string) ([]*content.Path, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	query := r.db.WithContext(ctx).Where("slug IN ?", slugs).Order("title")

	return r.collectPaths(ctx, query, "list paths by slug")
}

// Get returns one path with its members.
func (r *gormPathRepository) Get(ctx context.Context, slug string) (*content.Path, error) {
	var row models.Path

	err := r.db.WithContext(ctx).
		Preload("Courses", func(db *gorm.DB) *gorm.DB { return db.Order("path_courses.position") }).
		Preload("Skills", func(db *gorm.DB) *gorm.DB { return db.Order("path_skills.position") }).
		First(&row, "slug = ?", slug).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get path %s: %w", slug, err)
	}

	path := pathFromModel(&row)

	for _, member := range row.Courses {
		path.Courses = append(path.Courses, member.CourseSlug)
	}

	for _, member := range row.Skills {
		path.Skills = append(path.Skills, member.Skill)
	}

	return path, nil
}

// SlugsContainingCourse returns the slugs of every path that includes
// courseSlug, resolved by the index on path_courses.course_slug.
func (r *gormPathRepository) SlugsContainingCourse(ctx context.Context, courseSlug string) ([]string, error) {
	var slugs []string

	err := r.db.WithContext(ctx).
		Model(&models.PathCourse{}).
		Distinct().
		Where("course_slug = ?", courseSlug).
		Order("path_slug").
		Pluck("path_slug", &slugs).Error
	if err != nil {
		return nil, fmt.Errorf("list paths containing %s: %w", courseSlug, err)
	}

	return slugs, nil
}

// SkillsOfCourses returns the deduplicated union of the skills taught by
// the given courses, unnesting the denormalized skills column so the whole
// set resolves in one query rather than one per course.
func (r *gormPathRepository) SkillsOfCourses(ctx context.Context, courseSlugs []string) ([]string, error) {
	if len(courseSlugs) == 0 {
		return nil, nil
	}

	var skills []string

	err := r.db.WithContext(ctx).
		Table("courses").
		Select("DISTINCT s").
		Joins("CROSS JOIN UNNEST(courses.skills) AS s").
		Where("courses.slug IN ?", courseSlugs).
		Order("s").
		Pluck("s", &skills).Error
	if err != nil {
		return nil, fmt.Errorf("list skills of courses: %w", err)
	}

	return skills, nil
}

// SkillsByCourse returns the skills of each course in courseSlugs, keyed
// by course slug, unnesting the denormalized skills column so the whole
// set resolves in one query rather than one per course.
func (r *gormPathRepository) SkillsByCourse(ctx context.Context, courseSlugs []string) (map[string][]string, error) {
	bySlug := make(map[string][]string, len(courseSlugs))
	if len(courseSlugs) == 0 {
		return bySlug, nil
	}

	var rows []struct {
		Slug  string `gorm:"column:slug"`
		Skill string `gorm:"column:skill"`
	}

	err := r.db.WithContext(ctx).
		Table("courses").
		Select("courses.slug, s AS skill").
		Joins("CROSS JOIN UNNEST(courses.skills) AS s").
		Where("courses.slug IN ?", courseSlugs).
		Order("courses.slug, s").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list skills by course: %w", err)
	}

	for _, row := range rows {
		bySlug[row.Slug] = append(bySlug[row.Slug], row.Skill)
	}

	return bySlug, nil
}

// Create inserts a new path definition, reporting ErrConflict when the
// slug is already taken.
func (r *gormPathRepository) Create(ctx context.Context, path *content.Path) error {
	var existing int64

	err := r.db.WithContext(ctx).Model(&models.Path{}).Where("slug = ?", path.Slug).Count(&existing).Error
	if err != nil {
		return fmt.Errorf("check path %s: %w", path.Slug, err)
	}

	if existing > 0 {
		return ErrConflict
	}

	return r.Upsert(ctx, path)
}

// Upsert replaces a path definition wholesale, members included, in one
// transaction.
func (r *gormPathRepository) Upsert(ctx context.Context, path *content.Path) error {
	row, courses, skills := pathToModel(path)

	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		upsertErr := transaction.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: colSlug}},
			DoUpdates: clause.AssignmentColumns([]string{colTitle, colDescription, "kind", "level", colUpdatedAt}),
		}).Create(row).Error
		if upsertErr != nil {
			return fmt.Errorf("upsert path %s: %w", path.Slug, upsertErr)
		}

		return replacePathMembers(transaction, path.Slug, courses, skills)
	})
	if err != nil {
		return fmt.Errorf("write path %s: %w", path.Slug, err)
	}

	return nil
}

// replacePathMembers deletes and re-inserts a path's ordered courses and
// skills.
func replacePathMembers(transaction *gorm.DB, slug string, courses []models.PathCourse, skills []models.PathSkill) error {
	err := transaction.Where("path_slug = ?", slug).Delete(&models.PathCourse{}).Error
	if err != nil {
		return fmt.Errorf("clear courses of path %s: %w", slug, err)
	}

	err = transaction.Where("path_slug = ?", slug).Delete(&models.PathSkill{}).Error
	if err != nil {
		return fmt.Errorf("clear skills of path %s: %w", slug, err)
	}

	if len(courses) > 0 {
		err = transaction.Create(&courses).Error
		if err != nil {
			return fmt.Errorf("insert courses of path %s: %w", slug, err)
		}
	}

	if len(skills) > 0 {
		err = transaction.Create(&skills).Error
		if err != nil {
			return fmt.Errorf("insert skills of path %s: %w", slug, err)
		}
	}

	return nil
}

// Delete removes a path and its members.
func (r *gormPathRepository) Delete(ctx context.Context, slug string) error {
	result := r.db.WithContext(ctx).Delete(&models.Path{}, "slug = ?", slug)
	if result.Error != nil {
		return fmt.Errorf("delete path %s: %w", slug, result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// collectPaths runs a path query and loads the members of exactly the rows
// it returned, in two further queries — never one per path.
func (r *gormPathRepository) collectPaths(ctx context.Context, query *gorm.DB, what string) ([]*content.Path, error) {
	var rows []models.Path

	err := query.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	paths := make([]*content.Path, 0, len(rows))
	slugs := make([]string, 0, len(rows))
	bySlug := make(map[string]*content.Path, len(rows))

	for i := range rows {
		path := pathFromModel(&rows[i])

		paths = append(paths, path)
		slugs = append(slugs, path.Slug)
		bySlug[path.Slug] = path
	}

	err = r.attachMembers(ctx, slugs, bySlug)
	if err != nil {
		return nil, err
	}

	return paths, nil
}

// pathFromModel converts a persisted path row into its domain form,
// without its members.
func pathFromModel(row *models.Path) *content.Path {
	title := row.Title
	if title == "" {
		title = row.Slug
	}

	kind := row.Kind
	if kind == "" {
		kind = pathKindCourse
	}

	return &content.Path{
		Slug:        row.Slug,
		Title:       title,
		Description: row.Description,
		Kind:        kind,
		Level:       row.Level,
	}
}

// pathToModel converts a domain path into the row set that persists it,
// numbering members by their declared order and dropping empty entries.
func pathToModel(path *content.Path) (*models.Path, []models.PathCourse, []models.PathSkill) {
	title := path.Title
	if title == "" {
		title = path.Slug
	}

	kind := path.Kind
	if kind == "" {
		kind = pathKindCourse
	}

	row := &models.Path{
		Slug:        path.Slug,
		Title:       title,
		Description: path.Description,
		Kind:        kind,
		Level:       path.Level,
	}

	courses := make([]models.PathCourse, 0, len(path.Courses))

	for _, courseSlug := range path.Courses {
		if courseSlug == "" {
			continue
		}

		courses = append(courses, models.PathCourse{
			PathSlug:   path.Slug,
			Position:   len(courses),
			CourseSlug: courseSlug,
		})
	}

	skills := make([]models.PathSkill, 0, len(path.Skills))
	seen := make(map[string]struct{}, len(path.Skills))

	for _, skill := range path.Skills {
		if skill == "" {
			continue
		}

		if _, dup := seen[skill]; dup {
			continue
		}

		seen[skill] = struct{}{}

		skills = append(skills, models.PathSkill{
			PathSlug: path.Slug,
			Position: len(skills),
			Skill:    skill,
		})
	}

	return row, courses, skills
}

// attachMembers loads the ordered courses and skills of every path in
// slugs and attaches them to the matching entry of bySlug.
func (r *gormPathRepository) attachMembers(ctx context.Context, slugs []string, bySlug map[string]*content.Path) error {
	if len(slugs) == 0 {
		return nil
	}

	var courseRows []models.PathCourse

	err := r.db.WithContext(ctx).
		Where("path_slug IN ?", slugs).
		Order("path_slug, position").
		Find(&courseRows).Error
	if err != nil {
		return fmt.Errorf("load path courses: %w", err)
	}

	for _, row := range courseRows {
		if path, ok := bySlug[row.PathSlug]; ok {
			path.Courses = append(path.Courses, row.CourseSlug)
		}
	}

	var skillRows []models.PathSkill

	err = r.db.WithContext(ctx).
		Where("path_slug IN ?", slugs).
		Order("path_slug, position").
		Find(&skillRows).Error
	if err != nil {
		return fmt.Errorf("load path skills: %w", err)
	}

	for _, row := range skillRows {
		if path, ok := bySlug[row.PathSlug]; ok {
			path.Skills = append(path.Skills, row.Skill)
		}
	}

	return nil
}
