package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Sessions that can be revoked (docs/specs/190-backend-defects.md B7). A
// token stays a signed JWT, but each request also checks the user's row: the
// user must still exist and be active, and the token must have been issued
// after the user's last revocation. The role comes from the row too, so a
// demotion takes effect on the next request rather than at the token's
// expiry.

// CheckSession implements middleware.SessionCheck for the users of this
// domain. The monolith hands it to the guards of every domain.
func (s *State) CheckSession(ctx context.Context, subject string, issuedAt time.Time) (middleware.Session, error) {
	userID, err := uuid.Parse(subject)
	if err != nil {
		return middleware.Session{}, nil //nolint:nilerr // a malformed subject is an invalid session, not a failure
	}

	state, err := s.Repos.Users.SessionState(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return middleware.Session{}, nil
	}

	if err != nil {
		return middleware.Session{}, fmt.Errorf("check session: %w", err)
	}

	// JWT times have a one-second resolution: a token issued in the second
	// of the revocation is revoked too.
	revoked := state.SessionsValidAfter != nil && !issuedAt.After(*state.SessionsValidAfter)

	return middleware.Session{Valid: state.IsActive && !revoked, Role: state.Role}, nil
}

// revokeSessions ends every session of the user issued until now.
func (s *State) revokeSessions(ctx context.Context, userID uuid.UUID) error {
	err := s.Repos.Users.RevokeSessions(ctx, userID, time.Now().Truncate(time.Second))
	if err != nil {
		return fmt.Errorf("revoke sessions of %s: %w", userID, err)
	}

	return nil
}

// Logout godoc
// @Summary   End every session of the signed-in user
// @Tags      Auth
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]string
// @Router    /api/auth/logout [post].
func (s *State) Logout(writer http.ResponseWriter, request *http.Request) {
	userID, err := uuid.Parse(s.claims(request).Subject)
	if err != nil {
		s.Error(writer, http.StatusUnauthorized, "Invalid token")

		return
	}

	err = s.revokeSessions(request.Context(), userID)
	if err != nil {
		zap.L().Error("logout failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Signed out"})
}
