package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMiddleware covers the regression of defect B12: forwarding headers
// are believed only from trusted proxies, and only up to the first
// untrusted hop from the right.
func TestMiddleware(t *testing.T) {
	t.Parallel()

	trusted, err := ParsePrefixes(append([]string{"203.0.113.10"}, DefaultTrustedProxies...))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cases := []struct {
		name, peer, forwarded, realIP, want string
	}{
		{"direct client, spoofed header ignored", "198.51.100.7:4000", "1.2.3.4", "", "198.51.100.7:4000"},
		{"behind the ingress", "10.0.0.5:4000", "198.51.100.7", "", "198.51.100.7"},
		{"client prepends a fake hop", "10.0.0.5:4000", "1.2.3.4, 198.51.100.7", "", "198.51.100.7"},
		{"chain of trusted proxies", "10.0.0.5:4000", "198.51.100.7, 203.0.113.10, 10.0.0.9", "", "198.51.100.7"},
		{"X-Real-IP from a trusted proxy", "127.0.0.1:4000", "", "198.51.100.7", "198.51.100.7"},
		{"garbage hop", "10.0.0.5:4000", "not-an-ip", "", "10.0.0.5:4000"},
		{"every hop trusted", "10.0.0.5:4000", "10.1.1.1, 10.0.0.9", "", "10.1.1.1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got string

			handler := Middleware(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				got = req.RemoteAddr
			}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.peer

			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}

			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if got != tc.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParsePrefixes refuses a malformed entry.
func TestParsePrefixes(t *testing.T) {
	t.Parallel()

	_, err := ParsePrefixes([]string{"10.0.0.0/8", "nope"})
	if err == nil {
		t.Fatal("ParsePrefixes accepted a malformed entry")
	}
}
