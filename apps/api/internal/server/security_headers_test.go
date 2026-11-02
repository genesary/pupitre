package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecurityHeaders is the regression test of defect B15
// (docs/specs/190-backend-defects.md): every response carries the security
// headers, and an uploaded file can be framed by the frontend but runs no
// script.
func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	handler := securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	cases := []struct {
		path, csp, frame string
	}{
		{"/api/courses", "default-src 'none'; frame-ancestors 'none'", "DENY"},
		{"/health", "default-src 'none'; frame-ancestors 'none'", "DENY"},
		{"/uploads/diagram.svg", "script-src 'none'; frame-ancestors 'self'", "SAMEORIGIN"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, http.NoBody))

		headers := rec.Header()
		if headers.Get("X-Content-Type-Options") != "nosniff" || headers.Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: missing nosniff or Referrer-Policy: %v", tc.path, headers)
		}

		if headers.Get("Content-Security-Policy") != tc.csp || headers.Get("X-Frame-Options") != tc.frame {
			t.Errorf("%s: CSP %q, X-Frame-Options %q; want %q, %q", tc.path,
				headers.Get("Content-Security-Policy"), headers.Get("X-Frame-Options"), tc.csp, tc.frame)
		}
	}
}
