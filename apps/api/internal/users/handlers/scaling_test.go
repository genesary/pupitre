package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// These tests pin the scaling properties of the dashboard and reporting
// endpoints. They assert two different things:
//
//   - Call counts. A handler must issue a fixed number of course-service
//     requests and database queries regardless of how many courses, badges,
//     paths or enrolled learners it is rendering. This is the real
//     guarantee: it holds on any machine, and it is what stops an N+1 from
//     being reintroduced.
//   - Wall clock. Deliberately loose ceilings that only catch an order-of-
//     magnitude regression. CI runners are far slower than a developer
//     machine and every dependency here is in-process, so the budgets are
//     set with plenty of headroom.
//
// Read a timing failure as "something got much slower", never as a precise
// measurement.

// scalingUserID is the learner every scaling test renders a dashboard for.
const scalingUserID = "11111111-1111-1111-1111-111111111111"

// countingLessonProgress counts calls to the batched completion query, so a
// test can assert that a cohort costs one query rather than one per user.
type countingLessonProgress struct {
	*fake.LessonProgressRepository

	byUsersCalls atomic.Int64
}

// CompletedCourseSlugsByUsers records the call and delegates.
func (c *countingLessonProgress) CompletedCourseSlugsByUsers(
	ctx context.Context, userIDs, slugs []string,
) (map[string][]string, error) {
	c.byUsersCalls.Add(1)

	completed, err := c.LessonProgressRepository.CompletedCourseSlugsByUsers(ctx, userIDs, slugs)
	if err != nil {
		return nil, fmt.Errorf("completed course slugs by users: %w", err)
	}

	return completed, nil
}

// countingBadges counts calls to the batched earner-count query.
type countingBadges struct {
	*fake.BadgeRepository

	countsCalls atomic.Int64
}

// EarnedCounts records the call and delegates.
func (c *countingBadges) EarnedCounts(ctx context.Context, courseSlugs []string) (map[string]int64, error) {
	c.countsCalls.Add(1)

	counts, err := c.BadgeRepository.EarnedCounts(ctx, courseSlugs)
	if err != nil {
		return nil, fmt.Errorf("earned counts: %w", err)
	}

	return counts, nil
}

// scalingState wires a State against a mock course-service.
func scalingState(repos *repository.Repositories, courseServiceURL string) *State {
	return &State{
		Repos: repos,
		Config: &config.Config{
			JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
			LeaderboardMaxEntries: 100,
		},
		Courses: courseSourceAt(courseServiceURL),
	}
}

// scalingSlugs builds n distinct course slugs.
func scalingSlugs(n int) []string {
	slugs := make([]string, 0, n)
	for i := range n {
		slugs = append(slugs, fmt.Sprintf("course-%03d", i))
	}

	return slugs
}

// TestMyCourses_OneCatalogRequestWhateverTheCount pins the enrolled-course
// listing at one course-service request. It used to fetch each course's
// metadata separately, in series, so a learner with fifty enrollments paid
// fifty round-trips to render one page.
func TestMyCourses_OneCatalogRequestWhateverTheCount(t *testing.T) {
	t.Parallel()

	for _, courseCount := range []int{1, 100} {
		slugs := scalingSlugs(courseCount)

		fixture := catalogFixture{Courses: map[string]CourseInfo{}}
		enrollments := make([]models.Enrollment, 0, courseCount)

		for _, slug := range slugs {
			fixture.Courses[slug] = courseFixture(slug, "Course "+slug, "", "")
			enrollments = append(enrollments, models.Enrollment{UserID: scalingUserID, CourseSlug: slug})
		}

		server, calls := newCatalogServer(t, fixture)

		repos := fake.NewRepositories()
		repos.Enrollments = fake.NewEnrollmentRepository(enrollments...)

		state := scalingState(repos, server.URL)
		router := BuildRouter(state, state.Config, false)

		rec := htDo(t, router, http.MethodGet, "/api/my/courses", "",
			htAuthHeaderForSubject(t, "student", scalingUserID))
		if rec.Code != http.StatusOK {
			t.Fatalf("%d courses: want 200, got %d: %s", courseCount, rec.Code, rec.Body.String())
		}

		if got := calls.Load(); got != 1 {
			t.Errorf("%d courses: want 1 course-service request, got %d", courseCount, got)
		}
	}
}

// TestMyBadges_ConstantLookupsWhateverTheCount pins the badge listing at one
// course-service request and one earner-count query. It used to issue an
// HTTP request and a COUNT per badge, in series.
func TestMyBadges_ConstantLookupsWhateverTheCount(t *testing.T) {
	t.Parallel()

	for _, badgeCount := range []int{1, 100} {
		slugs := scalingSlugs(badgeCount)

		fixture := catalogFixture{Courses: map[string]CourseInfo{}}
		badges := &countingBadges{BadgeRepository: fake.NewBadgeRepository()}

		for _, slug := range slugs {
			fixture.Courses[slug] = courseFixture(slug, "Course "+slug, "Badge", "🏅")

			err := badges.Award(t.Context(), scalingUserID, slug)
			if err != nil {
				t.Fatalf("seed badge: %v", err)
			}
		}

		server, calls := newCatalogServer(t, fixture)

		repos := fake.NewRepositories()
		repos.Badges = badges

		state := scalingState(repos, server.URL)
		router := BuildRouter(state, state.Config, false)

		rec := htDo(t, router, http.MethodGet, "/api/my/badges", "",
			htAuthHeaderForSubject(t, "student", scalingUserID))
		if rec.Code != http.StatusOK {
			t.Fatalf("%d badges: want 200, got %d: %s", badgeCount, rec.Code, rec.Body.String())
		}

		if got := calls.Load(); got != 1 {
			t.Errorf("%d badges: want 1 course-service request, got %d", badgeCount, got)
		}

		if got := badges.countsCalls.Load(); got != 1 {
			t.Errorf("%d badges: want 1 earner-count query, got %d", badgeCount, got)
		}
	}
}

// TestMyPaths_ConstantCatalogRequests pins the learning-path dashboard at a
// fixed number of course-service requests: one for the paths themselves and
// one for the skills they name. Resolving each enrollment on its own turned
// a dashboard with ten paths into dozens of serial round-trips.
func TestMyPaths_ConstantCatalogRequests(t *testing.T) {
	t.Parallel()

	// One paths request; the fixture has no skill-kind path, so no skill
	// request is made either.
	const wantRequests = 1

	userID := uuid.MustParse(scalingUserID)

	for _, pathCount := range []int{1, 40} {
		fixture := catalogFixture{Paths: map[string]PathInfo{}}
		enrollments := make([]models.PathEnrollment, 0, pathCount)

		for i := range pathCount {
			slug := fmt.Sprintf("path-%03d", i)
			fixture.Paths[slug] = PathInfo{
				Slug: slug, Title: "Path " + slug,
				Courses: []string{slug + "-a", slug + "-b"},
			}
			enrollments = append(enrollments, models.PathEnrollment{
				UserID: userID, PathSlug: slug, EnrolledAt: time.Now(),
			})
		}

		server, calls := newCatalogServer(t, fixture)

		repos := fake.NewRepositories()
		repos.Paths = fake.NewPathEnrollmentRepository(enrollments...)

		state := scalingState(repos, server.URL)
		router := BuildRouter(state, state.Config, false)

		rec := htDo(t, router, http.MethodGet, "/api/my/paths", "",
			htAuthHeaderForSubject(t, "student", scalingUserID))
		if rec.Code != http.StatusOK {
			t.Fatalf("%d paths: want 200, got %d: %s", pathCount, rec.Code, rec.Body.String())
		}

		if got := calls.Load(); got != wantRequests {
			t.Errorf("%d paths: want %d course-service requests, got %d", pathCount, wantRequests, got)
		}
	}
}

// TestMyPaths_SkillPathsBatchTheirSkills pins a skill-kind dashboard at two
// course-service requests — the paths and, once, every skill they name —
// however many skills are involved.
func TestMyPaths_SkillPathsBatchTheirSkills(t *testing.T) {
	t.Parallel()

	const wantRequests = 2 // one for the paths, one for all their skills

	userID := uuid.MustParse(scalingUserID)

	for _, pathCount := range []int{1, 20} {
		fixture := catalogFixture{
			Paths:  map[string]PathInfo{},
			Skills: map[string][]SkillModule{},
		}
		enrollments := make([]models.PathEnrollment, 0, pathCount)

		for i := range pathCount {
			slug := fmt.Sprintf("skill-path-%03d", i)
			skill := fmt.Sprintf("skill-%03d", i)

			fixture.Paths[slug] = PathInfo{
				Slug: slug, Title: "Path " + slug, Kind: pathKindSkill, Skills: []string{skill},
			}
			fixture.Skills[skill] = []SkillModule{
				{Slug: "quiz-" + skill, Type: "quiz", CourseSlug: "course-" + skill},
			}

			enrollments = append(enrollments, models.PathEnrollment{
				UserID: userID, PathSlug: slug, EnrolledAt: time.Now(),
			})
		}

		server, calls := newCatalogServer(t, fixture)

		repos := fake.NewRepositories()
		repos.Paths = fake.NewPathEnrollmentRepository(enrollments...)

		state := scalingState(repos, server.URL)
		router := BuildRouter(state, state.Config, false)

		rec := htDo(t, router, http.MethodGet, "/api/my/paths", "",
			htAuthHeaderForSubject(t, "student", scalingUserID))
		if rec.Code != http.StatusOK {
			t.Fatalf("%d skill paths: want 200, got %d: %s", pathCount, rec.Code, rec.Body.String())
		}

		if got := calls.Load(); got != wantRequests {
			t.Errorf("%d skill paths: want %d course-service requests, got %d", pathCount, wantRequests, got)
		}
	}
}

// TestAdminListPathEnrollments_OneQueryWhateverTheCohort pins the admin
// path-enrollment report at a single completion query. It used to run one
// per enrolled learner, so a path with a thousand learners ran a thousand
// queries to fill in the same handful of course slugs.
func TestAdminListPathEnrollments_OneQueryWhateverTheCohort(t *testing.T) {
	t.Parallel()

	for _, cohort := range []int{1, 300} {
		users := fake.NewUserRepository()
		enrollments := make([]models.PathEnrollment, 0, cohort)
		progress := make([]models.LessonProgress, 0, cohort)

		for i := range cohort {
			userID := uuid.New()

			err := users.Create(t.Context(), &models.User{
				ID: userID, Email: fmt.Sprintf("u%d@test.com", i),
				Username: fmt.Sprintf("u%d", i), Role: "student",
			})
			if err != nil {
				t.Fatalf("seed user: %v", err)
			}

			enrollments = append(enrollments, models.PathEnrollment{
				UserID: userID, PathSlug: "devops-path", EnrolledAt: time.Now(),
			})
			progress = append(progress, models.LessonProgress{
				UserID: userID.String(), CourseSlug: "linux-intro", LessonSlug: fake.LessonSlugComplete,
			})
		}

		paths := fake.NewPathEnrollmentRepository(enrollments...)
		paths.Users = users

		lessons := &countingLessonProgress{
			LessonProgressRepository: fake.NewLessonProgressRepository(progress...),
		}

		server, _ := newCatalogServer(t, catalogFixture{Paths: map[string]PathInfo{
			"devops-path": {
				Slug: "devops-path", Title: "DevOps",
				Courses: []string{"linux-intro", "docker-fundamentals"},
			},
		}})

		repos := fake.NewRepositories()
		repos.Users = users
		repos.Paths = paths
		repos.LessonProgress = lessons

		state := scalingState(repos, server.URL)
		router := BuildRouter(state, state.Config, false)

		rec := htDo(t, router, http.MethodGet, "/api/admin/paths/devops-path/enrollments", "",
			htAuthHeader(t, "admin"))
		if rec.Code != http.StatusOK {
			t.Fatalf("cohort %d: want 200, got %d: %s", cohort, rec.Code, rec.Body.String())
		}

		if got := lessons.byUsersCalls.Load(); got != 1 {
			t.Errorf("cohort %d: want 1 completion query, got %d", cohort, got)
		}

		var body struct {
			Users []struct {
				CompletedCourses int `json:"completedCourses"`
			} `json:"users"`
		}

		err := json.NewDecoder(rec.Body).Decode(&body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(body.Users) != cohort {
			t.Fatalf("cohort %d: want %d users reported, got %d", cohort, cohort, len(body.Users))
		}

		if body.Users[0].CompletedCourses != 1 {
			t.Errorf("cohort %d: want each learner credited with 1 completed course, got %d",
				cohort, body.Users[0].CompletedCourses)
		}
	}
}

// dashboardBudget bounds one /api/my/courses render over 100 enrollments
// against an in-process course-service. The work is one batched fetch plus
// JSON encoding — sub-millisecond in practice — so this leaves two orders
// of magnitude of headroom for a loaded CI runner.
const dashboardBudget = 250 * time.Millisecond

// TestMyCourses_LatencyBudget catches an order-of-magnitude regression in
// the enrolled-course dashboard: a reintroduced per-course round-trip, or
// per-course work that is no longer linear.
func TestMyCourses_LatencyBudget(t *testing.T) {
	t.Parallel()

	const courseCount = 100

	slugs := scalingSlugs(courseCount)

	fixture := catalogFixture{Courses: map[string]CourseInfo{}}
	enrollments := make([]models.Enrollment, 0, courseCount)

	for _, slug := range slugs {
		fixture.Courses[slug] = courseFixture(slug, "Course "+slug, "", "")
		enrollments = append(enrollments, models.Enrollment{UserID: scalingUserID, CourseSlug: slug})
	}

	server, _ := newCatalogServer(t, fixture)

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(enrollments...)

	state := scalingState(repos, server.URL)
	router := BuildRouter(state, state.Config, false)

	auth := htAuthHeaderForSubject(t, "student", scalingUserID)

	// One warm-up request so the measurement excludes first-call costs
	// (connection setup, the catalog's initial fetch).
	htDo(t, router, http.MethodGet, "/api/my/courses", "", auth)

	start := time.Now()

	rec := htDo(t, router, http.MethodGet, "/api/my/courses", "", auth)

	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if elapsed > dashboardBudget {
		t.Errorf("rendering %d enrolled courses took %v, budget %v — expect a reintroduced per-course round-trip",
			courseCount, elapsed, dashboardBudget)
	}
}
