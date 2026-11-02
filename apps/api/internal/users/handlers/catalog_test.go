package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// catalogFixture is the catalog a mock course-service serves.
type catalogFixture struct {
	Courses map[string]CourseInfo
	Paths   map[string]PathInfo
	Skills  map[string][]SkillModule
	// Fail, when set, makes every batch endpoint answer 500.
	Fail bool
}

// newCatalogServer starts a mock course-service serving the batch endpoints
// the Catalog reads, and reports how many requests it received so tests can
// assert that a handler batches instead of looping.
func newCatalogServer(t *testing.T, fixture catalogFixture) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if fixture.Fail {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		slugs := requestedSlugs(r)

		switch r.URL.Path {
		case "/api/batch/courses":
			writeMatching(w, "courses", slugs, fixture.Courses)
		case "/api/batch/paths":
			writeMatching(w, "paths", slugs, fixture.Paths)
		case "/api/batch/skills":
			modules := make(map[string][]SkillModule, len(slugs))
			for _, slug := range slugs {
				if found, ok := fixture.Skills[slug]; ok {
					modules[slug] = found
				}
			}

			_ = json.NewEncoder(w).Encode(map[string]map[string][]SkillModule{"skills": modules})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)

	return server, &calls
}

// requestedSlugs parses the batch `slugs` query parameter.
func requestedSlugs(r *http.Request) []string {
	raw := r.URL.Query().Get("slugs")
	if raw == "" {
		return nil
	}

	return strings.Split(raw, ",")
}

// writeMatching writes the fixture entries named by slugs under key.
func writeMatching[T any](w http.ResponseWriter, key string, slugs []string, fixture map[string]T) {
	out := make([]T, 0, len(slugs))

	for _, slug := range slugs {
		if entry, ok := fixture[slug]; ok {
			out = append(out, entry)
		}
	}

	_ = json.NewEncoder(w).Encode(map[string][]T{key: out}) //nolint:errchkjson // fixture types marshal cleanly
}

// courseFixture builds a CourseInfo with an optional badge.
func courseFixture(slug, title, badgeName, badgeIcon string) CourseInfo {
	info := CourseInfo{Slug: slug, ID: slug, Title: title, IsPublic: true}
	if badgeName != "" || badgeIcon != "" {
		info.Badge = &struct {
			Name string `json:"name"`
			Icon string `json:"icon,omitempty"`
		}{Name: badgeName, Icon: badgeIcon}
	}

	return info
}

// TestCatalog_BatchesAndCaches is the guarantee the Catalog exists for: a
// set of slugs costs one upstream request, and asking again inside the TTL
// costs none.
func TestCatalog_BatchesAndCaches(t *testing.T) {
	t.Parallel()

	fixture := catalogFixture{Courses: map[string]CourseInfo{}}
	slugs := make([]string, 0, 50)

	for i := range 50 {
		slug := "course-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		fixture.Courses[slug] = courseFixture(slug, "Course "+slug, "", "")
		slugs = append(slugs, slug)
	}

	server, calls := newCatalogServer(t, fixture)

	catalog := NewCatalog(server.URL)

	first := catalog.Courses(t.Context(), slugs)
	if len(first) != len(slugs) {
		t.Fatalf("want %d courses, got %d", len(slugs), len(first))
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("want 1 upstream request for %d slugs, got %d", len(slugs), got)
	}

	second := catalog.Courses(t.Context(), slugs)
	if len(second) != len(slugs) {
		t.Fatalf("want %d courses on the cached read, got %d", len(slugs), len(second))
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("cached read should issue no upstream request, total calls %d", got)
	}
}

// TestCatalog_CachesAbsentSlugs verifies a slug the catalog does not know
// is remembered as absent rather than re-fetched on every lookup.
func TestCatalog_CachesAbsentSlugs(t *testing.T) {
	t.Parallel()

	server, calls := newCatalogServer(t, catalogFixture{Courses: map[string]CourseInfo{}})
	catalog := NewCatalog(server.URL)

	for range 3 {
		if _, found := catalog.Course(t.Context(), "ghost"); found {
			t.Fatal("want the unknown slug reported as absent")
		}
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("want the absent slug fetched once, got %d requests", got)
	}
}

// TestCatalog_UpstreamFailureIsEmpty verifies an unreachable course-service
// yields no entries rather than an error the caller has to handle.
func TestCatalog_UpstreamFailureIsEmpty(t *testing.T) {
	t.Parallel()

	server, _ := newCatalogServer(t, catalogFixture{Fail: true})

	catalog := NewCatalog(server.URL)

	if got := catalog.Courses(t.Context(), []string{"a", "b"}); len(got) != 0 {
		t.Errorf("want no courses from a failing upstream, got %v", got)
	}
}
