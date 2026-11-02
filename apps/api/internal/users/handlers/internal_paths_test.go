package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// decodeEnrolled pulls the "enrolled" boolean out of a JSON response body.
func decodeEnrolled(t *testing.T, body []byte) bool {
	t.Helper()

	var resp map[string]bool

	err := json.Unmarshal(body, &resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	return resp["enrolled"]
}

// TestInternalCheckPathEnrollment_MissingParams returns enrolled=false when
// userId or pathSlugs are absent.
func TestInternalCheckPathEnrollment_MissingParams(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, http.MethodGet, "/internal/paths/check", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	if decodeEnrolled(t, rec.Body.Bytes()) {
		t.Error("expected enrolled=false for missing params")
	}
}

// TestInternalCheckPathEnrollment_Enrolled returns true when the user is
// enrolled in one of the requested paths.
func TestInternalCheckPathEnrollment_Enrolled(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repos := fake.NewRepositories()
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID:     userID,
		PathSlug:   "devops-path",
		EnrolledAt: time.Now(),
	})
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodGet,
		"/internal/paths/check?userId="+userID.String()+"&pathSlugs=other-path,devops-path", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	if !decodeEnrolled(t, rec.Body.Bytes()) {
		t.Error("expected enrolled=true")
	}
}

// TestInternalCheckPathEnrollment_NotEnrolled returns false when the user has
// no matching path enrollment.
func TestInternalCheckPathEnrollment_NotEnrolled(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, http.MethodGet,
		"/internal/paths/check?userId="+uuid.NewString()+"&pathSlugs=devops-path", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	if decodeEnrolled(t, rec.Body.Bytes()) {
		t.Error("expected enrolled=false")
	}
}

// TestInternalCheckPathEnrollment_DBError surfaces a repository failure as
// 500.
func TestInternalCheckPathEnrollment_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodGet,
		"/internal/paths/check?userId="+uuid.NewString()+"&pathSlugs=devops-path", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
