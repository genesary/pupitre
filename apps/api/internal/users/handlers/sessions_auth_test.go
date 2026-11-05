package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/genesary/pupitre/apps/api/internal/users/fake"
	"github.com/genesary/pupitre/apps/api/internal/users/models"
)

// TestCheckSession_Revocation compares issue times to the microsecond: a
// token issued just after a revocation survives it, even within the same
// second, while one issued before it, or with only a whole-second iat in
// that second, does not.
func TestCheckSession_Revocation(t *testing.T) {
	t.Parallel()

	revokedAt := time.Date(2026, time.March, 2, 10, 0, 0, 500_000_000, time.UTC)

	cases := []struct {
		name     string
		issuedAt time.Time
		want     bool
	}{
		{"issued just after, same second", revokedAt.Add(time.Microsecond), true},
		{"issued just before, same second", revokedAt.Add(-time.Microsecond), false},
		{"issued at the revocation", revokedAt, false},
		{"whole-second iat of that second", revokedAt.Truncate(time.Second), false},
		{"issued in a later second", revokedAt.Add(time.Second), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repos := fake.NewRepositories()
			userID := uuid.New()

			err := repos.Users.Create(t.Context(), &models.User{ID: userID, IsActive: true, Role: "student"})
			if err != nil {
				t.Fatalf("create user: %v", err)
			}

			err = repos.Users.RevokeSessions(t.Context(), userID, revokedAt)
			if err != nil {
				t.Fatalf("revoke: %v", err)
			}

			session, err := (&State{Repos: repos}).CheckSession(t.Context(), userID.String(), tc.issuedAt)
			if err != nil {
				t.Fatalf("check session: %v", err)
			}

			if session.Valid != tc.want {
				t.Errorf("valid = %v, want %v", session.Valid, tc.want)
			}
		})
	}
}
