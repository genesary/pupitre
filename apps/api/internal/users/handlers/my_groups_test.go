package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ── MyGroups ──────────────────────────────────────────────────────────────────

// TestMyGroups_Unauthorized verifies 401 when no token is provided.
func TestMyGroups_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/my/groups", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestMyGroups_Empty verifies 200 with empty list when user has no groups.
func TestMyGroups_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/my/groups", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["total"] == nil {
		t.Error("expected total in response")
	}
}

// TestMyGroups_WithGroups verifies 200 with group data, excluding the
// platform-wide "everyone" group.
func TestMyGroups_WithGroups(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse(htUserID)
	everyoneID := uuid.New()
	teamID := uuid.New()

	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository(
		models.Group{ID: everyoneID, Name: "everyone", Source: "local"},
		models.Group{ID: teamID, Name: "team-rh", Source: "local"},
	)
	groups.UserGroup = []models.UserGroup{
		{UserID: userID.String(), GroupID: everyoneID},
		{UserID: userID.String(), GroupID: teamID},
	}
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/groups", "", htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	groups2 := htSliceField(t, resp, "groups")
	for _, g := range groups2 {
		m := htMapField(t, g)
		if m["name"] == "everyone" {
			t.Error("everyone group must be excluded from my-groups response")
		}
	}
}

// TestMyGroups_DBError verifies 500 when group lookup fails.
func TestMyGroups_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/groups", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── MyGroupMembers ────────────────────────────────────────────────────────────

// TestMyGroupMembers_Unauthorized verifies 401 when no token is provided.
func TestMyGroupMembers_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/my/groups/"+htGroupID+"/members", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestMyGroupMembers_NotMember verifies 403 when the caller is not in the
// requested group.
func TestMyGroupMembers_NotMember(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/my/groups/"+htGroupID+"/members", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestMyGroupMembers_Success verifies 200 when the caller belongs to the
// requested group.
func TestMyGroupMembers_Success(t *testing.T) {
	t.Parallel()

	groupUUID := uuid.MustParse(htGroupID)
	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository(models.Group{ID: groupUUID, Name: "team-rh", Source: "local"})
	groups.UserGroup = []models.UserGroup{
		{UserID: htUserID, GroupID: groupUUID},
	}
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/groups/"+htGroupID+"/members", "", htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["members"] == nil {
		t.Error("expected members in response")
	}
}

// TestMyGroupMembers_DBError verifies 500 when group membership check fails.
func TestMyGroupMembers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/groups/"+htGroupID+"/members", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
