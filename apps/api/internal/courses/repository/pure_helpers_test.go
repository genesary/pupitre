package repository

import (
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// TestTextArrayRoundTrip covers the nil/empty/populated mapping in both
// directions.
func TestTextArrayRoundTrip(t *testing.T) {
	t.Parallel()

	if got := textArray(nil); got == nil || len(got) != 0 {
		t.Errorf("textArray(nil) = %#v, want empty non-nil", got)
	}

	if got := textArray([]string{"a", "b"}); !reflect.DeepEqual([]string(got), []string{"a", "b"}) {
		t.Errorf("textArray = %#v", got)
	}

	if got := fromTextArray(pq.StringArray{}); got != nil {
		t.Errorf("fromTextArray(empty) = %#v, want nil", got)
	}

	if got := fromTextArray(pq.StringArray{"x"}); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("fromTextArray = %#v", got)
	}
}

// TestPrerequisiteFromModel maps every column onto the domain type.
func TestPrerequisiteFromModel(t *testing.T) {
	t.Parallel()

	got := prerequisiteFromModel(models.CoursePrerequisite{
		CourseSlug:     "k8s",
		RequiredCourse: "linux-intro",
		MinScore:       40,
		Modules:        pq.StringArray{"quiz-bases"},
	})

	if got.Course != "linux-intro" || got.MinScore != 40 {
		t.Errorf("scalars not mapped: %+v", got)
	}

	if !reflect.DeepEqual(got.Modules, []string{"quiz-bases"}) {
		t.Errorf("Modules = %#v", got.Modules)
	}

	// Empty modules array becomes nil.
	empty := prerequisiteFromModel(models.CoursePrerequisite{RequiredCourse: "c"})
	if empty.Modules != nil {
		t.Errorf("expected nil Modules, got %#v", empty.Modules)
	}
}

// TestSessionFromModel maps every column onto the domain type.
func TestSessionFromModel(t *testing.T) {
	t.Parallel()

	got := sessionFromModel(models.CourseSession{
		SessionID: "sess-1",
		Title:     "Cohort 1",
		Date:      "2026-09-01",
		Location:  "Room A",
		Capacity:  12,
	})

	want := struct {
		id, title, date, loc string
		cap                  int
	}{"sess-1", "Cohort 1", "2026-09-01", "Room A", 12}

	if got.ID != want.id || got.Title != want.title || got.Date != want.date ||
		got.Location != want.loc || got.Capacity != want.cap {
		t.Errorf("sessionFromModel = %+v", got)
	}
}

// TestPathFromModel fills in defaults for a blank title and kind.
func TestPathFromModel(t *testing.T) {
	t.Parallel()

	got := pathFromModel(&models.Path{Slug: "devops-path"})
	if got.Title != "devops-path" {
		t.Errorf("blank title should default to slug, got %q", got.Title)
	}

	if got.Kind != pathKindCourse {
		t.Errorf("blank kind should default to %q, got %q", pathKindCourse, got.Kind)
	}

	full := pathFromModel(&models.Path{
		Slug: "s", Title: "Real Title", Description: "d", Kind: "skill", Level: "advanced",
	})
	if full.Title != "Real Title" || full.Kind != "skill" || full.Level != "advanced" || full.Description != "d" {
		t.Errorf("pathFromModel = %+v", full)
	}
}

// TestRemainingUntil clamps negatives to zero and passes a nil deadline
// through as zero.
func TestRemainingUntil(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)

	if got := remainingUntil(nil, now); got != 0 {
		t.Errorf("nil deadline = %v, want 0", got)
	}

	past := now.Add(-time.Hour)
	if got := remainingUntil(&past, now); got != 0 {
		t.Errorf("past deadline = %v, want 0", got)
	}

	future := now.Add(90 * time.Second)
	if got := remainingUntil(&future, now); got != 90*time.Second {
		t.Errorf("future deadline = %v, want 90s", got)
	}
}

// TestDedupe drops blanks and later duplicates while keeping order.
func TestDedupe(t *testing.T) {
	t.Parallel()

	got := dedupe([]string{"a", "", "b", "a", "c", "b", ""})
	want := []string{"a", "b", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupe = %#v, want %#v", got, want)
	}

	if got := dedupe(nil); len(got) != 0 {
		t.Errorf("dedupe(nil) = %#v, want empty", got)
	}
}
