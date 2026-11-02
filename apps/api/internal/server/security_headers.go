package server

import (
	"net/http"
	"strings"
)

// securityHeaders sets the response headers that browsers use to contain
// what the API serves (docs/specs/190-backend-defects.md B15). The API
// answers JSON, which no page should frame or run, so every response
// forbids both. Uploaded files are shown by the frontend, from the same
// origin: they may be framed by it, and a script inside one (an SVG or an
// HTML file) never runs.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		headers := writer.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "no-referrer")

		if strings.HasPrefix(request.URL.Path, "/uploads/") {
			headers.Set("Content-Security-Policy", "script-src 'none'; frame-ancestors 'self'")
			headers.Set("X-Frame-Options", "SAMEORIGIN")
		} else {
			headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			headers.Set("X-Frame-Options", "DENY")
		}

		next.ServeHTTP(writer, request)
	})
}
