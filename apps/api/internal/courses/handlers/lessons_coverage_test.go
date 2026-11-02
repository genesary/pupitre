package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// getLesson issues an authenticated student GET for one lesson.
func getLesson(t *testing.T, s *State, courseSlug, lessonSlug string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/courses/"+courseSlug+"/lessons/"+lessonSlug, http.NoBody)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// completeLesson issues an authenticated student POST marking a lesson done.
func completeLesson(t *testing.T, s *State, courseSlug, lessonSlug string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/"+courseSlug+"/lessons/"+lessonSlug+"/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestGetLesson_GitBackedBody resolves a text lesson's body from a git
// fixture (moduleLessonBody success branch).
func TestGetLesson_GitBackedBody(t *testing.T) {
	t.Parallel()

	repoDir := gitFixture(t, map[string]string{
		"lessons/one.md": "# Lesson One\n\nStudy this.\n",
	})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "git-lessons", Title: "Git Lessons", IsPublic: true,
		Modules: []content.Module{
			{Name: "One", Type: "text", Src: repoDir, Ref: "main", Path: "lessons/one.md"},
		},
	})

	rec := getLesson(t, s, "git-lessons", "one")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lesson struct {
			Content string `json:"content"`
		} `json:"lesson"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Lesson.Content != "# Lesson One\n\nStudy this.\n" {
		t.Errorf("git lesson body = %q", resp.Lesson.Content)
	}
}

// TestGetLesson_GitFetchFailsFallsBack returns the placeholder body when the
// git source is unreachable (moduleLessonBody error branch).
func TestGetLesson_GitFetchFailsFallsBack(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "broken-lessons", Title: "Broken", IsPublic: true,
		Modules: []content.Module{
			{Name: "One", Type: "text", Src: "/nope/repo.git", Ref: "main", Path: "one.md"},
		},
	})

	rec := getLesson(t, s, "broken-lessons", "one")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lesson struct {
			Content string `json:"content"`
		} `json:"lesson"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Lesson.Content == "" || resp.Lesson.Content[:3] != "\xe2\x9a\xa0" {
		t.Errorf("expected the unavailable-content placeholder, got %q", resp.Lesson.Content)
	}
}

// TestGetLesson_VideoBodyIsSrc drives the non-text arm of moduleLessonBody.
func TestGetLesson_VideoBodyIsSrc(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "vid-lessons", Title: "Vid", IsPublic: true,
		Modules: []content.Module{
			{Name: "Watch", Type: "video", Src: "/uploads/watch.mp4"},
		},
	})

	rec := getLesson(t, s, "vid-lessons", "watch")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lesson struct {
			Content string `json:"content"`
			Type    string `json:"type"`
		} `json:"lesson"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Lesson.Type != "video" || resp.Lesson.Content != "/uploads/watch.mp4" {
		t.Errorf("video lesson = %+v", resp.Lesson)
	}
}

// TestMarkLessonComplete_UserServiceUnreachable maps a dead user-service to a
// 500 (postLessonComplete transport-error branch; autoEnroll swallows its
// own).
func TestMarkLessonComplete_UserServiceUnreachable(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, &content.Course{
		Slug: "c1", Title: "C1", IsPublic: true,
		Modules: []content.Module{{Name: "Intro", Type: "text", InlineContent: "hi"}},
	}), "http://127.0.0.1:1", "")

	rec := completeLesson(t, s, "c1", "intro")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMarkLessonComplete_UserServiceRejects maps a non-2xx progress response
// to a 500 (postLessonComplete status-check branch).
func TestMarkLessonComplete_UserServiceRejects(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/complete": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer mock.Close()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, &content.Course{
		Slug: "c2", Title: "C2", IsPublic: true,
		Modules: []content.Module{{Name: "Intro", Type: "text", InlineContent: "hi"}},
	}), mock.URL, "")

	rec := completeLesson(t, s, "c2", "intro")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMarkLessonComplete_CourseCompleteNotified finishes the only lesson of a
// skill-tagged course, so notifyCourseComplete fires (and its SkillTotals +
// non-2xx logging paths run) while the client still gets a 200.
func TestMarkLessonComplete_CourseCompleteNotified(t *testing.T) {
	t.Parallel()

	var sawCourseComplete bool

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-complete": func(w http.ResponseWriter, _ *http.Request) {
			sawCourseComplete = true

			w.WriteHeader(http.StatusBadGateway) // exercises the non-2xx log branch
		},
	})
	defer mock.Close()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, &content.Course{
		Slug: "c3", Title: "C3", IsPublic: true, Difficulty: "beginner",
		Skills:  []string{"linux", "docker"},
		Modules: []content.Module{{Name: "Only", Type: "text", InlineContent: "hi"}},
	}), mock.URL, "")

	rec := completeLesson(t, s, "c3", "only")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !sawCourseComplete {
		t.Error("notifyCourseComplete never called the user-service")
	}
}
