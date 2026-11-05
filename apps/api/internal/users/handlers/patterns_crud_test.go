package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestPatterns_CRUDLifecycle creates a markdown pattern (with a creator),
// reads it back through patternDTO, renames it and deletes it.
func TestPatterns_CRUDLifecycle(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	admin := htAuthHeaderForSubject(t, "admin", uuid.NewString())

	// Create.
	rec := htDo(t, r, http.MethodPost, "/api/admin/patterns",
		`{"name":"callout","label":"Callout","html":"<aside>{{content}}</aside>","scope":"global"}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		CreatedBy *string `json:"createdBy"`
	}

	err := json.NewDecoder(rec.Body).Decode(&created)
	if err != nil {
		t.Fatalf("decode create: %v", err)
	}

	if created.ID == "" || created.CreatedBy == nil {
		t.Fatalf("expected id and createdBy to be set: %+v", created)
	}

	// Get by id.
	rec = htDo(t, r, http.MethodGet, "/api/patterns/"+created.ID, "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", rec.Code)
	}

	// List.
	rec = htDo(t, r, http.MethodGet, "/api/patterns", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}

	// Update by name.
	rec = htDo(t, r, http.MethodPut, "/api/admin/patterns/callout",
		`{"name":"callout","label":"Callout v2","html":"<aside>x</aside>","scope":"global"}`, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete by name.
	rec = htDo(t, r, http.MethodDelete, "/api/admin/patterns/callout", "", admin)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Fatalf("delete: want 200/204, got %d", rec.Code)
	}

	// Gone.
	rec = htDo(t, r, http.MethodGet, "/api/patterns/"+created.ID, "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: want 404, got %d", rec.Code)
	}
}

// TestPatterns_CreateValidation rejects bad bodies.
func TestPatterns_CreateValidation(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	admin := htAuthHeaderForSubject(t, "admin", uuid.NewString())

	rec := htDo(t, r, http.MethodPost, "/api/admin/patterns", "{", admin)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: want 400, got %d", rec.Code)
	}

	rec = htDo(t, r, http.MethodPost, "/api/admin/patterns", `{"name":"x"}`, admin)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing fields: want 400, got %d", rec.Code)
	}
}

// TestPatterns_GetInvalidID rejects a non-UUID path parameter.
func TestPatterns_GetInvalidID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/patterns/not-a-uuid", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}
