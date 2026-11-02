package handlers

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// CourseSessionInfo is the part of a course session BookSession needs to
// enforce per-session seat limits.
type CourseSessionInfo struct {
	ID       string `json:"id"`
	Capacity int    `json:"capacity,omitempty"`
}

// markPresenceBody is the request body for MarkSessionPresence.
type markPresenceBody struct {
	Present bool `json:"present"`
}

// sessionCapacity returns the capacity of sessionID within slug by calling
// course-service. Returns 0 if the session has no capacity limit or is not
// found (unlimited booking allowed).
func (s *State) sessionCapacity(req *http.Request, slug, sessionID string) int {
	if !slugRE.MatchString(slug) {
		return 0
	}

	sessions, err := s.courseSource().CourseSessions(req.Context(), slug)
	if err != nil {
		zap.L().Warn("could not fetch course for capacity check", zap.String("slug", slug), zap.Error(err))

		return 0
	}

	for _, sess := range sessions {
		if sess.ID == sessionID {
			return sess.Capacity
		}
	}

	return 0
}

// BookSession godoc
// @Summary  Reserve a seat for an in-person session
// @Tags     Sessions
// @Produce  json
// @Param    slug       path  string  true  "Course slug"
// @Param    sessionId  path  string  true  "Session ID"
// @Success  204  "No Content"
// @Failure  409  {object}  map[string]string
// @Router   /api/courses/{slug}/sessions/{sessionId}/book [post].
func (s *State) BookSession(writer http.ResponseWriter, req *http.Request) {
	claims := s.claims(req)
	slug := param(req, "slug")
	sessionID := param(req, "sessionId")

	if capacity := s.sessionCapacity(req, slug, sessionID); capacity > 0 {
		count, err := s.Repos.SessionBookings.CountBySession(req.Context(), slug, sessionID)
		if err != nil {
			zap.L().Error("failed to count session bookings",
				zap.String("courseSlug", slug),
				zap.String("sessionID", sessionID),
				zap.Error(err))
			s.Error(writer, http.StatusInternalServerError, "Failed to check session capacity")

			return
		}

		if count >= int64(capacity) {
			s.Error(writer, http.StatusConflict, "Session is full")

			return
		}
	}

	err := s.Repos.SessionBookings.Book(req.Context(), claims.Subject, slug, sessionID)
	if err != nil {
		zap.L().Error("failed to book session",
			zap.String("userID", claims.Subject),
			zap.String("courseSlug", slug),
			zap.String("sessionID", sessionID),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to book session")

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// UnbookSession godoc
// @Summary  Cancel a seat reservation for an in-person session
// @Tags     Sessions
// @Produce  json
// @Param    slug       path  string  true  "Course slug"
// @Param    sessionId  path  string  true  "Session ID"
// @Success  204  "No Content"
// @Router   /api/courses/{slug}/sessions/{sessionId}/book [delete].
func (s *State) UnbookSession(writer http.ResponseWriter, req *http.Request) {
	claims := s.claims(req)
	slug := param(req, "slug")
	sessionID := param(req, "sessionId")

	err := s.Repos.SessionBookings.Unbook(req.Context(), claims.Subject, slug, sessionID)
	if err != nil {
		zap.L().Error("failed to unbook session",
			zap.String("userID", claims.Subject),
			zap.String("courseSlug", slug),
			zap.String("sessionID", sessionID),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to unbook session")

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// MySessionBookings godoc
// @Summary  List the current user's session bookings
// @Tags     Sessions
// @Produce  json
// @Success  200  {object}  map[string]interface{}
// @Router   /api/my/session-bookings [get].
func (s *State) MySessionBookings(writer http.ResponseWriter, req *http.Request) { //nolint:dupl // same shape as MySkills; different repository and response key
	claims := s.claims(req)

	bookings, err := s.Repos.SessionBookings.ListByUser(req.Context(), claims.Subject)
	if err != nil {
		zap.L().Error("failed to list session bookings",
			zap.String("userID", claims.Subject),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to fetch bookings")

		return
	}

	if bookings == nil {
		bookings = []repository.SessionBookingRow{}
	}

	s.JSON(writer, http.StatusOK, map[string]any{"bookings": bookings})
}

// ListSessionBookings godoc
// @Summary  List all participants for a session (admin/manager)
// @Tags     Sessions
// @Produce  json
// @Param    slug       path  string  true  "Course slug"
// @Param    sessionId  path  string  true  "Session ID"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/admin/courses/{slug}/sessions/{sessionId}/bookings [get].
func (s *State) ListSessionBookings(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")
	sessionID := param(req, "sessionId")

	participants, err := s.Repos.SessionBookings.ListBySession(req.Context(), slug, sessionID)
	if err != nil {
		zap.L().Error("failed to list session participants",
			zap.String("courseSlug", slug),
			zap.String("sessionID", sessionID),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to fetch participants")

		return
	}

	// A manager sees the participants of their scope only.
	scope, err := s.callerScope(req.Context(), s.claims(req))
	if err != nil {
		zap.L().Error("failed to load manager scope", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to fetch participants")

		return
	}

	if scope != nil {
		visible := participants[:0]
		for _, participant := range participants {
			if scope[participant.UserID] {
				visible = append(visible, participant)
			}
		}

		participants = visible
	}

	s.JSON(writer, http.StatusOK, map[string]any{"participants": participants})
}

// MarkSessionPresence godoc
// @Summary  Mark a participant as present or absent (admin/manager)
// @Tags     Sessions
// @Accept   json
// @Produce  json
// @Param    slug       path  string  true  "Course slug"
// @Param    sessionId  path  string  true  "Session ID"
// @Param    userId     path  string  true  "User ID"
// @Success  200  {object}  map[string]bool
// @Router   /api/admin/courses/{slug}/sessions/{sessionId}/bookings/{userId}/presence [patch].
func (s *State) MarkSessionPresence(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")
	sessionID := param(req, "sessionId")
	userID := param(req, "userId")

	var body markPresenceBody

	err := decode(req, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid request body")

		return
	}

	// A manager marks the attendance of their scope only.
	scope, err := s.callerScope(req.Context(), s.claims(req))
	if err != nil {
		zap.L().Error("failed to load manager scope", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to update presence")

		return
	}

	if scope != nil && !scope[userID] {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	err = s.Repos.SessionBookings.MarkPresent(req.Context(), userID, slug, sessionID, body.Present)
	if err != nil {
		zap.L().Error("failed to mark presence",
			zap.String("userID", userID),
			zap.String("courseSlug", slug),
			zap.String("sessionID", sessionID),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to update presence")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]bool{"ok": true})
}

// SessionBookingCount godoc
// @Summary  Get the number of bookings for a session
// @Tags     Sessions
// @Produce  json
// @Param    slug       path  string  true  "Course slug"
// @Param    sessionId  path  string  true  "Session ID"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/courses/{slug}/sessions/{sessionId}/booking-count [get].
func (s *State) SessionBookingCount(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")
	sessionID := param(req, "sessionId")

	count, err := s.Repos.SessionBookings.CountBySession(req.Context(), slug, sessionID)
	if err != nil {
		zap.L().Error("failed to count session bookings",
			zap.String("courseSlug", slug),
			zap.String("sessionID", sessionID),
			zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to count bookings")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{"count": count})
}
