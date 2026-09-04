package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"go.uber.org/zap"

	"github.com/genesary/pupitre/course-service/internal/content"
	"github.com/genesary/pupitre/course-service/internal/definition"
	"github.com/genesary/pupitre/course-service/internal/repository"
)

// devCourseDir is the embedded directory holding the dev seed courses.
const devCourseDir = "seed/courses"

// devCourses holds the demo course catalogue used to browse and exercise
// the app in a dev environment. It is embedded in the binary rather than
// mounted, so seeding works the same in a KinD cluster and on a laptop.
//
//go:embed seed/courses/*.yaml
var devCourses embed.FS

// Dev-seed modes, selected by the SEED_DEV_COURSES environment variable.
const (
	// SeedDevCoursesMissing creates the courses that do not exist yet and
	// leaves existing ones untouched, so local edits survive a restart.
	SeedDevCoursesMissing = "true"
	// SeedDevCoursesOverwrite replaces every seed course on each startup,
	// which is what you want after editing the seed files themselves.
	SeedDevCoursesOverwrite = "overwrite"
)

// SeedDevCourses loads the embedded demo catalogue into the database.
//
// mode selects the behaviour for courses that already exist (see the
// SeedDevCourses* constants); any other value disables seeding entirely.
// It is safe to run on every startup and never touches a course that is
// not part of the seed set.
func SeedDevCourses(ctx context.Context, courses repository.CourseRepository, mode string) error {
	if mode != SeedDevCoursesMissing && mode != SeedDevCoursesOverwrite {
		return nil
	}

	entries, err := fs.ReadDir(devCourses, devCourseDir)
	if err != nil {
		return fmt.Errorf("read embedded dev courses: %w", err)
	}

	// Directory order is not guaranteed; sort so the log reads the same
	// way on every run.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	created, skipped, replaced := 0, 0, 0

	for _, entry := range entries {
		outcome, seedErr := seedOneDevCourse(ctx, courses, entry.Name(), mode)
		if seedErr != nil {
			return seedErr
		}

		switch outcome {
		case seedCreated:
			created++
		case seedReplaced:
			replaced++
		case seedSkipped:
			skipped++
		}
	}

	zap.L().Info("dev courses seeded",
		zap.String("mode", mode),
		zap.Int("created", created),
		zap.Int("replaced", replaced),
		zap.Int("skippedExisting", skipped))

	return nil
}

// seedOutcome reports what SeedDevCourses did with one seed file.
type seedOutcome int

const (
	// seedCreated means the course did not exist and was inserted.
	seedCreated seedOutcome = iota
	// seedReplaced means an existing course was overwritten by the seed.
	seedReplaced
	// seedSkipped means the course was left as it was.
	seedSkipped
)

// seedOneDevCourse parses one embedded seed file and writes it, honouring
// mode for a course that already exists.
func seedOneDevCourse(
	ctx context.Context, courses repository.CourseRepository, name, mode string,
) (seedOutcome, error) {
	data, err := devCourses.ReadFile(path.Join(devCourseDir, name))
	if err != nil {
		return seedSkipped, fmt.Errorf("read dev course %s: %w", name, err)
	}

	file, err := definition.ParseCourseFile(data)
	if err != nil {
		return seedSkipped, fmt.Errorf("dev course %s: %w", name, err)
	}

	course := file.Spec.ToCourse(file.Slug)

	if mode == SeedDevCoursesOverwrite {
		err = courses.Upsert(ctx, course)
		if err != nil {
			return seedSkipped, fmt.Errorf("seed dev course %s: %w", file.Slug, err)
		}

		return seedReplaced, nil
	}

	err = courses.Create(ctx, course)

	switch {
	case errors.Is(err, repository.ErrConflict):
		return seedSkipped, nil
	case err != nil:
		return seedSkipped, fmt.Errorf("seed dev course %s: %w", file.Slug, err)
	default:
		return seedCreated, nil
	}
}

const (
	// devPathKind is the path kind used for all dev seed learning paths.
	devPathKind = "course"
	// devPathLevel is the default level for dev seed learning paths.
	devPathLevel = "beginner"
)

// seedPaths returns the learning paths to seed in dev/staging. Returned as a
// function (not a package-level var) to avoid gochecknoglobals.
func seedPaths() []content.Path {
	return []content.Path{
		{
			Slug:        "parcours-devops",
			Title:       "Parcours DevOps",
			Description: "Maîtrise les outils et pratiques DevOps modernes, de la conteneurisation à l'orchestration en passant par l'intégration continue.",
			Kind:        devPathKind,
			Level:       devPathLevel,
			Courses: []string{
				"docker-fundamentals",
				"kubernetes-basics",
				"gitlab-cicd",
				"infrastructure-as-code",
				"monitoring-observability",
			},
		},
		{
			Slug:        "parcours-securite",
			Title:       "Parcours Sécurité",
			Description: "Développe tes compétences en cybersécurité, des fondamentaux jusqu'aux techniques offensives et défensives.",
			Kind:        devPathKind,
			Level:       devPathLevel,
			Courses: []string{
				"cybersecurity-intro",
				"secrets-management",
				"container-security",
				"devsecops",
				"pentesting-basics",
			},
		},
	}
}

// SeedDevPaths upserts the dev learning paths. It runs whenever
// SEED_DEV_COURSES is set, immediately after course seeding.
func SeedDevPaths(ctx context.Context, paths repository.PathRepository, mode string) error {
	if mode != SeedDevCoursesMissing && mode != SeedDevCoursesOverwrite {
		return nil
	}

	devPaths := seedPaths()

	for i := range devPaths {
		err := paths.Upsert(ctx, &devPaths[i])
		if err != nil {
			return fmt.Errorf("seed dev path %s: %w", devPaths[i].Slug, err)
		}
	}

	zap.L().Info("dev paths seeded", zap.Int("count", len(devPaths)))

	return nil
}
