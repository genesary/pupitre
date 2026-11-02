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

// localGroup builds a local (admin-created) group model with the given id and
// optional parent.
func localGroup(id uuid.UUID, name string, parent *uuid.UUID) models.Group {
	return models.Group{ID: id, Name: name, Source: "local", ParentID: parent}
}

// TestUpdateGroup_MissingName rejects an empty name.
func TestUpdateGroup_MissingName(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPut, "/api/admin/groups/"+uuid.NewString(), `{"name":""}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateGroup_OK renames an existing local group.
func TestUpdateGroup_OK(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(localGroup(id, "old", nil))
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPut, "/api/admin/groups/"+id.String(), `{"name":"new"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

// TestUpdateGroup_NotFound returns 404 when no local group matches.
func TestUpdateGroup_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPut, "/api/admin/groups/"+uuid.NewString(), `{"name":"x"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestCreateSubgroup_MissingName rejects an empty name.
func TestCreateSubgroup_MissingName(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/groups/"+uuid.NewString()+"/subgroups", `{}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCreateSubgroup_OK creates a child under a valid parent id.
func TestCreateSubgroup_OK(t *testing.T) {
	t.Parallel()

	parent := uuid.New()
	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(localGroup(parent, "parent", nil))
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/groups/"+parent.String()+"/subgroups",
		`{"name":"child"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", rec.Code)
	}

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["id"] == "" {
		t.Error("expected an id in the response")
	}
}

// TestCreateSubgroup_InvalidParent returns 409 when the parent id is not a
// UUID.
func TestCreateSubgroup_InvalidParent(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/groups/not-a-uuid/subgroups",
		`{"name":"child"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestListSubgroups_OK returns the direct children of a group.
func TestListSubgroups_OK(t *testing.T) {
	t.Parallel()

	parent := uuid.New()
	child := uuid.New()
	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(
		localGroup(parent, "parent", nil),
		localGroup(child, "child", &parent),
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+parent.String()+"/subgroups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(htSliceField(t, resp, "groups")) != 1 {
		t.Errorf("expected 1 subgroup, got %v", resp["groups"])
	}
}

// TestListSubgroups_DBError surfaces a repository failure as 500.
func TestListSubgroups_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	gr := fake.NewGroupRepository()
	gr.Err = errors.New("db down")
	repos.Groups = gr
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+uuid.NewString()+"/subgroups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListAncestors covers the success and failure branches.
func TestListAncestors(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+uuid.NewString()+"/ancestors", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	repos := fake.NewRepositories()
	gr := fake.NewGroupRepository()
	gr.Err = errors.New("db down")
	repos.Groups = gr

	rec = htDo(t, newTestRouterWithRepos(repos), http.MethodGet,
		"/api/admin/groups/"+uuid.NewString()+"/ancestors", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 on error, got %d", rec.Code)
	}
}

// TestListDescendants covers the success and failure branches.
func TestListDescendants(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/admin/groups/"+uuid.NewString()+"/descendants", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	repos := fake.NewRepositories()
	gr := fake.NewGroupRepository()
	gr.Err = errors.New("db down")
	repos.Groups = gr

	rec = htDo(t, newTestRouterWithRepos(repos), http.MethodGet,
		"/api/admin/groups/"+uuid.NewString()+"/descendants", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 on error, got %d", rec.Code)
	}
}

// TestMoveGroup_BadJSON rejects a malformed body.
func TestMoveGroup_BadJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPatch, "/api/admin/groups/"+uuid.NewString()+"/parent", "{", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestMoveGroup_OK moves a group to root (null parent).
func TestMoveGroup_OK(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPatch, "/api/admin/groups/"+uuid.NewString()+"/parent",
		`{"parentId":null}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestMoveGroup_RepoError maps a repository failure to 400.
func TestMoveGroup_RepoError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	gr := fake.NewGroupRepository()
	gr.Err = errors.New("cycle detected")
	repos.Groups = gr
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPatch, "/api/admin/groups/"+uuid.NewString()+"/parent",
		`{"parentId":null}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}
