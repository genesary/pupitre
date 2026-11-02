package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// quizCourse returns a public course whose only module is an inline
// single-choice quiz with two questions and a 50% passing score.
func quizCourse() *content.Course {
	yes := true

	return &content.Course{
		Slug: "quiz-course", Title: "Quiz Course", IsPublic: true,
		Modules: []content.Module{
			{
				Name: "Knowledge Check", Type: "quiz", PassingScore: 50,
				Questions: []content.Question{
					{
						ID: "q1", Type: "single", Points: 1, Question: "2 + 2 ?",
						Answers: []content.Answer{
							{ID: "a", Text: "4", Correct: true},
							{ID: "b", Text: "5"},
						},
						Feedback: content.Feedback{Correct: "nice", Wrong: "nope"},
					},
					{
						ID: "q2", Type: "boolean", Points: 1, Question: "Go is compiled?",
						CorrectAnswer: &yes,
					},
				},
			},
		},
	}
}

// submitQuiz issues an authenticated student quiz submission.
func submitQuiz(t *testing.T, s *State, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/quiz-course/modules/0/submit", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestSubmitModule_InlineFullMarks scores a fully-correct submission end to
// end, covering resolveSubmitQuiz (inline), finalizeSubmission,
// acceptSubmission and the scoring/cooldown/progress recording.
func TestSubmitModule_InlineFullMarks(t *testing.T) {
	t.Parallel()

	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, quizCourse())

	rec := submitQuiz(t, s, `{"answers":{"q1":{"single":"a"},"q2":{"boolean":true}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		TotalScore int  `json:"totalScore"`
		MaxScore   int  `json:"maxScore"`
		Passed     bool `json:"passed"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.Passed || resp.TotalScore != 2 || resp.MaxScore != 2 {
		t.Errorf("unexpected score: %+v", resp)
	}
}

// TestSubmitModule_InlineWrongAnswers records a wrong answer, which the handler
// scores as failed and registers a per-question cooldown for.
func TestSubmitModule_InlineWrongAnswers(t *testing.T) {
	t.Parallel()

	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, quizCourse())

	rec := submitQuiz(t, s, `{"answers":{"q1":{"single":"b"},"q2":{"boolean":false}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		TotalScore int  `json:"totalScore"`
		Passed     bool `json:"passed"`
	}

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Passed || resp.TotalScore != 0 {
		t.Errorf("expected a failed submission, got %+v", resp)
	}
}

// TestSubmitModule_MalformedBody rejects a malformed submission body.
func TestSubmitModule_MalformedBody(t *testing.T) {
	t.Parallel()

	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, quizCourse())

	rec := submitQuiz(t, s, "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestSubmitModule_TextModuleRejected rejects a submission against a
// non-quiz module.
func TestSubmitModule_TextModuleRejected(t *testing.T) {
	t.Parallel()

	course := quizCourse()
	course.Modules[0].Type = "text"
	course.Modules[0].InlineContent = "reading"

	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, course)

	rec := submitQuiz(t, s, `{"answers":{}}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestSubmitModule_ResolvedFromGit resolves the quiz from a git fixture before
// scoring, covering the git branch of resolveSubmitQuiz.
func TestSubmitModule_ResolvedFromGit(t *testing.T) {
	t.Parallel()

	quizYAML := "" +
		"id: q\ntitle: Git Quiz\npassingScore: 50\n" +
		"questions:\n" +
		"  - id: g1\n    type: single\n    question: pick a\n    points: 1\n" +
		"    answers:\n      - id: a\n        text: A\n        correct: true\n      - id: b\n        text: B\n"

	repoDir := gitFixture(t, map[string]string{"quiz.yaml": quizYAML})

	course := &content.Course{
		Slug: "quiz-course", Title: "Quiz Course", IsPublic: true,
		Modules: []content.Module{
			{Name: "Git Quiz", Type: "quiz", Src: repoDir, Ref: "main", Path: "quiz.yaml"},
		},
	}
	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, course)

	rec := submitQuiz(t, s, `{"answers":{"g1":{"single":"a"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Passed bool `json:"passed"`
	}

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if !resp.Passed {
		t.Errorf("expected a passing git-quiz submission")
	}
}

// TestSubmitModule_NoCorrectAnswer is the regression test of defect B17
// (docs/specs/190-backend-defects.md): a wrong submission does not reveal
// the correct answers.
func TestSubmitModule_NoCorrectAnswer(t *testing.T) {
	t.Parallel()

	s := newStateWith(&config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}, quizCourse())

	rec := submitQuiz(t, s, `{"answers":{"q1":{"single":"b"},"q2":{"boolean":false}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "correctAnswer") {
		t.Errorf("the submission reveals the correct answers: %s", rec.Body.String())
	}
}
