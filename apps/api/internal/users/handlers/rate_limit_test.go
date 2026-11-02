package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLimitByEmail is the regression test for defect B12: attempts on one
// email are limited whatever address they come from, and other emails, or
// bodies without one, are not affected.
func TestLimitByEmail(t *testing.T) {
	t.Parallel()

	handler := limitByEmail(2, time.Minute)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	post := func(addr, body string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		return rec.Code
	}

	for i := range 2 {
		if code := post(fmt.Sprintf("198.51.100.%d:1", i), `{"email":"Alice@Example.com"}`); code != http.StatusOK {
			t.Fatalf("attempt %d: %d", i, code)
		}
	}

	if code := post("198.51.100.9:1", `{"email":"alice@example.com "}`); code != http.StatusTooManyRequests {
		t.Fatalf("third attempt on the same email from a new address: %d, want 429", code)
	}

	if code := post("198.51.100.9:1", `{"email":"bob@example.com"}`); code != http.StatusOK {
		t.Fatalf("another email: %d", code)
	}

	if code := post("198.51.100.9:1", `{"code":"x","state":"y"}`); code != http.StatusOK {
		t.Fatalf("a body without email: %d", code)
	}
}
