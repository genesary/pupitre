package handlers

import (
	"testing"

	"github.com/genesary/pupitre/apps/api/internal/users/repository"
)

// TestBuildMyCoursesLeavesOutDeletedCourses checks that an enrollment in a
// deleted course is not listed, while a failed catalog lookup keeps it.
func TestBuildMyCoursesLeavesOutDeletedCourses(t *testing.T) {
	t.Parallel()

	enrolled := []repository.EnrollmentRow{{Slug: "kept", CompletedLabs: 1}, {Slug: "deleted"}}
	details := map[string]CourseInfo{"kept": {ID: "kept", Title: "Kept course", LabCount: 3}}

	courses := buildMyCourses(enrolled, map[string]struct{}{}, details, true)

	if len(courses) != 1 {
		t.Fatalf("got %d courses, want 1: %+v", len(courses), courses)
	}

	if courses[0].Slug != "kept" || courses[0].Title != "Kept course" || courses[0].LabCount != 3 || courses[0].CompletedLabs != 1 {
		t.Errorf("unexpected course: %+v", courses[0])
	}

	// The lookup failed: the course may exist, so it stays, titled with its slug.
	courses = buildMyCourses(enrolled, map[string]struct{}{}, details, false)

	if len(courses) != 2 || courses[1].Title != "deleted" {
		t.Errorf("want both courses after a failed lookup, got %+v", courses)
	}
}
