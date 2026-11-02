package db

import (
	"io/fs"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
)

// TestDevCoursesParse verifies every embedded seed file is a valid course
// definition. It runs without a database, so a malformed seed file fails
// the normal test run rather than only surfacing at container startup.
func TestDevCoursesParse(t *testing.T) {
	t.Parallel()

	entries, err := fs.ReadDir(devCourses, devCourseDir)
	if err != nil {
		t.Fatalf("read embedded seed dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no dev seed courses are embedded")
	}

	seen := make(map[string]string, len(entries))

	for _, entry := range entries {
		data, readErr := devCourses.ReadFile(devCourseDir + "/" + entry.Name())
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}

		file, parseErr := definition.ParseCourseFile(data)
		if parseErr != nil {
			t.Errorf("%s: %v", entry.Name(), parseErr)

			continue
		}

		if previous, dup := seen[file.Slug]; dup {
			t.Errorf("%s and %s both declare slug %q", previous, entry.Name(), file.Slug)
		}

		seen[file.Slug] = entry.Name()

		course := file.Spec.ToCourse(file.Slug)
		if course.Title == "" {
			t.Errorf("%s: course has no title", entry.Name())
		}

		if len(course.Modules) == 0 {
			t.Errorf("%s: course has no modules", entry.Name())
		}

		for index, module := range course.Modules {
			if module.Name == "" {
				t.Errorf("%s: module %d has no name", entry.Name(), index)
			}
		}
	}
}

// TestSeedDevCourses_DisabledByDefault verifies an unset or unrecognised
// mode seeds nothing, so a production deployment cannot pick up demo
// content by accident.
func TestSeedDevCourses_DisabledByDefault(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"", "false", "yes", "1"} {
		repo := &countingCourseRepository{}

		err := SeedDevCourses(t.Context(), repo, mode)
		if err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}

		if repo.creates != 0 || repo.upserts != 0 {
			t.Errorf("mode %q seeded %d creates / %d upserts, want none",
				mode, repo.creates, repo.upserts)
		}
	}
}

// TestDevCoursesPrerequisites checks that every module prerequisite of the
// demo catalogue names a module of its course. A wrong slug locks the module
// for good: python-basics' quiz did (docs/specs/190-backend-defects.md B23).
func TestDevCoursesPrerequisites(t *testing.T) {
	t.Parallel()

	entries, err := fs.ReadDir(devCourses, devCourseDir)
	if err != nil {
		t.Fatalf("read embedded seed dir: %v", err)
	}

	for _, entry := range entries {
		data, readErr := devCourses.ReadFile(devCourseDir + "/" + entry.Name())
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}

		file, parseErr := definition.ParseCourseFile(data)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}

		course := file.Spec.ToCourse(file.Slug)
		slugs := make(map[string]bool, len(course.Modules))

		for _, module := range course.Modules {
			slugs[module.Slug()] = true
		}

		for _, module := range course.Modules {
			for _, prerequisite := range module.Prerequisites {
				if !slugs[prerequisite] {
					t.Errorf("%s: module %q requires %q, which is not a module of the course", entry.Name(), module.Name, prerequisite)
				}
			}
		}
	}
}
