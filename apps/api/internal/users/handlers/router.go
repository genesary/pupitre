package handlers

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	apimiddleware "github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
)

// remoteIP extracts the IP from r.RemoteAddr (already set to the real client
// IP by chiMiddleware.RealIP) to use as the rate-limit key.
func remoteIP(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr, nil //nolint:nilerr // best-effort: fall back to raw addr
	}

	return ip, nil
}

// Routing invariant: no path registered here may sit under a prefix that
// course-service owns (/api/courses, /api/admin/courses).
//
// The two services are one hostname behind a shared proxy, so whichever
// segment decides who owns a request has to come *before* any variable
// segment. Enrolling used to be POST /api/courses/{slug}/enroll — the owner
// was only decided after the slug, which no prefix match can express. Every
// front door then needed its own workaround: a Traefik-specific CRD, an
// nginx regex annotation, and an implementation-specific Gateway API match.
// Keying those endpoints by their own resource instead (/api/enrollments,
// /api/session-bookings) makes plain longest-prefix routing sufficient
// everywhere, so the platform no longer depends on what its ingress can
// express.
//
// Adding a route under /api/courses or /api/admin/courses reintroduces that
// coupling; TestRouter_NoCourseServicePrefixOverlap fails if you do.

// RegisterRoutes mounts every public route of the users domain on router,
// behind the 1 MB body limit, with each group's guard. It adds no global
// middleware, no /health or /metrics and no internal route: the monolith's
// router owns those (docs/specs/103-backend-http-api.md).
func RegisterRoutes(router chi.Router, state *State, cfg *config.Config) {
	sessions := apimiddleware.WithSessionCheck(state.Sessions)
	authMW := apimiddleware.Auth(cfg.JWTSecret, sessions)
	adminMW := apimiddleware.Admin(cfg.JWTSecret, sessions)
	managerMW := apimiddleware.Manager(cfg.JWTSecret, sessions)
	exportMW := apimiddleware.ManagerOrAdmin(cfg.JWTSecret, sessions)

	router.Group(func(group chi.Router) {
		group.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))

		registerPublicRoutes(group, state, cfg)
		registerAuthenticatedRoutes(group, state, authMW)
		registerAdminRoutes(group, state, adminMW)
		registerManagerRoutes(group, state, managerMW)
		registerExportRoutes(group, state, exportMW)
		registerPatternRoutes(group, state, authMW, adminMW)
	})
}

// registerPublicRoutes wires routes that require no authentication.
func registerPublicRoutes(router chi.Router, state *State, cfg *config.Config) {
	router.Get("/api/settings/public", state.PublicSettings)
	router.Get("/api/users/{id}", state.PublicUser)
	router.Get("/api/users/{id}/badges", state.UserBadges)
	router.Get("/api/badges/{slug}", state.BadgeStats)
	router.Get("/api/leaderboard", state.BadgeLeaderboard)
	router.Get("/api/auth/oauth/providers", state.ListProviders)
	router.Get("/api/auth/oauth/{provider}/authorize", state.OAuthAuthorize)
	router.Get("/api/auth/oidc/authorize", state.OIDCAuthorize)

	// Rate-limited auth endpoints — set AUTH_RATE_LIMIT_REQUESTS=0 to disable.
	router.Group(func(group chi.Router) {
		if cfg.AuthRateLimitRequests > 0 {
			window := time.Duration(cfg.AuthRateLimitWindowSeconds) * time.Second
			group.Use(httprate.LimitBy(cfg.AuthRateLimitRequests, window, remoteIP))
			// The same limit per email, whatever the address the attempts
			// come from (docs/specs/190-backend-defects.md B12).
			group.Use(limitByEmail(cfg.AuthRateLimitRequests, window))
		}

		group.Post("/api/auth/register", state.Register)
		group.Post("/api/auth/login", state.Login)
		group.Post("/api/auth/oauth/callback", state.OAuthCallback)
		group.Post("/api/auth/oidc/callback", state.OIDCCallback)
		group.Post("/api/auth/ldap/login", state.LDAPLogin)
	})
}

// registerAuthenticatedRoutes wires routes that require a valid session.
func registerAuthenticatedRoutes(router chi.Router, state *State, authMW func(http.Handler) http.Handler) {
	router.Group(func(group chi.Router) {
		group.Use(authMW)

		group.Get("/api/auth/me", state.Me)
		group.Post("/api/auth/logout", state.Logout)
		group.Put("/api/auth/profile", state.UpdateProfile)
		group.Put("/api/auth/password", state.ChangePassword)

		// Enrolling is a user-service concern keyed by course slug, so it is
		// its own resource rather than a verb hung off the catalog's URL —
		// see the routing note above.
		group.Post("/api/enrollments/{slug}", state.Enroll)
		group.Delete("/api/enrollments/{slug}", state.Unenroll)

		group.Get("/api/my/courses", state.MyCourses)
		group.Get("/api/my/paths", state.MyPaths)
		group.Get("/api/my/badges", state.MyBadges)
		group.Get("/api/my/xp", state.MyXP)
		group.Get("/api/my/skills", state.MySkills)
		group.Get("/api/my/skills/{slug}", state.MySkillModules)
		group.Get("/api/my/groups", state.MyGroups)
		group.Get("/api/my/groups/{groupId}/members", state.MyGroupMembers)

		group.Get("/api/my/session-bookings", state.MySessionBookings)
		group.Post("/api/session-bookings/{slug}/{sessionId}", state.BookSession)
		group.Delete("/api/session-bookings/{slug}/{sessionId}", state.UnbookSession)
		group.Get("/api/session-bookings/{slug}/{sessionId}/count", state.SessionBookingCount)
	})
}

// registerAdminRoutes wires routes restricted to admin users.
func registerAdminRoutes(router chi.Router, state *State, adminMW func(http.Handler) http.Handler) {
	router.Group(func(group chi.Router) {
		group.Use(adminMW)

		group.Get("/api/admin/settings", state.GetSettings)
		group.Put("/api/admin/settings", state.UpdateSettings)

		group.Get("/api/admin/stats", state.AdminStats)
		group.Get("/api/admin/leaderboard", state.AdminLeaderboard)

		group.Get("/api/admin/users", state.ListUsers)
		group.Get("/api/admin/users/providers", state.ListAuthProviders)
		group.Get("/api/admin/users/search", state.SearchUsers)
		group.Get("/api/admin/users/{userId}", state.GetUser)
		group.Put("/api/admin/users/{userId}", state.UpdateUser)
		group.Delete("/api/admin/users/{userId}", state.DeleteUser)

		group.Get("/api/admin/session-bookings/{slug}/{sessionId}", state.ListSessionBookings)
		group.Patch("/api/admin/session-bookings/{slug}/{sessionId}/{userId}/presence", state.MarkSessionPresence)

		group.Get("/api/admin/enrollments/{slug}", state.ListCourseEnrollments)
		group.Post("/api/admin/enrollments/{slug}", state.AdminEnrollUser)
		group.Get("/api/admin/enrollments/{slug}/groups", state.AdminListGroupEnrollments)
		group.Post("/api/admin/enrollments/{slug}/groups", state.AdminEnrollGroup)
		group.Delete("/api/admin/enrollments/{slug}/groups/{groupId}", state.AdminUnenrollGroup)
		// .../users/{userId} rather than .../{userId}: a bare wildcard here
		// would be shadowed by the static "groups" segment above.
		group.Delete("/api/admin/enrollments/{slug}/users/{userId}", state.AdminUnenrollUser)

		group.Get("/api/admin/paths/{slug}/enrollments", state.AdminListPathEnrollments)
		group.Post("/api/admin/paths/{slug}/enrollments", state.AdminEnrollUserInPath)
		group.Delete("/api/admin/paths/{slug}/enrollments/{userId}", state.AdminUnenrollUserFromPath)

		group.Post("/api/admin/sync-progress", state.SyncProgress)

		group.Get("/api/admin/groups", state.ListGroups)
		group.Post("/api/admin/groups", state.CreateGroup)
		group.Put("/api/admin/groups/{groupId}", state.UpdateGroup)
		group.Delete("/api/admin/groups/{groupId}", state.DeleteGroup)
		group.Get("/api/admin/groups/{groupId}/members", state.ListGroupMembers)
		group.Post("/api/admin/groups/{groupId}/members", state.AddGroupMember)
		group.Delete("/api/admin/groups/{groupId}/members/{userId}", state.RemoveGroupMember)
		group.Get("/api/admin/groups/{groupId}/subgroups", state.ListSubgroups)
		group.Post("/api/admin/groups/{groupId}/subgroups", state.CreateSubgroup)
		group.Get("/api/admin/groups/{groupId}/ancestors", state.ListAncestors)
		group.Get("/api/admin/groups/{groupId}/descendants", state.ListDescendants)
		group.Patch("/api/admin/groups/{groupId}/parent", state.MoveGroup)
		group.Get("/api/admin/groups/mappings", state.ListGroupMappings)
		group.Post("/api/admin/groups/mappings", state.UpsertGroupMapping)
		group.Delete("/api/admin/groups/mappings/{groupName}", state.DeleteGroupMapping)
	})
}

// registerManagerRoutes wires routes restricted to manager users.
func registerManagerRoutes(router chi.Router, state *State, managerMW func(http.Handler) http.Handler) {
	router.Group(func(group chi.Router) {
		group.Use(managerMW)

		group.Get("/api/manager/users", state.ManagerListUsers)
		group.Get("/api/manager/users/{userId}/enrollments", state.ManagerGetUserEnrollments)
		group.Get("/api/manager/users/{userId}/path-enrollments", state.ManagerGetUserPathEnrollments)
		group.Post("/api/manager/enrollments/{slug}", state.ManagerEnrollUser)
		group.Delete("/api/manager/enrollments/{slug}/users/{userId}", state.ManagerUnenrollUser)

		group.Get("/api/manager/groups", state.ManagerListGroups)
		group.Post("/api/manager/groups", state.ManagerCreateGroup)
		group.Delete("/api/manager/groups/{groupId}", state.ManagerDeleteGroup)
		group.Get("/api/manager/groups/{groupId}/members", state.ManagerListGroupMembers)
		group.Post("/api/manager/groups/{groupId}/members", state.ManagerAddGroupMember)
		group.Delete("/api/manager/groups/{groupId}/members/{userId}", state.ManagerRemoveGroupMember)
		group.Get("/api/manager/paths/{slug}/enrollments", state.ManagerListPathEnrollments)
		group.Post("/api/manager/groups/{groupId}/subgroups", state.ManagerCreateSubgroup)
		group.Post("/api/manager/paths/{slug}/enrollments", state.ManagerEnrollUserPath)
		group.Delete("/api/manager/paths/{slug}/enrollments/{userId}", state.ManagerUnenrollUserPath)
		group.Get("/api/manager/session-bookings/{slug}/{sessionId}", state.ListSessionBookings)
		group.Patch("/api/manager/session-bookings/{slug}/{sessionId}/{userId}/presence", state.MarkSessionPresence)
	})
}

// registerExportRoutes wires CSV export routes for managers and admins.
func registerExportRoutes(router chi.Router, state *State, exportMW func(http.Handler) http.Handler) {
	router.Group(func(group chi.Router) {
		group.Use(exportMW)
		group.Get("/api/admin/exports/categories", state.ExportCategories)
		group.Post("/api/admin/exports/preview", state.ExportPreview)
		group.Post("/api/admin/exports/download", state.ExportDownload)
	})
}

// registerPatternRoutes wires the markdown pattern routes, split between
// authenticated read access and admin-only write access.
func registerPatternRoutes(router chi.Router, state *State, authMW, adminMW func(http.Handler) http.Handler) {
	router.Group(func(group chi.Router) {
		group.Use(authMW)
		group.Get("/api/patterns", state.ListPatterns)
		group.Get("/api/patterns/{id}", state.GetPattern)
	})

	router.Group(func(group chi.Router) {
		group.Use(adminMW)
		group.Post("/api/admin/patterns", state.CreatePattern)
		group.Put("/api/admin/patterns/{name}", state.UpdatePattern)
		group.Delete("/api/admin/patterns/{name}", state.DeletePattern)
	})
}
