package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/genesary/pupitre/user-service/fake"
	"github.com/genesary/pupitre/user-service/internal/config"
)

// courseServicePrefixes are the URL prefixes course-service owns. A path
// registered here that falls under one of them cannot be separated from
// course-service's own routes by a prefix match, because the segment that
// decides the owner would come after a variable one (the course slug).
//
// That is what forced every front door to carry its own workaround — a
// Traefik IngressRoute, an nginx regex annotation, an implementation-specific
// Gateway API RegularExpression match. Keeping this list empty of user-service
// routes is what lets the platform be routed by any ingress or Gateway using
// only Core-conformance prefix matching.
var courseServicePrefixes = []string{ //nolint:gochecknoglobals // fixture data for the test below
	"/api/courses",
	"/api/admin/courses",
}

// TestRouter_NoCourseServicePrefixOverlap walks every route this service
// registers and fails if any of them nests under a course-service prefix.
func TestRouter_NoCourseServicePrefixOverlap(t *testing.T) {
	t.Parallel()

	router := ownershipRouter()

	var offenders []string

	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		for _, prefix := range courseServicePrefixes {
			if route == prefix || strings.HasPrefix(route, prefix+"/") {
				offenders = append(offenders, method+" "+route)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("user-service registers %d route(s) under a course-service prefix:\n  %s\n\n"+
			"A prefix match cannot separate these from course-service, so routing them needs a "+
			"regex — which not every ingress or Gateway can express. Key the endpoint by its own "+
			"resource instead (e.g. /api/enrollments/{slug}).",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// TestRouter_OwnedPrefixesAreDisjoint checks the converse: every route this
// service registers falls under one of the prefixes the deployment routes to
// user-service. A route outside them would reach the frontend instead.
func TestRouter_OwnedPrefixesAreDisjoint(t *testing.T) {
	t.Parallel()

	// Mirrors the user-service entries of `pupitre.routeTable` in the Helm
	// chart (helm/templates/_helpers.tpl).
	owned := []string{
		"/api/auth", "/api/my", "/api/settings", "/api/admin", "/api/manager",
		"/api/users", "/api/badges", "/api/leaderboard", "/api/patterns",
		"/api/enrollments", "/api/session-bookings",
		"/internal", "/health", "/metrics",
	}

	router := ownershipRouter()

	var unrouted []string

	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		for _, prefix := range owned {
			if route == prefix || strings.HasPrefix(route, prefix+"/") {
				return nil
			}
		}

		unrouted = append(unrouted, method+" "+route)

		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	if len(unrouted) > 0 {
		t.Errorf("user-service registers %d route(s) outside every prefix routed to it:\n  %s\n\n"+
			"Add the prefix to `pupitre.routeTable` in the Helm chart, and to this list.",
			len(unrouted), strings.Join(unrouted, "\n  "))
	}
}

// ownershipRouter builds the full router with fake repositories.
func ownershipRouter() *chi.Mux {
	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry,
		CORSOrigins: []string{"*"}, InternalSecret: htInternalSecret,
	}

	return BuildRouter(&State{Repos: fake.NewRepositories(), Config: cfg}, cfg, false)
}
