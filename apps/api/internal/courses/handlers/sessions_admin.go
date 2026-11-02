package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

const (
	// sessionIDBytes is the number of random bytes used to generate a session ID.
	// 8 bytes = 16 hex chars; collision probability is negligible even at scale.
	sessionIDBytes = 8
)

// sessionNotFoundMessage is returned when a session ID matches no session
// of the course.
const sessionNotFoundMessage = "Session not found"

// sessionBody is the request payload for creating or updating a session.
type sessionBody struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	Location string `json:"location"`
	Capacity int    `json:"capacity"`
}

// generateSessionID returns a random identifier of the form "sess-<hex>".
func generateSessionID() (string, error) {
	buf := make([]byte, sessionIDBytes)

	_, err := rand.Read(buf)
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	return "sess-" + hex.EncodeToString(buf), nil
}

// sessionFromBody converts a sessionBody and ID into a content.Session.
func sessionFromBody(id string, body sessionBody) content.Session {
	return content.Session{
		ID:       id,
		Title:    body.Title,
		Date:     body.Date,
		Location: body.Location,
		Capacity: body.Capacity,
	}
}

// CreateSession godoc
// @Summary  Add a session to a course (admin or manager)
// @Tags     Admin - Sessions
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/sessions [post].
func (s *State) CreateSession(writer http.ResponseWriter, req *http.Request) {
	slug := chi.URLParam(req, "slug")

	var body sessionBody

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.Title == "" || body.Date == "" {
		s.Error(writer, http.StatusBadRequest, "title and date are required")

		return
	}

	sessionID, err := generateSessionID()
	if err != nil {
		zap.L().Error("generate session id failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to generate session ID")

		return
	}

	// One row insert, rather than a read-modify-write of the whole course
	// definition — two managers adding a session at the same time can no
	// longer clobber each other.
	err = s.Repos.Courses.PutSession(req.Context(), slug, sessionFromBody(sessionID, body))
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "create session",
			zap.String("slug", slug), zap.String("sessionId", sessionID))

		return
	}

	s.JSON(writer, http.StatusCreated, map[string]string{"id": sessionID})
}

// UpdateSession godoc
// @Summary  Update a session on a course (admin or manager)
// @Tags     Admin - Sessions
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/sessions/{sessionId} [put].
func (s *State) UpdateSession(writer http.ResponseWriter, req *http.Request) {
	slug := chi.URLParam(req, "slug")
	sessionID := chi.URLParam(req, "sessionId")

	var body sessionBody

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	exists, err := s.Repos.Courses.SessionExists(req.Context(), slug, sessionID)
	if err != nil {
		zap.L().Error("check session failed", zap.String("slug", slug), zap.String("sessionId", sessionID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	if !exists {
		s.Error(writer, http.StatusNotFound, sessionNotFoundMessage)

		return
	}

	err = s.Repos.Courses.PutSession(req.Context(), slug, sessionFromBody(sessionID, body))
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "update session",
			zap.String("slug", slug), zap.String("sessionId", sessionID))

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{"id": sessionID})
}

// DeleteSession godoc
// @Summary  Delete a session from a course (admin or manager)
// @Tags     Admin - Sessions
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/sessions/{sessionId} [delete].
func (s *State) DeleteSession(writer http.ResponseWriter, req *http.Request) {
	slug := chi.URLParam(req, "slug")
	sessionID := chi.URLParam(req, "sessionId")

	err := s.Repos.Courses.DeleteSession(req.Context(), slug, sessionID)
	if err != nil {
		s.writeRepoError(writer, err, sessionNotFoundMessage, "delete session",
			zap.String("slug", slug), zap.String("sessionId", sessionID))

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{messageJSONKey: "Session deleted"})
}
