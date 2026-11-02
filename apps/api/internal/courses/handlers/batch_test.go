package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// batchRouter wires a public router over the given courses.
func batchRouter(courses ...*content.Course) http.Handler {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return BuildRouter(newStateWith(cfg, courses...), cfg, false)
}

// getJSON issues an unauthenticated GET and decodes the JSON body.
func getJSON(t *testing.T, router http.Handler, target string) (int, map[string]json.RawMessage) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body map[string]json.RawMessage

	if rec.Body.Len() > 0 {
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("decode %s: %v (body %s)", target, err, rec.Body.String())
		}
	}

	return rec.Code, body
}

// TestBatchRoutes_DoNotShadowASlug is the regression test for the batch
// endpoints having lived at /api/courses/batch and /api/paths/batch.
//
// chi matches a static segment before a wildcard, so a course legitimately
// slugged "batch" became unreachable: GET /api/courses/batch answered with a
// course *list* instead of that course. Moving the batch lookups under their
// own /api/batch prefix means no slug can collide with them.
func TestBatchRoutes_DoNotShadowASlug(t *testing.T) {
	t.Parallel()

	router := batchRouter(&content.Course{
		Slug: "batch", Title: "Batch Processing", IsPublic: true,
		Modules: []content.Module{{Name: "Intro", Type: "text", InlineContent: "x"}},
	})

	status, body := getJSON(t, router, "/api/courses/batch")
	if status != http.StatusOK {
		t.Fatalf("a course slugged \"batch\" must be reachable, got %d", status)
	}

	if _, isList := body["courses"]; isList {
		t.Fatal("GET /api/courses/batch returned a course list — the batch endpoint is shadowing the slug again")
	}

	var title string

	err := json.Unmarshal(body["title"], &title)
	if err != nil || title != "Batch Processing" {
		t.Errorf("want the course itself, got title %q (err %v)", title, err)
	}
}

// TestBatchRoutes_ResolveManySlugs covers the three batch lookups at their
// own prefix, including a slug the catalog does not know.
func TestBatchRoutes_ResolveManySlugs(t *testing.T) {
	t.Parallel()

	router := batchRouter(
		&content.Course{
			Slug: "linux", Title: "Linux", IsPublic: true,
			Modules: []content.Module{{Name: "Shell", Type: "text", InlineContent: "x", Skills: []string{"cli"}}},
		},
		&content.Course{Slug: "docker", Title: "Docker", IsPublic: true},
	)

	status, body := getJSON(t, router, "/api/batch/courses?slugs=linux,docker,absent")
	if status != http.StatusOK {
		t.Fatalf("batch courses: want 200, got %d", status)
	}

	var courses []CourseResponse

	err := json.Unmarshal(body["courses"], &courses)
	if err != nil {
		t.Fatalf("decode courses: %v", err)
	}

	if len(courses) != 2 {
		t.Errorf("want the 2 known courses and no entry for the absent slug, got %d", len(courses))
	}

	status, _ = getJSON(t, router, "/api/batch/paths?slugs=none")
	if status != http.StatusOK {
		t.Errorf("batch paths: want 200, got %d", status)
	}

	status, skillBody := getJSON(t, router, "/api/batch/skills?slugs=cli")
	if status != http.StatusOK {
		t.Fatalf("batch skills: want 200, got %d", status)
	}

	var bySkill map[string][]SkillModuleEntry

	err = json.Unmarshal(skillBody["skills"], &bySkill)
	if err != nil {
		t.Fatalf("decode skills: %v", err)
	}

	if len(bySkill["cli"]) != 1 || bySkill["cli"][0].CourseSlug != "linux" {
		t.Errorf("want the cli module attributed to linux, got %+v", bySkill["cli"])
	}
}

// TestBatchRoutes_RejectTooManySlugs pins the per-request cap.
func TestBatchRoutes_RejectTooManySlugs(t *testing.T) {
	t.Parallel()

	router := batchRouter()

	slugs := make([]byte, 0, maxBatchSlugs*4)
	for i := range maxBatchSlugs + 1 {
		if i > 0 {
			slugs = append(slugs, ',')
		}

		slugs = append(slugs, []byte("s")...)
		slugs = append(slugs, []byte{byte('a' + i%26), byte('a' + i/26)}...)
	}

	status, _ := getJSON(t, router, "/api/batch/courses?slugs="+string(slugs))
	if status != http.StatusBadRequest {
		t.Errorf("want 400 past the slug cap, got %d", status)
	}
}

// TestBatchCourses_PublicRouteHidesPrivateCourses is the regression test for
// defect B10: the public batch route listed private courses, while the
// in-process lookup of the users domain must still see them.
func TestBatchCourses_PublicRouteHidesPrivateCourses(t *testing.T) {
	t.Parallel()

	courses := []*content.Course{
		{Slug: "open", Title: "Open", IsPublic: true},
		{Slug: "closed", Title: "Closed", IsPublic: false},
	}

	status, body := getJSON(t, batchRouter(courses...), "/api/batch/courses?slugs=open,closed")
	if status != http.StatusOK {
		t.Fatalf("batch courses: status %d", status)
	}

	var listed []CourseResponse

	err := json.Unmarshal(body["courses"], &listed)
	if err != nil {
		t.Fatalf("decode courses: %v", err)
	}

	if len(listed) != 1 || listed[0].Slug != "open" {
		t.Fatalf("public batch route listed %+v, want the public course only", listed)
	}

	cfg := &config.Config{JWTSecret: "test-secret"}

	all, err := newStateWith(cfg, courses...).BatchCourses(t.Context(), []string{"open", "closed"})
	if err != nil || len(all) != 2 {
		t.Fatalf("in-process BatchCourses = %d courses, %v; want both", len(all), err)
	}
}
