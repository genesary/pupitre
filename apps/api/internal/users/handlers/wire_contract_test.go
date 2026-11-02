package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// The frontend reads these endpoints field by field, in plain JavaScript,
// with no generated client and no compile-time check that the names still
// exist — a renamed or dropped field surfaces as `undefined` in a browser,
// not as a failing build.
//
// Each case below lists the fields one page of `frontend/src/pages` actually
// reads off the response. They are asserted against a rendered response, so
// a refactor that reshapes a handler fails here rather than in the UI.
//
// Adding a field is always fine. Removing or renaming one means the page
// that reads it has to change in the same commit.

// wireCase is one endpoint and the field names its consumer depends on.
type wireCase struct {
	name string
	// path is the endpoint, rendered as the student in scalingUserID.
	path string
	// collection is the top-level array key in the response body.
	collection string
	// itemFields are read off each element of that array.
	itemFields []string
	// nested, when set, names an array field inside each element and the
	// fields read off *its* elements.
	nestedKey    string
	nestedFields []string
}

// TestWireContract_FrontendFields pins the response fields the frontend
// consumes on the learner-facing dashboard endpoints.
func TestWireContract_FrontendFields(t *testing.T) {
	t.Parallel()

	cases := []wireCase{
		{
			// frontend/src/pages/my-courses.astro
			name:       "my courses",
			path:       "/api/my/courses",
			collection: "courses",
			itemFields: []string{
				"slug", "id", "title", "description", "labCount",
				"completedLabs", "completed", "totalScore", "lastActivity",
			},
		},
		{
			// frontend/src/pages/my-paths.astro
			name:         "my paths",
			path:         "/api/my/paths",
			collection:   "paths",
			itemFields:   []string{"slug", "title", "description", "kind", "courses"},
			nestedKey:    "courses",
			nestedFields: []string{"slug", "status"},
		},
		{
			// frontend/src/pages/my-badges.astro
			name:       "my badges",
			path:       "/api/my/badges",
			collection: "badges",
			itemFields: []string{"courseSlug", "courseTitle", "name", "icon", "earnedAt", "earnedBy"},
		},
		{
			// frontend/src/pages/my-groups.astro
			name:       "my groups",
			path:       "/api/my/groups",
			collection: "groups",
			itemFields: []string{"id", "name", "source", "mappedRole", "courseSlugs"},
		},
		{
			// frontend/src/pages/skills/[slug].astro
			name:       "skill modules",
			path:       "/api/my/skills/linux",
			collection: "modules",
			itemFields: []string{"slug", "courseSlug", "status"},
		},
	}

	router := wireContractRouter(t)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rec := htDo(t, router, http.MethodGet, testCase.path, "",
				htAuthHeaderForSubject(t, "student", scalingUserID))
			if rec.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var body map[string]json.RawMessage

			err := json.Unmarshal(rec.Body.Bytes(), &body)
			if err != nil {
				t.Fatalf("decode body: %v", err)
			}

			items := decodeItems(t, body, testCase.collection)
			if len(items) == 0 {
				t.Fatalf("fixture produced no %s to check", testCase.collection)
			}

			assertFields(t, testCase.collection, items[0], testCase.itemFields)

			if testCase.nestedKey == "" {
				return
			}

			nested := decodeItems(t, items[0], testCase.nestedKey)
			if len(nested) == 0 {
				t.Fatalf("fixture produced no %s to check", testCase.nestedKey)
			}

			assertFields(t, testCase.nestedKey, nested[0], testCase.nestedFields)
		})
	}
}

// decodeItems reads the array stored under key and returns its elements as
// raw JSON objects.
func decodeItems(t *testing.T, object map[string]json.RawMessage, key string) []map[string]json.RawMessage {
	t.Helper()

	raw, present := object[key]
	if !present {
		t.Fatalf("response has no %q key; the frontend reads it", key)
	}

	var items []map[string]json.RawMessage

	err := json.Unmarshal(raw, &items)
	if err != nil {
		t.Fatalf("decode %q as an array of objects: %v", key, err)
	}

	return items
}

// assertFields fails for every field the consumer reads that the response
// does not carry.
func assertFields(t *testing.T, what string, item map[string]json.RawMessage, fields []string) {
	t.Helper()

	for _, field := range fields {
		if _, present := item[field]; !present {
			t.Errorf("%s item is missing %q — the frontend reads it; got keys %v",
				what, field, sortedKeys(item))
		}
	}
}

// sortedKeys lists an object's keys for a failure message.
func sortedKeys(item map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(item))
	for key := range item {
		keys = append(keys, key)
	}

	return keys
}

// wireContractRouter builds a router whose fixtures produce one non-empty
// item for every collection the contract checks.
func wireContractRouter(t *testing.T) http.Handler {
	t.Helper()

	userID := uuid.MustParse(scalingUserID)
	groupID := uuid.New()

	server, _ := newCatalogServer(t, catalogFixture{
		Courses: map[string]CourseInfo{
			"linux-intro": courseFixture("linux-intro", "Linux Intro", "Tux", "🐧"),
		},
		Paths: map[string]PathInfo{
			"devops": {
				Slug: "devops", Title: "DevOps", Description: "A path",
				Kind: "course", Courses: []string{"linux-intro"},
			},
		},
		Skills: map[string][]SkillModule{
			"linux": {{Slug: "quiz-linux", Type: "quiz", CourseSlug: "linux-intro", Index: 0}},
		},
	})

	users := fake.NewUserRepository()

	err := users.Create(t.Context(), &models.User{
		ID: userID, Email: "learner@test.com", Username: "learner", Role: "student",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	badges := fake.NewBadgeRepository()

	err = badges.Award(t.Context(), scalingUserID, "linux-intro")
	if err != nil {
		t.Fatalf("seed badge: %v", err)
	}

	groups := fake.NewGroupRepository(models.Group{ID: groupID, Name: "team", Source: "local"})
	groups.UserGroup = []models.UserGroup{{UserID: scalingUserID, GroupID: groupID}}
	groups.GroupEnrollments = []models.GroupEnrollment{{GroupID: groupID, CourseSlug: "linux-intro"}}
	// Both mappedRole and kind are `omitempty`; the fixture gives them values
	// so the contract checks the field is emitted, not that it is always set.
	groups.Mappings = []models.GroupRoleMapping{{GroupName: "team", PlatformRole: "admin"}}

	repos := fake.NewRepositories()
	repos.Users = users
	repos.Badges = badges
	repos.Groups = groups
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: scalingUserID, CourseSlug: "linux-intro",
	})
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "devops", EnrolledAt: time.Now(),
	})

	state := scalingState(repos, server.URL)

	return BuildRouter(state, state.Config, false)
}
