package repository

import (
	"slices"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// richCourse is a course exercising every field that has to survive a
// write/read cycle: scalars, arrays, the jsonb payloads, and children.
func richCourse() *content.Course {
	return &content.Course{
		Slug:        "kubernetes-basics",
		Title:       "Kubernetes Basics",
		Description: "Intro to K8s",
		Category:    "kubernetes",
		Difficulty:  "beginner",
		IsPublic:    true,
		Hidden:      false,
		Scope:       "team-a",
		XPRequired:  120,
		InPerson:    true,
		Badge:       &content.Badge{Name: "Operator", Icon: "🚀"},
		Prerequisites: []content.CoursePrerequisite{
			{Course: "linux-intro", MinScore: 30, Modules: []string{"quiz-bases"}},
			{Course: ""}, // dropped: names no course
		},
		Sessions: []content.Session{
			{ID: "sess-1", Title: "Cohort 1", Date: "2026-09-01T10:00:00Z", Location: "Room A", Capacity: 12},
			{ID: ""}, // dropped: no ID
		},
		Modules: []content.Module{
			{
				Name:          "What is K8s",
				Type:          "text",
				Src:           "https://github.com/org/repo",
				Ref:           "main",
				Path:          "lessons/intro.md",
				Prerequisites: []string{"welcome"},
				Skills:        []string{"kubernetes"},
			},
			{
				Name:                   "Quiz",
				Type:                   "quiz",
				PassingScore:           80,
				MaxAttemptsPerQuestion: new(3),
				LockOnMaxAttempts:      true,
				Cooldown:               content.CooldownSpec{Strategy: "linear", BaseSeconds: 15},
				Questions: []content.Question{
					{
						Type:     "single",
						Question: "Q?",
						Answers:  []content.Answer{{ID: "a", Text: "A", Correct: true}},
					},
				},
				Skills: []string{"kubernetes", "ops"},
			},
			{
				Name:          "Lab",
				Type:          "lab",
				CheckProvider: "gitlab",
				CheckType:     "mr-open",
				CheckParams:   map[string]any{"project": "e-learning/{{ .Username }}"},
				Steps: []content.CheckStep{
					{Title: "Open an MR", CheckType: "mr-open", CheckParams: map[string]any{"target": "main"}},
				},
			},
		},
	}
}

// TestCourseToModel_RoundTrip verifies that a course survives conversion to
// its persisted rows and back without losing or mangling any field.
func TestCourseToModel_RoundTrip(t *testing.T) {
	t.Parallel()

	original := richCourse()

	row, children, err := courseToModel(original)
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	got := courseFromModel(row)

	for i := range children.modules {
		module, convErr := moduleFromModel(&children.modules[i])
		if convErr != nil {
			t.Fatalf("moduleFromModel[%d]: %v", i, convErr)
		}

		got.Modules = append(got.Modules, module)
	}

	if got.Slug != original.Slug || got.Title != original.Title ||
		got.Category != original.Category || got.Difficulty != original.Difficulty {
		t.Errorf("scalar fields not preserved: %+v", got)
	}

	if !got.IsPublic || !got.InPerson || got.Scope != "team-a" || got.XPRequired != 120 {
		t.Errorf("flags not preserved: public=%v inPerson=%v scope=%q xp=%d",
			got.IsPublic, got.InPerson, got.Scope, got.XPRequired)
	}

	if got.Badge == nil || got.Badge.Name != "Operator" || got.Badge.Icon != "🚀" {
		t.Errorf("badge not preserved: %+v", got.Badge)
	}

	if len(got.Modules) != 3 {
		t.Fatalf("want 3 modules, got %d", len(got.Modules))
	}

	quiz := got.Modules[1]
	if quiz.PassingScore != 80 || !quiz.LockOnMaxAttempts ||
		quiz.MaxAttemptsPerQuestion == nil || *quiz.MaxAttemptsPerQuestion != 3 {
		t.Errorf("quiz config not preserved: %+v", quiz)
	}

	if len(quiz.Questions) != 1 || quiz.Questions[0].Answers[0].ID != "a" {
		t.Errorf("quiz questions not preserved: %+v", quiz.Questions)
	}

	lab := got.Modules[2]
	if lab.CheckParams["project"] != "e-learning/{{ .Username }}" {
		t.Errorf("check params not preserved: %+v", lab.CheckParams)
	}

	if len(lab.Steps) != 1 || lab.Steps[0].CheckParams["target"] != "main" {
		t.Errorf("check steps not preserved: %+v", lab.Steps)
	}
}

// TestCourseToModel_Defaults verifies the defaults that used to be applied
// while converting a custom resource are applied on write instead.
func TestCourseToModel_Defaults(t *testing.T) {
	t.Parallel()

	row, children, err := courseToModel(&content.Course{
		Slug: "bare",
		Modules: []content.Module{
			{}, // no name, no type
			{Name: "Quiz", Type: "quiz", Cooldown: content.CooldownSpec{}},
			{Name: "Quiz 2", Type: "quiz", Cooldown: content.CooldownSpec{MaxSeconds: 600}},
		},
	})
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	if row.Title != "bare" {
		t.Errorf("title should default to the slug, got %q", row.Title)
	}

	first := children.modules[0]
	if first.Name != "module-1" {
		t.Errorf("name should default to module-<position>, got %q", first.Name)
	}

	if first.Type != content.ModuleTypeText {
		t.Errorf("type should default to text, got %q", first.Type)
	}

	// A module that configures no cooldown at all keeps zeroed columns...
	if children.modules[1].CooldownStrategy != "" || children.modules[1].CooldownBaseSeconds != 0 {
		t.Errorf("unset cooldown should stay zeroed, got %+v", children.modules[1])
	}

	// ...but one that configures any part of it gets the rest filled in.
	third := children.modules[2]
	if third.CooldownStrategy != content.CooldownStrategyExponential {
		t.Errorf("strategy should default to exponential, got %q", third.CooldownStrategy)
	}

	if third.CooldownBaseSeconds != cooldownDefaultBaseSeconds || third.CooldownMultiplier != cooldownDefaultMultiplier {
		t.Errorf("cooldown defaults not applied: %+v", third)
	}
}

// TestCourseToModel_QuestionDefaults verifies that per-question IDs,
// difficulty and points are resolved once on write rather than on every read.
func TestCourseToModel_QuestionDefaults(t *testing.T) {
	t.Parallel()

	_, children, err := courseToModel(&content.Course{
		Slug: "quizzes",
		Modules: []content.Module{{
			Name: "Quiz",
			Type: "quiz",
			Questions: []content.Question{
				{Type: "single", Question: "First?"},
				{ID: "kept", Type: "boolean", Question: "Second?", Difficulty: "hard", Points: 5},
			},
		}},
	})
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	module, err := moduleFromModel(&children.modules[0])
	if err != nil {
		t.Fatalf("moduleFromModel: %v", err)
	}

	first := module.Questions[0]
	if first.ID != "q-1" || first.Difficulty != content.DifficultyMedium || first.Points != 1 {
		t.Errorf("question defaults not applied: %+v", first)
	}

	second := module.Questions[1]
	if second.ID != "kept" || second.Difficulty != "hard" || second.Points != 5 {
		t.Errorf("explicit question values overwritten: %+v", second)
	}
}

// TestCourseToModel_SkillsAggregated verifies the denormalized course skill
// list is the deduplicated union of its modules' skills.
func TestCourseToModel_SkillsAggregated(t *testing.T) {
	t.Parallel()

	row, _, err := courseToModel(&content.Course{
		Slug: "skilled",
		Modules: []content.Module{
			{Name: "A", Skills: []string{"docker", "linux"}},
			{Name: "B", Skills: []string{"linux", ""}},
		},
	})
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	want := []string{"docker", "linux"}
	if !slices.Equal([]string(row.Skills), want) {
		t.Errorf("want skills %v, got %v", want, row.Skills)
	}
}

// TestCourseToModel_ModulePositions verifies modules are numbered by display
// order and given a persisted slug derived from their name.
func TestCourseToModel_ModulePositions(t *testing.T) {
	t.Parallel()

	_, children, err := courseToModel(&content.Course{
		Slug: "ordered",
		Modules: []content.Module{
			{Name: "What is K8s"},
			{Name: "Deep Dive"},
		},
	})
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	for i, module := range children.modules {
		if module.Position != i {
			t.Errorf("module %d has position %d", i, module.Position)
		}

		if module.CourseSlug != "ordered" {
			t.Errorf("module %d has course slug %q", i, module.CourseSlug)
		}
	}

	if children.modules[0].Slug != "what-is-k8s" {
		t.Errorf("want slug what-is-k8s, got %q", children.modules[0].Slug)
	}
}

// TestCourseToModel_DropsIncompleteChildren verifies that prerequisites
// naming no course and sessions without an ID are not persisted.
func TestCourseToModel_DropsIncompleteChildren(t *testing.T) {
	t.Parallel()

	_, children, err := courseToModel(richCourse())
	if err != nil {
		t.Fatalf("courseToModel: %v", err)
	}

	if len(children.prerequisites) != 1 || children.prerequisites[0].RequiredCourse != "linux-intro" {
		t.Errorf("want the one named prerequisite, got %+v", children.prerequisites)
	}

	if len(children.sessions) != 1 || children.sessions[0].SessionID != "sess-1" {
		t.Errorf("want the one identified session, got %+v", children.sessions)
	}
}

// TestPathToModel_RoundTrip verifies a path's ordered members survive
// conversion, with empty and duplicate entries dropped.
func TestPathToModel_RoundTrip(t *testing.T) {
	t.Parallel()

	row, courses, skills := pathToModel(&content.Path{
		Slug:    "devops",
		Courses: []string{"linux-intro", "", "docker"},
		Skills:  []string{"linux", "linux", "docker", ""},
	})

	if row.Title != "devops" {
		t.Errorf("title should default to the slug, got %q", row.Title)
	}

	if row.Kind != pathKindCourse {
		t.Errorf("kind should default to course, got %q", row.Kind)
	}

	if len(courses) != 2 || courses[0].CourseSlug != "linux-intro" || courses[0].Position != 0 ||
		courses[1].CourseSlug != "docker" || courses[1].Position != 1 {
		t.Errorf("course members not numbered contiguously: %+v", courses)
	}

	if len(skills) != 2 || skills[0].Skill != "linux" || skills[1].Skill != "docker" {
		t.Errorf("skill members not deduplicated in order: %+v", skills)
	}
}
