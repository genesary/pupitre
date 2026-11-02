package middleware

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testSecret is the HMAC signing secret shared by tests in this file.
const testSecret = "test-signing-secret-32bytes-long!!"

// TestCreateToken verifies CreateToken issues a non-empty signed token.
func TestCreateToken(t *testing.T) {
	t.Parallel()

	token, err := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

// TestVerifyToken_Valid verifies VerifyToken accepts a valid token.
func TestVerifyToken_Valid(t *testing.T) {
	t.Parallel()

	token, err := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	claims, err := VerifyToken(token, testSecret)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Errorf("Subject: want user-1, got %q", claims.Subject)
	}

	if claims.Email != "user@example.com" {
		t.Errorf("Email: want user@example.com, got %q", claims.Email)
	}

	if claims.Role != "student" {
		t.Errorf("Role: want student, got %q", claims.Role)
	}
}

// TestVerifyToken_WrongSecret verifies VerifyToken rejects a token
// signed with a different secret.
func TestVerifyToken_WrongSecret(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)

	_, err := VerifyToken(token, "wrong-secret")
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

// TestVerifyToken_Malformed verifies VerifyToken rejects malformed
// token strings.
func TestVerifyToken_Malformed(t *testing.T) {
	t.Parallel()

	_, err := VerifyToken("not.a.valid.token", testSecret)
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

// TestVerifyToken_Expired verifies VerifyToken rejects expired tokens.
func TestVerifyToken_Expired(t *testing.T) {
	t.Parallel()

	// Create token with -1 hour expiry (already expired).
	claims := Claims{
		Email: "user@example.com",
		Role:  "student",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}

	_, err = VerifyToken(tok, testSecret)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

// TestVerifyToken_WrongSigningMethod verifies VerifyToken rejects
// tokens using a non-HMAC signing method.
func TestVerifyToken_WrongSigningMethod(t *testing.T) {
	t.Parallel()

	// Create a token signed with RS256 (not HMAC) — should be rejected.
	_, err := VerifyToken("header.payload.sig", testSecret)
	if err == nil {
		t.Error("expected error for invalid token structure")
	}
}

// TestGetClaims_Set verifies GetClaims returns claims stored in the
// request context.
func TestGetClaims_Set(t *testing.T) {
	t.Parallel()

	claims := &Claims{Email: "a@b.com", Role: "admin"}
	claims.Subject = "uid-123"

	ctx := context.WithValue(context.Background(), ClaimsKey, claims)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)

	got := GetClaims(req)
	if got == nil {
		t.Fatal("GetClaims returned nil")
	}

	if got.Email != "a@b.com" {
		t.Errorf("Email: want a@b.com, got %q", got.Email)
	}
}

// TestGetClaims_Missing verifies GetClaims returns nil when no claims
// are set on the request context.
func TestGetClaims_Missing(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	if got := GetClaims(req); got != nil {
		t.Errorf("GetClaims with no claims: want nil, got %+v", got)
	}
}

// TestAuth_Valid verifies Auth lets a valid bearer token through.
func TestAuth_Valid(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)

	mw := Auth(testSecret)

	var captured *Claims

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetClaims(r)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Auth valid: want 200, got %d", rec.Code)
	}

	if captured == nil || captured.Subject != "user-1" {
		t.Errorf("claims not propagated: %+v", captured)
	}
}

// TestAuth_MissingHeader verifies Auth rejects requests without an
// Authorization header.
func TestAuth_MissingHeader(t *testing.T) {
	t.Parallel()

	mw := Auth(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Auth missing header: want 401, got %d", rec.Code)
	}
}

// TestAuth_InvalidToken verifies Auth rejects an unparsable token.
func TestAuth_InvalidToken(t *testing.T) {
	t.Parallel()

	mw := Auth(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer bad-token")

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Auth invalid token: want 401, got %d", rec.Code)
	}
}

// TestAuth_MissingSubject verifies Auth rejects tokens without a
// subject claim.
func TestAuth_MissingSubject(t *testing.T) {
	t.Parallel()

	// Valid issuer, audience and expiry, but no subject: the token must be
	// rejected by the subject guard rather than by claim validation.
	tok := signedClaims(t, "student", "")

	mw := Auth(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Auth missing subject: want 401, got %d", rec.Code)
	}
}

// TestAdmin_ValidAdminToken verifies Admin lets a valid admin token
// through.
func TestAdmin_ValidAdminToken(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("admin-1", "admin@example.com", "admin", "admin-1", testSecret, 24)

	mw := Admin(testSecret)

	var captured *Claims

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetClaims(r)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Admin valid: want 200, got %d", rec.Code)
	}

	if captured == nil || captured.Role != "admin" {
		t.Errorf("admin claims not propagated: %+v", captured)
	}
}

// TestAdmin_StudentForbidden verifies Admin rejects a non-admin role.
func TestAdmin_StudentForbidden(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)

	mw := Admin(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Admin student: want 403, got %d", rec.Code)
	}
}

// TestAdmin_MissingHeader verifies Admin rejects requests without an
// Authorization header.
func TestAdmin_MissingHeader(t *testing.T) {
	t.Parallel()

	mw := Admin(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Admin missing header: want 401, got %d", rec.Code)
	}
}

// TestVerifyToken_NonHMACMethod verifies VerifyToken rejects a token
// with a forged RS256 header.
func TestVerifyToken_NonHMACMethod(t *testing.T) {
	t.Parallel()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user-1","email":"u@test.com","role":"student"}`))
	fake := header + "." + payload + ".invalidsig"

	_, err := VerifyToken(fake, testSecret)
	if err == nil {
		t.Error("expected error for non-HMAC signing method")
	}
}

// TestAdmin_InvalidToken verifies Admin rejects an unparsable token.
func TestAdmin_InvalidToken(t *testing.T) {
	t.Parallel()

	mw := Admin(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Admin invalid token: want 401, got %d", rec.Code)
	}
}

// TestAdmin_MissingSubject verifies Admin rejects tokens without a
// subject claim.
func TestAdmin_MissingSubject(t *testing.T) {
	t.Parallel()

	tok := signedClaims(t, "admin", "")
	mw := Admin(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Admin missing subject: want 401, got %d", rec.Code)
	}
}

// ── Manager middleware ────────────────────────────────────────────────────────

// TestManager_ValidManagerToken verifies Manager lets a manager token through.
func TestManager_ValidManagerToken(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("mgr-1", "mgr@example.com", "manager", "mgr-1", testSecret, 24)
	mw := Manager(testSecret)

	var captured *Claims

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetClaims(r)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Manager valid: want 200, got %d", rec.Code)
	}

	if captured == nil || captured.Role != "manager" {
		t.Errorf("manager claims not propagated: %+v", captured)
	}
}

// TestManager_StudentForbidden verifies Manager rejects a student token.
func TestManager_StudentForbidden(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("user-1", "user@example.com", "student", "user-1", testSecret, 24)
	mw := Manager(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Manager student: want 403, got %d", rec.Code)
	}
}

// TestManager_AdminForbidden verifies Manager rejects an admin token.
func TestManager_AdminForbidden(t *testing.T) {
	t.Parallel()

	token, _ := CreateToken("admin-1", "admin@example.com", "admin", "admin-1", testSecret, 24)
	mw := Manager(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Manager admin: want 403, got %d", rec.Code)
	}
}

// TestManager_MissingHeader verifies Manager rejects requests without auth.
func TestManager_MissingHeader(t *testing.T) {
	t.Parallel()

	mw := Manager(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Manager missing header: want 401, got %d", rec.Code)
	}
}

// TestManager_InvalidToken verifies Manager rejects an unparsable token.
func TestManager_InvalidToken(t *testing.T) {
	t.Parallel()

	mw := Manager(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Manager invalid token: want 401, got %d", rec.Code)
	}
}

// signedClaims signs claims with the test secret, filling in the issuer,
// audience and expiry that VerifyToken requires. Tests that probe a single
// claim must otherwise pass iss/aud/exp validation first, or they assert a
// 401 that never reaches the code under test.
func signedClaims(t *testing.T, role, subject string) string {
	t.Helper()

	claims := Claims{
		Email: "user@example.com",
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}

	return tok
}

// TestVerifyToken_Empty rejects an empty token string.
func TestVerifyToken_Empty(t *testing.T) {
	t.Parallel()

	_, err := VerifyToken("", testSecret)
	if err == nil {
		t.Error("expected error for empty token")
	}
}

// TestVerifyToken_MissingExpiry rejects a correctly-signed token that carries
// no exp claim, which the parser would otherwise treat as never expiring.
func TestVerifyToken_MissingExpiry(t *testing.T) {
	t.Parallel()

	claims := Claims{
		Role: "student",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  "user-1",
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer:   jwtIssuer,
			Audience: jwt.ClaimStrings{jwtAudience},
		},
	}

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}

	_, err = VerifyToken(tok, testSecret)
	if err == nil {
		t.Error("expected error for token without exp claim")
	}
}

// TestVerifyToken_MissingIssuerAudience rejects a token that omits the iss
// and aud claims rather than accepting it as unconstrained.
func TestVerifyToken_MissingIssuerAudience(t *testing.T) {
	t.Parallel()

	claims := Claims{
		Role: "student",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign claims: %v", err)
	}

	_, err = VerifyToken(tok, testSecret)
	if err == nil {
		t.Error("expected error for token without iss/aud claims")
	}
}
