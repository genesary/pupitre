package definition

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// sampleCourse is a fully-populated wire Course used across the round-trip
// tests.
func sampleCourse() Course {
	return Course{
		Title:       "Linux Intro",
		Description: "From zero to shell",
		Public:      true,
		Hidden:      false,
		Category:    "ops",
		Difficulty:  "beginner",
		Scope:       "public",
		XPRequired:  100,
		InPerson:    true,
		Badge:       &content.Badge{Name: "Penguin", Icon: "🐧"},
		Modules: []content.Module{
			{Name: "Intro", Type: "text", InlineContent: "hello"},
		},
		Prerequisites: []content.CoursePrerequisite{
			{Course: "prep-course", MinScore: 50},
		},
		Sessions: map[string]Session{
			"s2": {Title: "Second", Date: "2026-02-01", Location: "Room B", Capacity: 20},
			"s1": {Title: "First", Date: "2026-01-01", Location: "Room A", Capacity: 10},
		},
	}
}

// TestToCourse_MapsAllFields checks every scalar and slice is carried over.
func TestToCourse_MapsAllFields(t *testing.T) {
	t.Parallel()

	got := sampleCourse().ToCourse("linux-intro")

	if got.Slug != "linux-intro" {
		t.Errorf("Slug = %q", got.Slug)
	}

	if got.Title != "Linux Intro" || got.Description != "From zero to shell" {
		t.Errorf("title/description not mapped: %+v", got)
	}

	if !got.IsPublic || got.Hidden || !got.InPerson {
		t.Errorf("bool flags not mapped: %+v", got)
	}

	if got.XPRequired != 100 || got.Scope != "public" || got.Category != "ops" || got.Difficulty != "beginner" {
		t.Errorf("scalar fields not mapped: %+v", got)
	}

	if got.Badge == nil || got.Badge.Name != "Penguin" {
		t.Errorf("badge not mapped: %+v", got.Badge)
	}

	if len(got.Modules) != 1 || len(got.Prerequisites) != 1 {
		t.Errorf("modules/prereqs not mapped: %+v", got)
	}
}

// TestToCourse_SessionsSortedByID confirms the map is flattened into a
// slice ordered by session ID, so a repeated write is deterministic.
func TestToCourse_SessionsSortedByID(t *testing.T) {
	t.Parallel()

	got := sampleCourse().ToCourse("linux-intro")

	if len(got.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got.Sessions))
	}

	if got.Sessions[0].ID != "s1" || got.Sessions[1].ID != "s2" {
		t.Errorf("sessions not sorted by ID: %+v", got.Sessions)
	}

	if got.Sessions[0].Title != "First" || got.Sessions[1].Location != "Room B" {
		t.Errorf("session fields not mapped: %+v", got.Sessions)
	}
}

// TestFromCourse_IsInverseOfToCourse round-trips a domain course through the
// wire form and back.
func TestFromCourse_IsInverseOfToCourse(t *testing.T) {
	t.Parallel()

	domain := sampleCourse().ToCourse("linux-intro")
	spec := FromCourse(domain)

	if spec.Title != domain.Title || spec.Public != domain.IsPublic || spec.XPRequired != domain.XPRequired {
		t.Errorf("scalar round-trip mismatch: %+v vs %+v", spec, domain)
	}

	if len(spec.Sessions) != 2 {
		t.Fatalf("expected 2 sessions in spec map, got %d", len(spec.Sessions))
	}

	if spec.Sessions["s1"].Title != "First" || spec.Sessions["s2"].Capacity != 20 {
		t.Errorf("session map not rebuilt: %+v", spec.Sessions)
	}

	// A second ToCourse from the rebuilt spec must equal the first.
	again := spec.ToCourse("linux-intro")
	if !reflect.DeepEqual(domain, again) {
		t.Errorf("ToCourse not stable across round-trip:\n%+v\n%+v", domain, again)
	}
}

// TestFromCourse_NoSessions leaves the session map nil when there are none.
func TestFromCourse_NoSessions(t *testing.T) {
	t.Parallel()

	spec := FromCourse(&content.Course{Title: "Bare"})
	if spec.Sessions != nil {
		t.Errorf("expected nil Sessions, got %+v", spec.Sessions)
	}
}

// TestPathRoundTrip converts a Path wire definition to domain and back.
func TestPathRoundTrip(t *testing.T) {
	t.Parallel()

	def := Path{
		Title:       "DevOps",
		Description: "path",
		Kind:        "course",
		Level:       "intermediate",
		Courses:     []string{"a", "b", "c"},
		Skills:      []string{"linux", "docker"},
	}

	domain := def.ToPath("devops-path")
	if domain.Slug != "devops-path" || domain.Level != "intermediate" || len(domain.Courses) != 3 {
		t.Fatalf("ToPath mismatch: %+v", domain)
	}

	back := FromPath(domain)
	if !reflect.DeepEqual(def, back) {
		t.Errorf("FromPath not inverse of ToPath:\n%+v\n%+v", def, back)
	}
}

// TestDecodeYAML_RoutesThroughJSONTags decodes YAML using the json tags.
func TestDecodeYAML_RoutesThroughJSONTags(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte(`
slug: linux-intro
spec:
  title: Linux Intro
  public: true
  xpRequired: 42
  modules:
    - name: Intro
      type: text
`)

	var file File

	err := DecodeYAML(yamlBytes, &file)
	if err != nil {
		t.Fatalf("DecodeYAML: %v", err)
	}

	if file.Slug != "linux-intro" || file.Spec.Title != "Linux Intro" {
		t.Errorf("decoded file mismatch: %+v", file)
	}

	if !file.Spec.Public || file.Spec.XPRequired != 42 {
		t.Errorf("spec scalars not decoded: %+v", file.Spec)
	}

	if len(file.Spec.Modules) != 1 || file.Spec.Modules[0].Type != "text" {
		t.Errorf("modules not decoded: %+v", file.Spec.Modules)
	}
}

// TestDecodeYAML_InvalidYAML reports a parse error.
func TestDecodeYAML_InvalidYAML(t *testing.T) {
	t.Parallel()

	var out map[string]any

	err := DecodeYAML([]byte("::: not yaml :::\n  - broken"), &out)
	if err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
}

// TestDecodeYAML_TypeMismatch reports a decode error when the YAML shape
// does not fit the target.
func TestDecodeYAML_TypeMismatch(t *testing.T) {
	t.Parallel()

	var file File

	err := DecodeYAML([]byte("slug: [not, a, string]\n"), &file)
	if err == nil {
		t.Fatal("expected a decode error for a list where a string is required")
	}
}

// TestEncodeYAML_IsInverseOfDecode round-trips a definition through YAML.
func TestEncodeYAML_IsInverseOfDecode(t *testing.T) {
	t.Parallel()

	original := File{Slug: "linux-intro", Spec: Course{Title: "Linux Intro", Public: true}}

	out, err := EncodeYAML(original)
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}

	var back File

	err = DecodeYAML(out, &back)
	if err != nil {
		t.Fatalf("DecodeYAML: %v", err)
	}

	if !reflect.DeepEqual(original, back) {
		t.Errorf("YAML round-trip mismatch:\n%+v\n%+v", original, back)
	}
}

// TestEncodeYAML_Deterministic produces identical bytes for the same value.
func TestEncodeYAML_Deterministic(t *testing.T) {
	t.Parallel()

	value := sampleCourse()

	first, err := EncodeYAML(value)
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}

	second, err := EncodeYAML(value)
	if err != nil {
		t.Fatalf("EncodeYAML: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("EncodeYAML not deterministic:\n%s\n---\n%s", first, second)
	}
}

// TestEncodeYAML_UnmarshalableValue reports an error for a value json
// cannot marshal.
func TestEncodeYAML_UnmarshalableValue(t *testing.T) {
	t.Parallel()

	_, err := EncodeYAML(make(chan int))
	if err == nil {
		t.Fatal("expected an error encoding an unmarshalable value")
	}
}

// TestParseCourseFile_OK parses a valid course file.
func TestParseCourseFile_OK(t *testing.T) {
	t.Parallel()

	file, err := ParseCourseFile([]byte("slug: linux-intro\nspec:\n  title: Linux Intro\n"))
	if err != nil {
		t.Fatalf("ParseCourseFile: %v", err)
	}

	if file.Slug != "linux-intro" || file.Spec.Title != "Linux Intro" {
		t.Errorf("unexpected parse result: %+v", file)
	}
}

// TestParseCourseFile_MissingSlug rejects a file that names no slug.
func TestParseCourseFile_MissingSlug(t *testing.T) {
	t.Parallel()

	_, err := ParseCourseFile([]byte("spec:\n  title: No Slug\n"))
	if err == nil {
		t.Fatal("expected an error for a slugless file")
	}
}

// TestParseCourseFile_InvalidYAML propagates the decode failure.
func TestParseCourseFile_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := ParseCourseFile([]byte("slug: [oops]\n"))
	if err == nil {
		t.Fatal("expected an error for malformed course file")
	}
}
