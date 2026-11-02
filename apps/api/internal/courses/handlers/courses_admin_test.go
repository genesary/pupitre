package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
)

// newAdminState builds a State with an empty catalogue, for the admin
// create/read/update/delete tests.
func newAdminState(t *testing.T) (*State, http.Handler) {
	t.Helper()

	mock := newUserServiceMock()
	t.Cleanup(mock.Close)

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	state := withPeers(newStateWith(cfg), mock.URL, "")

	return state, BuildRouter(state, cfg, false)
}

// adminRequest issues an authenticated admin request, with an optional
// JSON body.
func adminRequest(t *testing.T, router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// courseDefinitionBody is a course create payload covering the nested-spec
// shape the admin UI sends.
const courseDefinitionBody = `{
  "slug": "kubernetes-basics",
  "spec": {
    "title": "Kubernetes Basics",
    "description": "Intro to K8s",
    "public": true,
    "category": "kubernetes",
    "difficulty": "beginner",
    "modules": [
      {"name": "What is K8s", "type": "text", "src": "https://github.com/org/repo",
       "ref": "main", "path": "intro.md", "skills": ["kubernetes"]},
      {"name": "Quiz", "type": "quiz", "passingScore": 80,
       "questions": [{"type": "single", "question": "Q?",
                      "answers": [{"id": "a", "text": "A", "correct": true}]}]}
    ],
    "prerequisites": [{"course": "linux-intro", "minScore": 30}],
    "sessions": {"sess-1": {"title": "Cohort 1", "date": "2026-09-01T10:00:00Z", "capacity": 12}}
  }
}`

// TestAdminCourse_CreateReadUpdateDelete walks a course through the whole
// admin lifecycle and checks it is readable back in the same shape it was
// written — which is what lets the admin UI fetch, edit and re-submit it.
func TestAdminCourse_CreateReadUpdateDelete(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", courseDefinitionBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/admin/courses/kubernetes-basics/definition", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var read struct {
		Slug string            `json:"slug"`
		Spec definition.Course `json:"spec"`
	}

	err := json.NewDecoder(rec.Body).Decode(&read)
	if err != nil {
		t.Fatalf("decode definition: %v", err)
	}

	if read.Slug != "kubernetes-basics" || read.Spec.Title != "Kubernetes Basics" || !read.Spec.Public {
		t.Errorf("definition not round-tripped: %+v", read)
	}

	if len(read.Spec.Modules) != 2 || read.Spec.Modules[0].Name != "What is K8s" {
		t.Fatalf("modules not round-tripped: %+v", read.Spec.Modules)
	}

	if len(read.Spec.Modules[1].Questions) != 1 || read.Spec.Modules[1].PassingScore != 80 {
		t.Errorf("quiz not round-tripped: %+v", read.Spec.Modules[1])
	}

	if len(read.Spec.Prerequisites) != 1 || read.Spec.Prerequisites[0].Course != "linux-intro" {
		t.Errorf("prerequisites not round-tripped: %+v", read.Spec.Prerequisites)
	}

	if session, ok := read.Spec.Sessions["sess-1"]; !ok || session.Capacity != 12 {
		t.Errorf("sessions not round-tripped: %+v", read.Spec.Sessions)
	}

	// Re-submitting the definition with one field changed is exactly what the
	// admin UI does after an edit.
	read.Spec.Title = "Kubernetes Basics (v2)"

	updated, err := json.Marshal(map[string]any{"spec": read.Spec})
	if err != nil {
		t.Fatalf("encode update: %v", err)
	}

	rec = adminRequest(t, router, http.MethodPut,
		"/api/admin/courses/kubernetes-basics/definition", string(updated))
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/courses/kubernetes-basics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("public read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var public CourseResponse

	err = json.NewDecoder(rec.Body).Decode(&public)
	if err != nil {
		t.Fatalf("decode course: %v", err)
	}

	if public.Title != "Kubernetes Basics (v2)" {
		t.Errorf("update not visible: %q", public.Title)
	}

	if public.ModuleCount != 2 {
		t.Errorf("want moduleCount 2, got %d", public.ModuleCount)
	}

	rec = adminRequest(t, router, http.MethodDelete, "/api/admin/courses/kubernetes-basics/definition", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/admin/courses/kubernetes-basics/definition", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("read after delete: want 404, got %d", rec.Code)
	}
}

// TestAdminCourse_CreateFlatBody verifies the admin API also accepts a
// definition supplied as flat top-level fields rather than nested under
// "spec".
func TestAdminCourse_CreateFlatBody(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	body := `{"slug": "flat-course", "title": "Flat Course", "public": true, "category": "linux"}`

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/courses/flat-course", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var public CourseResponse

	err := json.NewDecoder(rec.Body).Decode(&public)
	if err != nil {
		t.Fatalf("decode course: %v", err)
	}

	if public.Title != "Flat Course" || public.Category != "linux" {
		t.Errorf("flat fields not applied: %+v", public)
	}
}

// TestAdminCourse_CreateDuplicateConflicts verifies a repeated slug is
// rejected rather than silently overwriting the existing course.
func TestAdminCourse_CreateDuplicateConflicts(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	body := `{"slug": "dup", "title": "First"}`
	if rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create: want 201, got %d", rec.Code)
	}

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", `{"slug": "dup", "title": "Second"}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("second create: want 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminCourse_CreateRequiresSlug verifies a definition without a slug is
// rejected.
func TestAdminCourse_CreateRequiresSlug(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", `{"title": "No slug"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminCourse_UpdateUnknownCourse verifies updating a course that does
// not exist is a 404 rather than an implicit create.
func TestAdminCourse_UpdateUnknownCourse(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPut,
		"/api/admin/courses/ghost/definition", `{"spec": {"title": "Ghost"}}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminCourse_InvalidJSON verifies a malformed body is a 400.
func TestAdminCourse_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", `{bad json`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminPath_CreateReadUpdateDelete walks a learning path through its
// admin lifecycle and checks it is served back with its members in order.
func TestAdminPath_CreateReadUpdateDelete(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	body := `{"slug": "devops", "spec": {"title": "DevOps", "kind": "course",
	          "courses": ["linux-intro", "docker-fundamentals"]}}`

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/paths", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/paths/devops", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var path struct {
		Slug    string   `json:"slug"`
		Title   string   `json:"title"`
		Courses []string `json:"courses"`
	}

	err := json.NewDecoder(rec.Body).Decode(&path)
	if err != nil {
		t.Fatalf("decode path: %v", err)
	}

	if path.Title != "DevOps" || len(path.Courses) != 2 || path.Courses[0] != "linux-intro" {
		t.Errorf("path not round-tripped: %+v", path)
	}

	rec = adminRequest(t, router, http.MethodPut, "/api/admin/courses/paths/devops/definition",
		`{"spec": {"title": "DevOps v2", "courses": ["linux-intro"]}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodDelete, "/api/admin/courses/paths/devops/definition", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/paths/devops", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("read after delete: want 404, got %d", rec.Code)
	}
}

// TestAdminPath_ReadDefinition verifies the admin definition endpoint serves
// a path back in the shape the create endpoint accepts, with the stored
// skills rather than the aggregate the public endpoint substitutes.
func TestAdminPath_ReadDefinition(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	body := `{"slug": "linux-skills", "spec": {"title": "Linux Skills", "kind": "skill",
	          "description": "Skill by skill", "level": "beginner",
	          "skills": ["shell", "systemd"]}}`

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/paths", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/admin/courses/paths/linux-skills/definition", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var read struct {
		Slug string          `json:"slug"`
		Spec definition.Path `json:"spec"`
	}

	err := json.NewDecoder(rec.Body).Decode(&read)
	if err != nil {
		t.Fatalf("decode definition: %v", err)
	}

	if read.Slug != "linux-skills" || read.Spec.Title != "Linux Skills" || read.Spec.Kind != "skill" {
		t.Errorf("definition not round-tripped: %+v", read)
	}

	if read.Spec.Level != "beginner" || read.Spec.Description != "Skill by skill" {
		t.Errorf("metadata not round-tripped: %+v", read.Spec)
	}

	if len(read.Spec.Skills) != 2 || read.Spec.Skills[0] != "shell" {
		t.Errorf("skills not round-tripped: %+v", read.Spec.Skills)
	}
}

// TestAdminPath_ReadUnknownDefinition verifies an unknown slug is a 404.
func TestAdminPath_ReadUnknownDefinition(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodGet, "/api/admin/courses/paths/ghost/definition", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminDefinition_RejectsUnroutableSlug verifies a slug that could never
// appear in a URL is refused rather than stored out of reach.
func TestAdminDefinition_RejectsUnroutableSlug(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{"Not Lowercase", "with/slash", "-leading-dash", "accentué"} {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()

			_, router := newAdminState(t)

			body := `{"slug": ` + strconv.Quote(slug) + `, "spec": {"title": "Nope"}}`

			rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("course: want 400, got %d: %s", rec.Code, rec.Body.String())
			}

			rec = adminRequest(t, router, http.MethodPost, "/api/admin/courses/paths", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("path: want 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminPath_CreateDuplicateConflicts verifies a repeated path slug is
// rejected.
func TestAdminPath_CreateDuplicateConflicts(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	body := `{"slug": "dup-path", "spec": {"title": "First"}}`
	if rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/paths", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create: want 201, got %d", rec.Code)
	}

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/paths", body)
	if rec.Code != http.StatusConflict {
		t.Errorf("second create: want 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
