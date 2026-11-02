# 101 - Backend: configuration

The three services each had a configuration loader. The monolith has **one**, in
`internal/platform/config`, with the same mechanism: built-in defaults, then a YAML file, then
environment variables, each step overriding the previous one.

## Loading order

1. Built-in defaults (table below).
2. The YAML file named by `CONFIG_PATH` (in Kubernetes, a mounted ConfigMap). A missing file is a
   warning, not an error, as today.
3. Environment variables. A secret may also come from a file through its `*_FILE` variable, which
   wins over the plain variable, as today for `ADMIN_PASSWORD_FILE`, `OIDC_CLIENT_SECRET_FILE`
   and `SSO_<ID>_CLIENT_SECRET_FILE`.

Non-fatal problems (an unreadable file, an invalid number) are returned as warnings and logged at
start-up, as both loaders do today. A value that cannot work (an invalid port) stops the start-up.

## Keys

"From" names the service the key comes from; "u" is user-service, "c" course-service, "k"
checker-service.

| YAML key | Environment variable | Default | From | Notes |
| --- | --- | --- | --- | --- |
| `port` | `PORT` | `8080` | u, c, k | **New default**: one port instead of 8081, 8082 and 8083 |
| `corsOrigins` | `CORS_ORIGINS` | `http://localhost:3000`, `http://localhost:5173` | u, c, k | Same list in the three services today |
| `databaseUrl` | `DATABASE_URL` | `postgres://pupitre:pupitre@localhost:5432/pupitre` | u, c | **Required** in production (course-service refused to start without it) |
| `dbMaxOpenConns` | `DB_MAX_OPEN_CONNS` | `30` | u (20), c (10) | **One pool**: the sum of the two pools |
| `dbMaxIdleConns` | `DB_MAX_IDLE_CONNS` | `30` | u (20), c (10) | Same |
| `jwtSecret` | `JWT_SECRET` | none | u, c | One value; **required**, at least 32 characters ([190](190-backend-defects.md) B4) |
| `jwtExpiryHours` | `JWT_EXPIRY_HOURS` | `8` | u | course-service's `24` was never used: it only verifies tokens. The Helm chart sets `24` for user-service, so deployed tokens last 24 h |
| `oauthStateSecret` | `OAUTH_STATE_SECRET` | the JWT secret | u | |
| `oauthRedirectBase` | `OAUTH_REDIRECT_BASE` | `http://localhost:3000` | u | |
| `providers[]` (`id`, `name`, `clientId`, `clientSecret`, `issuerUrl`…) | `SSO_<ID>_CLIENT_SECRET[_FILE]` | none | u | OAuth providers of `sso.providers` ([105](105-backend-authentication.md)) |
| `authRateLimitRequests` | `AUTH_RATE_LIMIT_REQUESTS` | `10` | u | Sign-in routes, per client IP |
| `authRateLimitWindowSeconds` | `AUTH_RATE_LIMIT_WINDOW_SECONDS` | `60` | u | |
| `leaderboardMaxEntries` | `LEADERBOARD_MAX_ENTRIES` | `20` | u | |
| `uploadsDir` | `UPLOADS_DIR` | `./uploads` | c | |
| `gitToken` | `GIT_TOKEN` | none | c | Token for every git source |
| `gitCredentialsPath` | `GIT_CREDENTIALS_PATH` | `/etc/pupitre/git-credentials.yaml` | c | Per-URL tokens; **new default path** (was `/etc/course-service/…`) |
| `gitCacheTtlMinutes` | `GIT_CACHE_TTL` | `10` | c | |
| `gitlabBaseUrl` | `GITLAB_BASE_URL` | `http://gitlab.local:8880` | k | Labs ([107](107-backend-labs-and-checker.md)) |
| `gitlabToken` | `GITLAB_TOKEN` | none | k | Secret |
| `labCheckRateLimitRequests` | `LAB_CHECK_RATE_LIMIT_REQUESTS` (was `RATE_LIMIT_REQUESTS`) | `20` | k | See "Lab check rate limit" |
| `labCheckRateLimitWindowSeconds` | `LAB_CHECK_RATE_LIMIT_WINDOW_SECONDS` (was `RATE_LIMIT_WINDOW_SECONDS`) | `60` | k | |
| - | `ADMIN_PASSWORD[_FILE]` | none | u | Bootstrap admin account ([105](105-backend-authentication.md)) |
| - | `OIDC_ENABLED`, `OIDC_PROVIDER_URL`, `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET[_FILE]`, `OIDC_SCOPES`, `OIDC_GROUP_CLAIM`, `OIDC_GROUP_ADMINS`, `OIDC_REDIRECT_BASE`, `OIDC_BROWSER_BASE_URL`, `OIDC_INSECURE_SKIP_VERIFY` | none | u | Seed the `oidc_*` platform settings at start-up ([105](105-backend-authentication.md)) |
| - | `SEED_DEV_COURSES` | unset | c | `true` seeds the demo catalogue, `overwrite` replaces it; never in production |
| - | `SEED_MOCK_DATA` | unset | u | `true` seeds demo users and progress; never in production |
| `trustedProxies` | `TRUSTED_PROXIES` | loopback and private networks | - | Comma-separated CIDRs or addresses whose `X-Forwarded-For` is believed ([190](190-backend-defects.md) B12) |
| - | `LOG_LEVEL` | `info` | u, c, k | `debug`, `info`, `warn`, `error` |
| - | `LOG_FORMAT` | `json` | u, c, k | `text` for local development |

### Removed keys

| Key | Why |
| --- | --- |
| `courseServiceUrl` / `COURSE_SERVICE_URL` | user-service read the catalogue over HTTP: now in-process ([104](104-backend-in-process-calls.md)) |
| `userServiceUrl` / `USER_SERVICE_URL` | course-service read progress over HTTP: now in-process |
| `checkerServiceUrl` / `CHECKER_SERVICE_URL` | course-service called checker-service: now in-process ([107](107-backend-labs-and-checker.md)) |
| `INTERNAL_SERVICE_SECRET` | No internal route is left |
| course-service `jwtExpiryHours` | Never used |

A removed key that is still set is ignored with a start-up warning naming it, so an old ConfigMap
does not break the monolith. checker-service's `RATE_LIMIT_REQUESTS` and
`RATE_LIMIT_WINDOW_SECONDS` still set the lab check limit when the `LAB_CHECK_*` variables are
not set, with a deprecation warning.

Each domain keeps its configuration package with the keys it reads; the keys above are gone from
them too, and the checker's has no port, CORS origins or secret of its own any more.

### Lab check rate limit

checker-service limited `/evaluate` and `/check-step` per client IP. Its only client was
course-service, so in practice the limit (20 checks per 60 s) applied to **all learners
together**, and course-service relayed its `429` as `502`. Since [190](190-backend-defects.md) B9
the limit counts **each learner's** checks apart, with the same defaults: over it, the learner
gets `429 Too many lab checks, retry in N s` with `Retry-After: N`, N being the window.

## Default secrets

The old defaults of `jwtSecret` and of the removed internal secret are public. The API refuses to
start when `JWT_SECRET` is that default or shorter than 32 characters
([190](190-backend-defects.md) B4); local development sets one in `apps/api/.env`.

## Acceptance criteria

- A ConfigMap of the user-service, one of course-service and the env of checker-service, merged by
  hand into one file, start the monolith with the same behaviour.
- `PORT=9000` moves the listener; `DATABASE_URL` set by both env and file uses the env value.
- Setting `USER_SERVICE_URL` logs a warning that it is ignored.

## Sources

`user-service/internal/config/config.go`, `course-service/internal/config/config.go`,
`checker-service/internal/config/config.go` and their tests; `docs/ARCHITECTURE.md`
("Configuration des services"); `docs/SSO.md`.
