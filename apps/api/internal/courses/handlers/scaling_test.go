package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// These tests pin the scaling properties of the request paths a learner
// exercises constantly. They assert two different things:
//
//   - Call counts. A handler must issue a fixed number of internal calls
//     regardless of how many modules, prerequisites or lessons the course
//     has. This is the real guarantee: it holds on any machine, and it is
//     what stops an N+1 from being reintroduced.
//   - Wall clock. Deliberately loose ceilings that only catch an order-of-
//     magnitude regression — a serial fan-out reappearing, or per-request
//     work that grows super-linearly. CI runners are far slower than a
//     developer machine, so the budgets are set with plenty of headroom
//     and every internal call is answered instantly by an in-process mock.
//
// Read a timing failure as "something got much slower", never as a precise
// measurement.

// countingUserService is a user-service mock that records how many times
// each internal endpoint was called.
type countingUserService struct {
	*httptest.Server

	mu    sync.Mutex
	calls map[string]int
}

// newCountingUserService starts a user-service mock that answers every
// internal endpoint with an empty-progress response and counts the calls.
func newCountingUserService(t *testing.T, overrides map[string]http.HandlerFunc) *countingUserService {
	t.Helper()

	counter := &countingUserService{calls: make(map[string]int)}
	inner := newUserServiceMockWith(overrides)

	counter.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.mu.Lock()
		counter.calls[r.URL.Path]++
		counter.mu.Unlock()

		inner.Config.Handler.ServeHTTP(w, r)
	}))

	t.Cleanup(func() {
		counter.Close()
		inner.Close()
	})

	return counter
}

// count returns how many times path was requested.
func (c *countingUserService) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls[path]
}

// total returns how many internal requests were made in all.
func (c *countingUserService) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	sum := 0
	for _, n := range c.calls {
		sum += n
	}

	return sum
}

// bigCourse builds a public course with moduleCount text modules and
// prereqCount cross-course prerequisites.
func bigCourse(slug string, moduleCount, prereqCount int) *content.Course {
	course := &content.Course{
		Slug: slug, Title: slug, IsPublic: true,
		Modules: make([]content.Module, 0, moduleCount),
	}

	for i := range moduleCount {
		course.Modules = append(course.Modules, content.Module{
			Name: fmt.Sprintf("Lesson %d", i), Type: "text", InlineContent: "body",
		})
	}

	for i := range prereqCount {
		course.Prerequisites = append(course.Prerequisites, content.CoursePrerequisite{
			Course: fmt.Sprintf("prereq-%d", i),
		})
	}

	return course
}

// getAs issues an authenticated student GET and returns the recorder.
func getAs(t *testing.T, router http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// routerForCourse wires a router serving exactly the given courses against
// the given user-service mock.
func routerForCourse(mockURL string, courses ...*content.Course) http.Handler {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return BuildRouter(withPeers(newStateWith(cfg, courses...), mockURL, ""), cfg, false)
}

// TestListModules_OneInternalCallWhateverTheSize pins the module listing at
// a single user-service round-trip: the learner's progress for the whole
// course arrives in one response, not one per module and not one per kind
// of progress.
func TestListModules_OneInternalCallWhateverTheSize(t *testing.T) {
	t.Parallel()

	for _, moduleCount := range []int{1, 200} {
		mock := newCountingUserService(t, nil)
		router := routerForCourse(mock.URL, bigCourse("big-course", moduleCount, 0))

		rec := getAs(t, router, "/api/courses/big-course/modules")
		if rec.Code != http.StatusOK {
			t.Fatalf("%d modules: want 200, got %d: %s", moduleCount, rec.Code, rec.Body.String())
		}

		if got := mock.count("/internal/progress/overview"); got != 1 {
			t.Errorf("%d modules: want 1 progress lookup, got %d", moduleCount, got)
		}

		if got := mock.total(); got != 1 {
			t.Errorf("%d modules: want 1 internal call in all, got %d", moduleCount, got)
		}
	}
}

// TestPrerequisites_OneInternalCallWhateverTheChain pins prerequisite
// checking at one batched call. It used to ask user-service once per
// prerequisite, in series, so the cost of opening a course grew with the
// length of its prerequisite chain.
func TestPrerequisites_OneInternalCallWhateverTheChain(t *testing.T) {
	t.Parallel()

	for _, prereqCount := range []int{1, 50} {
		mock := newCountingUserService(t, map[string]http.HandlerFunc{
			"/internal/progress/course-summaries": courseSummariesHandler(100, nil),
		})
		router := routerForCourse(mock.URL, bigCourse("gated", 3, prereqCount))

		rec := getAs(t, router, "/api/courses/gated/modules")
		if rec.Code != http.StatusOK {
			t.Fatalf("%d prerequisites: want 200, got %d: %s", prereqCount, rec.Code, rec.Body.String())
		}

		if got := mock.count("/internal/progress/course-summaries"); got != 1 {
			t.Errorf("%d prerequisites: want 1 summaries lookup, got %d", prereqCount, got)
		}
	}
}

// TestGetModule_ConstantInternalCalls pins rendering one module at a single
// progress lookup, including when the next module is an inline quiz —
// which used to trigger a second, separate progress fetch.
func TestGetModule_ConstantInternalCalls(t *testing.T) {
	t.Parallel()

	course := bigCourse("inline-course", 1, 0)
	course.Modules = append(course.Modules, content.Module{
		Name: "Check", Type: "quiz", Inline: true, PassingScore: 1,
		Questions: []content.Question{{
			ID: "q1", Type: "single", Points: 1,
			Answers: []content.Answer{{ID: "a", Text: "a", Correct: true}},
		}},
	})

	mock := newCountingUserService(t, nil)
	router := routerForCourse(mock.URL, course)

	rec := getAs(t, router, "/api/courses/inline-course/modules/0")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if got := mock.total(); got != 1 {
		t.Errorf("want 1 internal call to render a module with an inline quiz, got %d", got)
	}
}

// markLessonMaxInternalCalls is what marking a lesson complete is allowed
// to cost: the progress read, the completion write, and — only when that
// lesson finishes the course — the course-complete notification.
const markLessonMaxInternalCalls = 3

// TestMarkLessonComplete_BoundedInternalCalls pins the write path. It used
// to read the learner's progress again after writing, to decide whether the
// course was finished; it now credits the just-written lesson against the
// progress it already had.
func TestMarkLessonComplete_BoundedInternalCalls(t *testing.T) {
	t.Parallel()

	mock := newCountingUserService(t, nil)
	router := routerForCourse(mock.URL, bigCourse("write-course", 100, 0))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/write-course/lessons/lesson-0/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if got := mock.total(); got > markLessonMaxInternalCalls {
		t.Errorf("want at most %d internal calls to mark a lesson complete, got %d",
			markLessonMaxInternalCalls, got)
	}
}

// listModulesBudget bounds one module listing over a 200-module course
// against an in-process user-service. A single internal round-trip plus
// JSON encoding is sub-millisecond in practice; this leaves two orders of
// magnitude of headroom for a loaded CI runner.
const listModulesBudget = 150 * time.Millisecond

// TestListModules_LatencyBudget catches an order-of-magnitude regression in
// the module listing — a reintroduced serial fan-out, or per-module work
// that is no longer linear.
func TestListModules_LatencyBudget(t *testing.T) {
	t.Parallel()

	mock := newCountingUserService(t, nil)
	router := routerForCourse(mock.URL, bigCourse("timed-course", 200, 0))

	// One warm-up request so the measurement excludes first-call costs
	// (connection setup, lazily-built router state).
	getAs(t, router, "/api/courses/timed-course/modules")

	start := time.Now()

	rec := getAs(t, router, "/api/courses/timed-course/modules")

	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Modules []moduleResponse `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Modules) != 200 {
		t.Fatalf("want 200 modules, got %d", len(body.Modules))
	}

	if elapsed > listModulesBudget {
		t.Errorf("listing 200 modules took %v, budget %v — expect a reintroduced per-module round-trip",
			elapsed, listModulesBudget)
	}
}
