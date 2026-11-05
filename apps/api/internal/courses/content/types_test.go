package content

import (
	"reflect"
	"testing"
)

// TestSlugifyModuleName covers letters, digits, separators, unicode and
// trimming.
func TestSlugifyModuleName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"Hello World":      "hello-world",
		"  spaced  out  ":  "spaced--out",
		"keep-dash_and_us": "keep-dash_and_us",
		"Trim!!!":          "trim",
		"Accénté":          "accénté", // unicode letters are kept, just lowercased
		"a b":              "a-b",
		"123 Numbers 456":  "123-numbers-456",
		"---already---":    "already",
		"":                 "",
		"!!!":              "",
		"CamelCaseModule":  "camelcasemodule",
	}

	for in, want := range cases {
		if got := SlugifyModuleName(in); got != want {
			t.Errorf("SlugifyModuleName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestModuleSlug delegates to SlugifyModuleName.
func TestModuleSlug(t *testing.T) {
	t.Parallel()

	m := Module{Name: "My First Module"}
	if got := m.Slug(); got != "my-first-module" {
		t.Errorf("Slug() = %q", got)
	}
}

// TestModuleContent covers each module type branch.
func TestModuleContent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		module Module
		want   string
	}{
		{"video returns src", Module{Type: "video", Src: "https://v/1"}, "https://v/1"},
		{"image returns src", Module{Type: "image", Src: "https://i/1"}, "https://i/1"},
		{"lab with inline", Module{Type: "lab", InlineContent: "do this", Src: "https://l/1"}, "do this"},
		{"lab without inline", Module{Type: "lab", Src: "https://l/1"}, "https://l/1"},
		{"text returns inline", Module{Type: "text", InlineContent: "body"}, "body"},
		{"quiz returns inline", Module{Type: "quiz", InlineContent: "q"}, "q"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.module.Content(); got != tc.want {
				t.Errorf("Content() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestModuleHasQuestions is true only for quizzes with inline questions.
func TestModuleHasQuestions(t *testing.T) {
	t.Parallel()

	if (Module{Type: "quiz"}).HasQuestions() {
		t.Error("quiz with no questions should report false")
	}

	if (Module{Type: "text", Questions: []Question{{}}}).HasQuestions() {
		t.Error("non-quiz should report false even with questions")
	}

	if !(Module{Type: "quiz", Questions: []Question{{}}}).HasQuestions() {
		t.Error("quiz with questions should report true")
	}
}

// TestModuleHasGitContent needs src, ref and path all set.
func TestModuleHasGitContent(t *testing.T) {
	t.Parallel()

	full := Module{Src: "repo", Ref: "main", Path: "docs/x.md"}
	if !full.HasGitContent() {
		t.Error("fully-specified git module should report true")
	}

	for _, m := range []Module{
		{Ref: "main", Path: "p"},
		{Src: "repo", Path: "p"},
		{Src: "repo", Ref: "main"},
		{},
	} {
		if m.HasGitContent() {
			t.Errorf("incomplete git module %+v should report false", m)
		}
	}
}

// TestAggregateSkills dedups, drops blanks and sorts.
func TestAggregateSkills(t *testing.T) {
	t.Parallel()

	if got := AggregateSkills(nil); got != nil {
		t.Errorf("nil modules = %v, want nil", got)
	}

	if got := AggregateSkills([]Module{{Skills: []string{"", ""}}}); got != nil {
		t.Errorf("blank-only skills = %v, want nil", got)
	}

	got := AggregateSkills([]Module{
		{Skills: []string{"linux", "docker"}},
		{Skills: []string{"docker", "", "ansible"}},
	})
	want := []string{"ansible", "docker", "linux"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AggregateSkills = %v, want %v", got, want)
	}
}
