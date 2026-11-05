package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// moaRun drives the ManagerOrAdmin middleware with the given Authorization
// header value and returns the recorded status code.
func moaRun(t *testing.T, authHeader string) int {
	t.Helper()

	mw := ManagerOrAdmin(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	return rec.Code
}

// TestManagerOrAdmin_ManagerAllowed lets a manager token through.
func TestManagerOrAdmin_ManagerAllowed(t *testing.T) {
	t.Parallel()

	tok, _ := CreateToken("m-1", "m@example.com", "manager", "m-1", testSecret, 24)

	if code := moaRun(t, "Bearer "+tok); code != http.StatusOK {
		t.Errorf("manager: want 200, got %d", code)
	}
}

// TestManagerOrAdmin_AdminAllowed lets an admin token through.
func TestManagerOrAdmin_AdminAllowed(t *testing.T) {
	t.Parallel()

	tok, _ := CreateToken("a-1", "a@example.com", "admin", "a-1", testSecret, 24)

	if code := moaRun(t, "Bearer "+tok); code != http.StatusOK {
		t.Errorf("admin: want 200, got %d", code)
	}
}

// TestManagerOrAdmin_StudentForbidden rejects a student token with 403.
func TestManagerOrAdmin_StudentForbidden(t *testing.T) {
	t.Parallel()

	tok, _ := CreateToken("s-1", "s@example.com", "student", "s-1", testSecret, 24)

	if code := moaRun(t, "Bearer "+tok); code != http.StatusForbidden {
		t.Errorf("student: want 403, got %d", code)
	}
}

// TestManagerOrAdmin_MissingHeader rejects a request with no Authorization
// header.
func TestManagerOrAdmin_MissingHeader(t *testing.T) {
	t.Parallel()

	if code := moaRun(t, ""); code != http.StatusUnauthorized {
		t.Errorf("missing header: want 401, got %d", code)
	}
}

// TestManagerOrAdmin_InvalidToken rejects an unparsable bearer token.
func TestManagerOrAdmin_InvalidToken(t *testing.T) {
	t.Parallel()

	if code := moaRun(t, "Bearer garbage"); code != http.StatusUnauthorized {
		t.Errorf("invalid token: want 401, got %d", code)
	}
}

// TestManagerOrAdmin_MissingSubject rejects a token whose subject is empty.
func TestManagerOrAdmin_MissingSubject(t *testing.T) {
	t.Parallel()

	tok := signedClaims(t, "admin", "")

	if code := moaRun(t, "Bearer "+tok); code != http.StatusUnauthorized {
		t.Errorf("missing subject: want 401, got %d", code)
	}
}
