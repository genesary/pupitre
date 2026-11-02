package handlers

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// catalogTTL is how long a course or path definition fetched from
// course-service is reused before being fetched again.
//
// Course definitions change when an author edits them, which is rare, and
// every consumer here only needs the title, badge and member list. A short
// TTL keeps an edit visible within seconds while removing the bulk of the
// cross-service traffic: rendering a learner's dashboard used to fetch the
// same handful of courses on every page load, once per course.
const catalogTTL = 30 * time.Second

// catalogBatchSize is how many slugs are requested per batch call. It stays
// under course-service's own per-request slug cap.
const catalogBatchSize = 200

// catalogMaxEntries bounds each of the three caches. A platform's whole
// catalog is far smaller than this, so the cap only ever bites on a cache
// that has accumulated stale entries.
const catalogMaxEntries = 4096

// CourseInfo is the course metadata user-service consumes: enough to
// render a card, gate an enrollment, or label a badge.
type CourseInfo struct {
	Slug            string `json:"slug"`
	ID              string `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Difficulty      string `json:"difficulty"`
	IsPublic        bool   `json:"isPublic"`
	LabCount        int    `json:"labCount"`
	EnrollmentCount int    `json:"enrollmentCount"`
	XPRequired      int    `json:"xpRequired,omitempty"`
	Badge           *struct {
		Name string `json:"name"`
		Icon string `json:"icon,omitempty"`
	} `json:"badge,omitempty"`
}

// PathInfo is the learning-path metadata user-service consumes.
type PathInfo struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	Courses     []string `json:"courses,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

// SkillModule is one module teaching a skill, as reported by
// course-service.
type SkillModule struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Index       int    `json:"index"`
	Type        string `json:"type"`
	CourseSlug  string `json:"courseSlug"`
	CourseTitle string `json:"courseTitle"`
}

// Catalog is user-service's read-through view of the course catalog.
//
// It solves the same problem three times over — for courses, paths and
// skills — in the two ways that matter under load:
//
//   - Batching. Callers ask for a set of slugs and get one HTTP request
//     per batch, not one per slug. Listing a learner's twenty enrolled
//     courses was twenty serial round-trips to course-service.
//   - Caching. Entries are memoized for catalogTTL and concurrent misses
//     for the same key collapse into one upstream call, so a burst of
//     dashboard loads does not multiply into a burst of catalog fetches.
type Catalog struct {
	source CourseSource
	ttl    time.Duration
	// maxEntries bounds each cache, so a long-running replica cannot
	// accumulate an entry per slug it has ever been asked about.
	maxEntries int

	mu      sync.RWMutex
	courses map[string]catalogEntry[CourseInfo]
	paths   map[string]catalogEntry[PathInfo]
	skills  map[string]catalogEntry[[]SkillModule]

	group singleflight.Group

	// now is time.Now, overridable in tests.
	now func() time.Time
}

// catalogEntry is one cached value and the instant it expires. A miss and
// a known-absent slug are both represented: found=false is cached too, so
// a slug that no longer exists is not re-fetched on every request.
type catalogEntry[T any] struct {
	value     T
	found     bool
	expiresAt time.Time
}

// NewCatalogFrom builds a Catalog reading from source.
func NewCatalogFrom(source CourseSource) *Catalog {
	return &Catalog{
		source:     source,
		ttl:        catalogTTL,
		maxEntries: catalogMaxEntries,
		courses:    make(map[string]catalogEntry[CourseInfo]),
		paths:      make(map[string]catalogEntry[PathInfo]),
		skills:     make(map[string]catalogEntry[[]SkillModule]),
		now:        time.Now,
	}
}

// Courses resolves every slug in slugs, returning the ones course-service
// knows about. Cached entries are served from memory; the rest are fetched
// in batches.
func (c *Catalog) Courses(ctx context.Context, slugs []string) map[string]CourseInfo {
	return resolveCatalog(ctx, c, slugs, &c.courses, "courses", c.fetchCourses)
}

// Course resolves a single course slug.
func (c *Catalog) Course(ctx context.Context, slug string) (CourseInfo, bool) {
	info, found := c.Courses(ctx, []string{slug})[slug]

	return info, found
}

// Paths resolves every path slug in slugs.
func (c *Catalog) Paths(ctx context.Context, slugs []string) map[string]PathInfo {
	return resolveCatalog(ctx, c, slugs, &c.paths, "paths", c.fetchPaths)
}

// Path resolves a single path slug.
func (c *Catalog) Path(ctx context.Context, slug string) (PathInfo, bool) {
	info, found := c.Paths(ctx, []string{slug})[slug]

	return info, found
}

// SkillModules resolves the modules teaching each of skills.
func (c *Catalog) SkillModules(ctx context.Context, skills []string) map[string][]SkillModule {
	return resolveCatalog(ctx, c, skills, &c.skills, "skills", c.fetchSkillModules)
}

// Invalidate drops every cached entry. Exposed so a write path that knows
// it has changed the catalog does not have to wait out the TTL.
func (c *Catalog) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clear(c.courses)
	clear(c.paths)
	clear(c.skills)
}

// resolveCatalog is the shared read-through path for all three caches:
// serve what is fresh, fetch the rest in batches under a per-kind
// singleflight key, then cache both the hits and the misses.
func resolveCatalog[T any](
	ctx context.Context,
	catalog *Catalog,
	keys []string,
	cache *map[string]catalogEntry[T],
	kind string,
	fetch func(context.Context, []string) (map[string]T, error),
) map[string]T {
	out := make(map[string]T, len(keys))

	missing := readCached(catalog, keys, cache, out)
	if len(missing) == 0 {
		return out
	}

	for chunk := range slices.Chunk(missing, catalogBatchSize) {
		// Keyed by the exact set requested: two dashboards loading at once
		// ask for the same slugs, and one upstream call serves both.
		key := kind + ":" + strings.Join(chunk, ",")

		fetched, err, _ := catalog.group.Do(key, func() (any, error) {
			return fetch(ctx, chunk)
		})
		if err != nil {
			zap.L().Warn("catalog fetch failed",
				zap.String("kind", kind), zap.Int("slugs", len(chunk)), zap.Error(err))

			continue
		}

		found, _ := fetched.(map[string]T)

		storeFetched(catalog, chunk, found, cache)
		maps.Copy(out, found)
	}

	return out
}

// readCached copies every fresh entry for keys into out and returns the
// keys that still need fetching.
func readCached[T any](catalog *Catalog, keys []string, cache *map[string]catalogEntry[T], out map[string]T) []string {
	now := catalog.now()

	catalog.mu.RLock()
	defer catalog.mu.RUnlock()

	var missing []string

	for _, key := range keys {
		entry, cached := (*cache)[key]
		if !cached || now.After(entry.expiresAt) {
			missing = append(missing, key)

			continue
		}

		if entry.found {
			out[key] = entry.value
		}
	}

	return missing
}

// storeFetched caches a batch's results, recording the keys the upstream
// did not return as known-absent so they are not re-fetched immediately.
func storeFetched[T any](catalog *Catalog, keys []string, found map[string]T, cache *map[string]catalogEntry[T]) {
	now := catalog.now()
	expiresAt := now.Add(catalog.ttl)

	catalog.mu.Lock()
	defer catalog.mu.Unlock()

	for _, key := range keys {
		value, ok := found[key]
		(*cache)[key] = catalogEntry[T]{value: value, found: ok, expiresAt: expiresAt}
	}

	evictExpired(now, cache, catalog.maxEntries)
}

// evictExpired keeps a cache bounded. Expired entries are dropped first;
// if that is not enough, entries are dropped until the cache is back under
// its cap.
//
// Without this the cache would be a slow leak: entries expire but nothing
// ever removes them, so a catalog of ten thousand courses would eventually
// hold ten thousand stale entries in every replica. Which entries the
// second pass drops does not matter — every one of them is a cache miss
// away from being correct again.
func evictExpired[T any](now time.Time, cache *map[string]catalogEntry[T], maxEntries int) {
	if len(*cache) <= maxEntries {
		return
	}

	for key, entry := range *cache {
		if now.After(entry.expiresAt) {
			delete(*cache, key)
		}
	}

	for key := range *cache {
		if len(*cache) <= maxEntries {
			return
		}

		delete(*cache, key)
	}
}

// fetchCourses resolves a batch of course slugs from the courses domain.
func (c *Catalog) fetchCourses(ctx context.Context, slugs []string) (map[string]CourseInfo, error) {
	courses, err := c.source.Courses(ctx, slugs)
	if err != nil {
		return nil, fmt.Errorf("batch courses: %w", err)
	}

	out := make(map[string]CourseInfo, len(courses))
	for _, course := range courses {
		out[course.Slug] = course
	}

	return out, nil
}

// fetchPaths resolves a batch of path slugs from the courses domain.
func (c *Catalog) fetchPaths(ctx context.Context, slugs []string) (map[string]PathInfo, error) {
	paths, err := c.source.Paths(ctx, slugs)
	if err != nil {
		return nil, fmt.Errorf("batch paths: %w", err)
	}

	out := make(map[string]PathInfo, len(paths))
	for _, path := range paths {
		out[path.Slug] = path
	}

	return out, nil
}

// fetchSkillModules resolves the modules of a batch of skills.
func (c *Catalog) fetchSkillModules(ctx context.Context, skills []string) (map[string][]SkillModule, error) {
	modules, err := c.source.SkillModules(ctx, skills)
	if err != nil {
		return nil, fmt.Errorf("batch skill modules: %w", err)
	}

	return modules, nil
}

// CourseSource is what the users domain reads from the courses domain. In
// the monolith it calls the courses domain directly; the HTTP
// implementation below serves the standalone user-service binary until it
// is removed (docs/specs/104-backend-in-process-calls.md).
type CourseSource interface {
	// Courses returns the courses among slugs that exist, private ones
	// included.
	Courses(ctx context.Context, slugs []string) ([]CourseInfo, error)
	// Paths returns the paths among slugs that exist.
	Paths(ctx context.Context, slugs []string) ([]PathInfo, error)
	// SkillModules returns the modules teaching each skill.
	SkillModules(ctx context.Context, skills []string) (map[string][]SkillModule, error)
	// CourseSessions returns the sessions of a course, private or not: an
	// error for an unknown course.
	CourseSessions(ctx context.Context, courseSlug string) ([]CourseSessionInfo, error)
	// PublicCourseTitles returns the title of every public course, by slug,
	// as the anonymous catalogue lists them. CSV exports label rows with it.
	PublicCourseTitles(ctx context.Context) (map[string]string, error)
	// PathTitles returns the title of every path, by slug.
	PathTitles(ctx context.Context) (map[string]string, error)
}
