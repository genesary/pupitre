package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// seedGroupWithMember returns repos with one local group that has a single
// member, plus the group id.
func seedGroupWithMember(t *testing.T) (*fake.GroupRepository, string) {
	t.Helper()

	gid := uuid.New()
	member := uuid.New()

	gr := fake.NewGroupRepository(models.Group{ID: gid, Name: "cohort", Source: "local"})
	gr.UserGroup = []models.UserGroup{{UserID: member.String(), GroupID: gid}}

	return gr, gid.String()
}

// TestAdminListGroupMembers_Direct returns the direct members of a group.
func TestAdminListGroupMembers_Direct(t *testing.T) {
	t.Parallel()

	gr, gid := seedGroupWithMember(t)
	repos := fake.NewRepositories()
	repos.Groups = gr

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+gid+"/members", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp["members"]; !ok {
		t.Errorf("expected members key: %v", resp)
	}
}

// TestAdminListGroupMembers_Recursive uses the recursive query parameter.
func TestAdminListGroupMembers_Recursive(t *testing.T) {
	t.Parallel()

	gr, gid := seedGroupWithMember(t)
	repos := fake.NewRepositories()
	repos.Groups = gr

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+gid+"/members?recursive=true", "",
		htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminDeleteGroup_Empty deletes a local group with no members or
// subgroups.
func TestAdminDeleteGroup_Empty(t *testing.T) {
	t.Parallel()

	gid := uuid.New()
	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(models.Group{ID: gid, Name: "empty", Source: "local"})

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/admin/groups/"+gid.String(), "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminDeleteGroup_HasMembers is a 409 when the group still has members.
func TestAdminDeleteGroup_HasMembers(t *testing.T) {
	t.Parallel()

	gr, gid := seedGroupWithMember(t)
	repos := fake.NewRepositories()
	repos.Groups = gr

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/admin/groups/"+gid, "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestAdminDeleteGroup_HasSubgroups is a 409 when the group has children.
func TestAdminDeleteGroup_HasSubgroups(t *testing.T) {
	t.Parallel()

	parent := uuid.New()
	child := uuid.New()
	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(
		models.Group{ID: parent, Name: "parent", Source: "local"},
		models.Group{ID: child, Name: "child", Source: "local", ParentID: &parent},
	)

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/admin/groups/"+parent.String(), "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestAdminDeleteGroup_NotFound is a 404 for an unknown group.
func TestAdminDeleteGroup_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodDelete, "/api/admin/groups/"+uuid.NewString(), "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}
