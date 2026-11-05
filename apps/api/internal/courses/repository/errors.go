// Package repository defines data-access interfaces for course-service's
// persisted entities, decoupling handlers from the underlying storage
// technology (GORM/Postgres in production, in-memory fakes in tests).
package repository

import (
	"errors"

	"github.com/lib/pq"
)

// ErrNotFound is returned by reads whose subject does not exist. Handlers
// map it to 404 and treat every other error as a 500, so a database
// outage is no longer indistinguishable from a missing course.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write would duplicate an existing
// primary key, e.g. creating a course whose slug is already taken.
var ErrConflict = errors.New("already exists")

// Column names shared by more than one query, pulled out so the ON CONFLICT
// clauses below cannot drift from the schema by a typo.
const (
	colSlug        = "slug"
	colTitle       = "title"
	colDescription = "description"
	colCourseSlug  = "course_slug"
	colSessionID   = "session_id"
	colDate        = "date"
	colLocation    = "location"
	colCapacity    = "capacity"
	colUpdatedAt   = "updated_at"
)

// textArray converts a Go string slice to a Postgres text[] value,
// mapping nil to an empty array so it satisfies the NOT NULL constraint
// the schema puts on every array column.
func textArray(values []string) pq.StringArray {
	if values == nil {
		return pq.StringArray{}
	}

	return values
}

// fromTextArray converts a Postgres text[] value back to a Go string
// slice, mapping the empty array to nil so JSON responses omit it.
func fromTextArray(values pq.StringArray) []string {
	if len(values) == 0 {
		return nil
	}

	return values
}
