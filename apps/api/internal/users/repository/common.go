// Package repository defines data-access interfaces for user-service's
// persisted entities, decoupling handlers from the underlying storage
// technology (GORM/Postgres in production, in-memory fakes in tests).
package repository

import "gorm.io/gorm/clause"

// returningAll requests the full updated row back from Postgres (RETURNING
// *) so an Updates() call against a zero-value struct populates every
// column, not just the ones that were written.
var returningAll = clause.Returning{} //nolint:gochecknoglobals // stateless clause value, reused as a query modifier

// Column names shared by ON CONFLICT clauses and Updates() maps across
// multiple repository files.
const (
	colName        = "name"
	colEmail       = "email"
	colUsername    = "username"
	colUpdatedAt   = "updatedat"
	colCourseSlug  = "courseslug"
	colUserID      = "userid"
	colCreatedAt   = "createdat"
	colMaxScore    = "maxscore"
	colCompletedAt = "completed_at"
	colLessonSlug  = "lessonslug"
	colPassed      = "passed"
	colSource      = "source"
)

// Platform role values, shared by group-role-mapping derivation and the
// admin/mock-student seeding helpers.
const (
	roleAdmin   = "admin"
	roleStudent = "student"
)
