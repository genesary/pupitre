# 190 - Backend: known defects

Defects of the Go backend at `3cdf7b2`, found while writing the frontend ([090](090-gaps-and-defects.md))
or listed by the security audit of `genesary/pupitre` (`docs/audit-pupitre.html`, issue
genesary/pupitre#194). Per invariant 5 of [100](100-backend-scope-and-architecture.md), **the move
keeps each one as it is**. Each fix is a separate change after the move (batch B3 of
[199](199-backend-migration-plan.md)), with its own test, and updates this table.

## Resolved by the move itself

| Audit | Defect | Why it disappears |
| --- | --- | --- |
| H6 | `/internal/*` is served on the public port, protected only by a shared secret with a public default | No internal route is left ([103](103-backend-http-api.md)) |
| C1 (part) | Default `INTERNAL_SERVICE_SECRET` | The secret is removed ([101](101-backend-configuration.md)) |
| B19 | The start-up test of user-service (`TestRun_BootAndGracefulShutdown`) failed at `3cdf7b2`: two `run()` of the package shared one `SIGTERM` | The service and its test are gone; `internal/server/run_integration_test.go` starts the monolith, waits for `/health`, sends `SIGTERM` and gets a clean return |

## Fixed after the move

Batch B3.1 of [199](199-backend-migration-plan.md), security first. Each fix has a regression
test that failed before it. Batch B3.2 fixed the manager group mapping, batch B3.3 the
secondary security findings, batch B3.4 the functional defects, batch B4 the chart's uploads
volume.

| # | Source | Defect | Fix |
| --- | --- | --- | --- |
| B4 | Audit C1 | The default `JWT_SECRET` was accepted: anyone could mint an admin token | The API refuses to start when `JWT_SECRET` is the old public default or shorter than 32 characters ([101](101-backend-configuration.md)) |
| B5 | - | Without `ADMIN_PASSWORD`, the bootstrap admin got `Admin@1234` | An admin account created without `ADMIN_PASSWORD` gets a random password nobody knows, with a warning; an existing account keeps its password ([102](102-backend-data-and-migrations.md)) |
| B6 | Audit H2 | Managers acted outside their groups | A manager adds to their groups only users already in their scope, exports the members of their groups only, and sees and marks the attendance of their scope only (`callerScope`, `ExportScope`); the mock mode does the same |
| B7 | Audit H3 | No revocation: the role was read from the token for its whole life | The guards read the user's row on every request: an unknown, deactivated or revoked user is refused with `401 Session expired`, and the role comes from the row. `POST /api/auth/logout` (called by « Déconnexion »), a password change and a deactivation revoke every token issued until then ([105](105-backend-authentication.md)) |
| B10 | Audit M1 | `/api/batch/courses` was public and returned private courses | The public route returns public courses only; the users domain's in-process catalog still sees every course |
| B11 | Audit M2 | The GitLab project of a lab check came from the editable username | The project comes from the identity provider's `preferred_username`, recorded at each OIDC sign-in; a learner without one is refused (`403`) ([107](107-backend-labs-and-checker.md)) |
| B12 | Audit H4, H7 | `RealIP` trusted any `X-Forwarded-For`, so the sign-in rate limit could be bypassed | Forwarding headers are read only from `TRUSTED_PROXIES`, from the right; sign-in attempts are also limited per email ([101](101-backend-configuration.md), [105](105-backend-authentication.md)) |
| B20 | Front + back test | A group mapping to `manager` failed with `500 Database error`: the check constraint allowed only `admin` and `student` | The constraint allows `manager`; a breaking migration drops the old one so AutoMigrate creates it again ([102](102-backend-data-and-migrations.md)) |
| B13 | Audit M3 | The OAuth `state` was a JWT not bound to the browser (login CSRF) | `authorize` sets an HttpOnly cookie `pupitre_oauth_state` (`SameSite=Lax`, path `/api/auth`, `Secure` when the redirect base is HTTPS) holding a nonce that the signed `state` carries too; a callback accepts the state only with that cookie, then clears it ([105](105-backend-authentication.md)) |
| B14 | Audit M6 | Account enumeration through the sign-in messages | An unknown email, an SSO account and a wrong password all get `401 Invalid email or password`, in about the same time ([105](105-backend-authentication.md)) |
| B15 | Audit M7 | No security headers | Every API response carries `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and a CSP that forbids framing and scripts; `/uploads/*` may be framed by the frontend's origin but runs no script ([103](103-backend-http-api.md)) |
| B17 | Audit L4 | The quiz submission returned `correctAnswer` for every question: a learner could read it in the browser's network tools and submit again with it | `correctAnswer` is no longer in the submission response; the review shows ✓/✗, points, feedback and links ([013](013-quizzes.md)). The only change to a response struct of `3cdf7b2`, made on purpose |
| B1 | 090 | A manager who created a subgroup was not made a member of it, so it left their scope and their list | The manager becomes a member of the subgroup, in the same transaction; the mock mode does the same ([030](030-manager-space.md)) |
| B8 | Audit M4 | CSV formula injection (`=`, `+`, `-`, `@`) | Every exported cell that starts with `=`, `+`, `-`, `@`, a tab or a carriage return is prefixed with `'` (`platform/csvsafe`), in the users and lab exports and in the mock mode |
| B9 | checker | The lab check limit (20 per 60 s) applied to all learners together, and its `429` reached the learner as `502` | The limit counts each learner's checks apart; over it, the learner gets `429` with `Retry-After` and the frontend says « Trop de vérifications d'affilée : réessayez dans N s. » ([101](101-backend-configuration.md), [014](014-labs.md)) |
| B23 | Monolith test | 14 module prerequisites of 11 demo courses (`python-basics`' `listes,-tuples-et-dictionnaires`, `linux-intro`, `container-security`…) did not match their module's slug: the final quizzes stayed `locked` | Every prerequisite names its module's exact slug, in the seed and the mock fixture; a test checks every seed prerequisite. A local database seeded before keeps its courses until `SEED_DEV_COURSES=overwrite` |
| B24 | Monolith work | Booking a session of a **private** course ignored its capacity: the capacity was read with an anonymous course read, which a private course refuses | The capacity is read whatever the course's visibility (`CourseSource.CourseSessions`, [104](104-backend-in-process-calls.md)) |
| B22 | Helm chart | course-service wrote uploads to `/data/uploads`, but the chart mounted no volume there and the root filesystem is read-only: media uploads and `replication` could not be written, nor survive a restart or be shared between replicas | The API's chart ([`pupitre2-helm`](https://github.com/jojo91550/pupitre2-helm), batch B4) mounts a `PersistentVolumeClaim` at `/data/uploads`, kept on uninstall, with `fsGroup` 1000; `ReadWriteMany` for several replicas, an emptyDir when persistence is disabled ([108](108-backend-operations-and-testing.md)) |

B21 was withdrawn: with `oidc_group_admins` set, every OIDC sign-in makes a user outside those
groups a `student`, managers included. That is the rule documented by the reference chart
(`sso.oidc.groupAdmins`: "this mapping is authoritative"), so it is kept as it is
([105](105-backend-authentication.md)). A deployment with managers leaves `groupAdmins` empty, maps
`admin` and `manager` to groups in `/admin/groups`, or names managers by hand.

## Kept as the reference

These audit findings describe a behaviour the reference documents or that its deployment relies
on. Changing it would move pupitre2 away from `genesary/pupitre`, so it is kept as it is:

| # | Source | Finding | Why it is kept |
| --- | --- | --- | --- |
| B15 | Audit M7 | `/metrics` is public | Documented as public by the reference and routed on purpose by its chart's ingress; the API's chart routes it the same way ([108](108-backend-operations-and-testing.md)) |
| B16 | Audit L1, L2 | OIDC client secret and LDAP bind password stored in clear; `oidc_insecure_skip_verify` available in production | Encryption would need a new mandatory key in every deployment. `insecureSkipVerify` is a documented option of the reference chart, for providers behind an internal CA |
| B17 | Audit L3 | No email verification at registration | The reference has no mail sending, and pupitre2 signs in through Keycloak only ([001](001-keycloak-authentication.md)), which verifies emails. For the same reason, `POST /api/auth/register` still answers `409` for a taken email or username: without a verification mail, it cannot hide that an account exists |
| B2 | 090 A22 | `PathEnrollmentRow` and `GroupRow` have no json tags: fields are sent as `Slug`, `EnrolledAt`… | The reference's frontend reads `Slug` and so does pupitre2's (`api-types.ts`): it works, renaming would change the response for nothing |
| B3 | 090 A23 | No route recomputes progress; `/api/admin/sync-progress` marks one lesson for one user | A need to specify, not a defect: the route is kept as it is |
| B18 | 090 A2, A18 | No course `thumbnail`; `GET /api/my/paths` does not return `level` | The reference has neither field: they would be new features (column, admin input, display), out of the scope of the move |

## To fix after the move

None.

Audit findings C2, C3, C4 and H5 were in the Astro frontend and are covered by
[003](003-security.md) and [001](001-keycloak-authentication.md).

## Rules

- A fix that changes a response (B2, B9, B10, B17, B18) updates the frontend types, the mock mode
  and the specs 000-099 in the same change.
- A fix never weakens a test of the move: it adds a test that failed before.
