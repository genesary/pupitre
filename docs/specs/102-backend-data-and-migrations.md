# 102 - Backend: data and migrations

The Helm chart of `genesary/pupitre` already points user-service and course-service at the **same**
PostgreSQL database. The monolith keeps that database, its tables and its data as they are: the
move needs no data migration.

## Tables

Each domain keeps ownership of its tables. A domain reads or writes another domain's tables only
through the interfaces of [104](104-backend-in-process-calls.md), never with its own SQL, exactly
as the services could not reach each other's tables before.

| Domain | Table | Model | Content |
| --- | --- | --- | --- |
| users | `users` | `User` | Accounts: login, email, role, auth provider, profile, hashed password |
| users | `platform_settings` | `PlatformSetting` | Key-value settings (sign-in methods, `oidc_*`, display) |
| users | `enrollments` | `Enrollment` | Learner enrolled in a course |
| users | `lesson_progress` | `LessonProgress` | Course-level progress and completion |
| users | `module_progress` | `ModuleProgress` | Module completion, quiz scores and attempts |
| users | `groups` | `Group` | Groups, nested through `parent_id` |
| users | `user_groups` | `UserGroup` | Group membership and role in the group |
| users | `group_role_mappings` | `GroupRoleMapping` | IdP group name to pupitre group and role |
| users | `group_enrollments` | `GroupEnrollment` | A group enrolled in a course or path |
| users | `markdown_patterns` | `MarkdownPattern` | Reusable markdown snippets of the editor |
| users | `path_enrollments` | `PathEnrollment` | Learner enrolled in a path |
| users | `user_badges` | `UserBadge` | Badges earned |
| users | `user_skill_levels` | `UserSkillLevel` | Skill level reached |
| users | `session_bookings` | `SessionBooking` | Learner booked on a course session; attendance |
| users | `user_xp_events` | `UserXPEvent` | XP ledger, unique on (`userid`, `source`, `source_slug`) |
| users | `export_logs` | `ExportLog` | Audit trail of CSV exports |
| courses | `courses` | `Course` | Courses, their git source and settings |
| courses | `course_modules` | `CourseModule` | Modules of a course (lesson, quiz, lab…) |
| courses | `course_prerequisites` | `CoursePrerequisite` | Course required before another |
| courses | `course_sessions` | `CourseSession` | Dated classroom or remote sessions |
| courses | `paths` | `Path` | Learning paths |
| courses | `path_courses` | `PathCourse` | Ordered courses of a path |
| courses | `path_skills` | `PathSkill` | Skills a path grants |
| courses | `quiz_question_attempts` | `QuizQuestionAttempt` | Per-question quiz answers |
| courses | `lab_checks` | `LabCheck` | Results of lab checks |
| platform | `_schema_migrations` | - | Names of the breaking migrations already applied |

No two models share a table name, so the two schemas join without a rename.

Foreign keys stay where they are: `users` and `groups` are referenced with `ON DELETE CASCADE` by
the user-domain tables (`ON DELETE SET NULL` for `markdown_patterns.createdby`, `ON DELETE
RESTRICT` for `groups.parent_id`). No foreign key crosses domains: a course slug or id stored in
a user-domain table (`enrollments`, `module_progress`, `group_enrollments`…) is a plain column, and
it stays one. Adding such a key is not part of the move.

## Migrations

Both services migrated the shared database at start-up with the same three steps: **breaking
migrations** (each tracked by name in `_schema_migrations` and run in its own transaction),
**AutoMigrate** (it only adds tables, columns and indexes), then **foreign keys** added if missing
(checked by name in `pg_constraint`). The monolith runs each domain's migration runner, unchanged,
once at start-up: the users domain's, then the courses domain's (`server.Migrate`). Since no table,
breaking migration or constraint of one domain depends on the other, the order gives the same
schema as the two services starting in any order.

The move kept the one breaking migration of user-service, `20260829_dedupe_user_xp_events`.
Names are kept as they are, so a database that already ran it does not run it again. The fixes
added `20261026_group_role_mappings_allow_manager` ([190](190-backend-defects.md) B20): it drops
the check constraint of `group_role_mappings.platformrole`, which AutoMigrate does not change, and
AutoMigrate creates it again with `admin`, `manager` and `student`.
`_schema_migrations` was already shared by the two services; nothing changes for it.

Each domain keeps its own list of models and of breaking migrations in its `db` package. A new
breaking migration gets a dated name, unique across domains.

Running the migrations on a database that the two services already migrated changes nothing:
this is checked by a test against a database migrated by the `3cdf7b2` code
([108](108-backend-operations-and-testing.md)).

## Seeding at start-up

| Step | Condition | Origin | Effect |
| --- | --- | --- | --- |
| Bootstrap admin | Always | user-service `SeedAdmin` | Creates or updates the `admin` account from `ADMIN_PASSWORD` (clear text or a bcrypt hash); without it, creates the account with a random password nobody knows, logs a warning, and leaves an existing account as it is |
| Admin password watch | `ADMIN_PASSWORD_FILE` set | user-service `WatchAdminPassword` | Re-applies the password when the mounted file changes |
| OIDC settings | Always | user-service `SeedOIDC` | Writes the `oidc_*` settings from `OIDC_*` variables ([105](105-backend-authentication.md)) |
| Demo users and progress | `SEED_MOCK_DATA=true` | user-service `SeedMockData` | Errors are logged, not fatal |
| Demo catalogue and paths | `SEED_DEV_COURSES=true` or `overwrite` | course-service `SeedDevCourses`, `SeedDevPaths` | In one transaction |

The order is: migrations, then courses seed, then users seed, so demo progress can refer to demo
courses. Every step stays idempotent.

The fixes of [190](190-backend-defects.md) added two nullable columns to `users`, created by
AutoMigrate: `sessionsvalidafter` (B7, the revocation date of the user's tokens) and
`identityname` (B11, the identity provider's login).

## Connection pool

One pool for the whole process, sized by `DB_MAX_OPEN_CONNS` and `DB_MAX_IDLE_CONNS`
([101](101-backend-configuration.md)): their default is the sum of the two services' defaults, so
one replica of the monolith opens no more connections than one replica of each service did.

## Acceptance criteria

- On an empty database, the monolith creates exactly the tables, columns, indexes and constraints
  that the two services created together (schema dump compared in a test).
- On a database migrated by the two services at `3cdf7b2`, with data, the monolith starts, applies
  only the migrations of the fixes and serves the same data.
- `_schema_migrations` holds each breaking migration once.

## Sources

`user-service/internal/db/db.go`, `seed.go`, `seed_mock.go`, `user-service/internal/models/`;
`course-service/internal/db/db.go`, `seed_dev.go`, `course-service/internal/models/`; both
`main.go`; `pupitre-helm` `values.yaml`.
