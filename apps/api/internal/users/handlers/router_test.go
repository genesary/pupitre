package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRemoteIP strips the port from RemoteAddr and falls back to the raw
// value when there is none.
func TestRemoteIP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	req.RemoteAddr = "198.51.100.9:53344"

	got, err := remoteIP(req)
	if err != nil || got != "198.51.100.9" {
		t.Errorf("remoteIP(%q) = %q, %v; want 198.51.100.9", req.RemoteAddr, got, err)
	}

	req.RemoteAddr = "bare-host"

	got, err = remoteIP(req)
	if err != nil || got != "bare-host" {
		t.Errorf("remoteIP fallback = %q, %v; want bare-host", got, err)
	}
}
