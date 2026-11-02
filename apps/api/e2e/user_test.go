//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestRegisterAndLogin covers user registration followed by login and
// profile retrieval.
func TestRegisterAndLogin(t *testing.T) {

	email := uniqueEmail("reg")
	pass := "Password123!"

	t.Run("register", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", map[string]string{
			"email":    email,
			"username": strings.Split(email, "@")[0],
			"password": pass,
		}, "")
		mustStatus(t, resp, http.StatusCreated)

		var out struct {
			Token string `json:"token"`
			User  struct {
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"user"`
		}
		decodeBody(t, resp, &out)

		if out.Token == "" {
			t.Fatal("register: expected token in response")
		}

		if out.User.Email != email {
			t.Fatalf("register: expected email %q, got %q", email, out.User.Email)
		}

		if out.User.Role != "student" {
			t.Fatalf("register: expected role student, got %q", out.User.Role)
		}
	})

	t.Run("login", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/login", map[string]string{
			"email":    email,
			"password": pass,
		}, "")
		mustStatus(t, resp, http.StatusOK)

		var out struct {
			Token string `json:"token"`
		}
		decodeBody(t, resp, &out)

		if out.Token == "" {
			t.Fatal("login: expected token in response")
		}

		t.Run("me", func(t *testing.T) {
			resp2 := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/auth/me", nil, out.Token)
			mustStatus(t, resp2, http.StatusOK)

			var me struct {
				Email string `json:"email"`
			}
			decodeBody(t, resp2, &me)

			if me.Email != email {
				t.Fatalf("me: expected email %q, got %q", email, me.Email)
			}
		})
	})

	t.Run("login_wrong_password", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/login", map[string]string{
			"email":    email,
			"password": "wrong",
		}, "")
		mustStatus(t, resp, http.StatusUnauthorized)
		resp.Body.Close()
	})
}

// TestAuthMeUnauthorized checks that /api/auth/me rejects missing tokens.
func TestAuthMeUnauthorized(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/auth/me", nil, "")
	mustStatus(t, resp, http.StatusUnauthorized)
	resp.Body.Close()
}

// TestRegisterValidation checks that missing fields are rejected.
func TestRegisterValidation(t *testing.T) {

	cases := []struct {
		name string
		body map[string]string
	}{
		{name: "missing_email", body: map[string]string{"username": "u", "password": "P@ssword1"}},
		{name: "missing_password", body: map[string]string{"email": uniqueEmail("val"), "username": "u"}},
		{name: "empty_body", body: map[string]string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", tc.body, "")
			if resp.StatusCode == http.StatusCreated {
				t.Fatalf("expected error for %s, got 201", tc.name)
			}

			resp.Body.Close()
		})
	}
}

// TestAdminUserCRUD exercises the admin user management endpoints using
// the shared admin token from TestMain.
func TestAdminUserCRUD(t *testing.T) {

	email := uniqueEmail("admin-crud")

	// Create user via register, then manage via admin endpoints
	resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", map[string]string{
		"email":    email,
		"username": strings.Split(email, "@")[0],
		"password": "Password123!",
	}, "")
	mustStatus(t, resp, http.StatusCreated)

	var reg struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}

	decodeBody(t, resp, &reg)

	if reg.User.ID == "" {
		t.Fatal("register: no user ID in response")
	}

	userID := reg.User.ID

	t.Run("get_user", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet,
			fmt.Sprintf("%s/api/admin/users/%s", globalCfg.APIURL, userID),
			nil, globalCfg.adminToken)
		mustStatus(t, resp, http.StatusOK)

		var u struct {
			Email string `json:"email"`
		}
		decodeBody(t, resp, &u)

		if u.Email != email {
			t.Fatalf("get_user: expected email %q, got %q", email, u.Email)
		}
	})

	t.Run("update_user", func(t *testing.T) {
		resp := doJSON(t, http.MethodPut,
			fmt.Sprintf("%s/api/admin/users/%s", globalCfg.APIURL, userID),
			map[string]string{"role": "admin"},
			globalCfg.adminToken)
		mustStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})

	t.Run("delete_user", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete,
			fmt.Sprintf("%s/api/admin/users/%s", globalCfg.APIURL, userID),
			nil, globalCfg.adminToken)
		mustStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})

	t.Run("get_deleted_user_returns_404", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet,
			fmt.Sprintf("%s/api/admin/users/%s", globalCfg.APIURL, userID),
			nil, globalCfg.adminToken)
		mustStatus(t, resp, http.StatusNotFound)
		resp.Body.Close()
	})
}

// TestAdminListUsers checks pagination of the admin users list.
func TestAdminListUsers(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/admin/users?limit=5", nil, globalCfg.adminToken)
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
	}

	decodeBody(t, resp, &out)

	if out.Users == nil {
		t.Fatal("admin/users: expected users array in response")
	}
}

// TestAdminRequiresAuth verifies that admin endpoints reject non-admin tokens.
func TestAdminRequiresAuth(t *testing.T) {

	email := uniqueEmail("noadmin")

	resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", map[string]string{
		"email":    email,
		"username": strings.Split(email, "@")[0],
		"password": "Password123!",
	}, "")
	mustStatus(t, resp, http.StatusCreated)

	var reg struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp, &reg)

	resp2 := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/admin/users", nil, reg.Token)
	mustStatus(t, resp2, http.StatusForbidden)
	resp2.Body.Close()
}

// TestEnrollment covers course enrollment and unenrollment for a student.
func TestEnrollment(t *testing.T) {

	// Register a student
	email := uniqueEmail("enroll")
	resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", map[string]string{
		"email":    email,
		"username": strings.Split(email, "@")[0],
		"password": "Password123!",
	}, "")
	mustStatus(t, resp, http.StatusCreated)

	var reg struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp, &reg)

	tok := reg.Token
	slug := "linux-intro"

	t.Run("enroll", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost,
			fmt.Sprintf("%s/api/enrollments/%s", globalCfg.APIURL, slug),
			nil, tok)
		// 200 (enrolled) or 409 (already enrolled) are both acceptable
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
			body := readBody(t, resp)
			t.Fatalf("enroll: expected 200 or 409, got %d: %s", resp.StatusCode, body)
		}

		resp.Body.Close()
	})

	t.Run("my_courses_includes_enrollment", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/my/courses", nil, tok)
		mustStatus(t, resp, http.StatusOK)

		var out struct {
			Courses []struct {
				Slug string `json:"slug"`
			} `json:"courses"`
		}

		decodeBody(t, resp, &out)

		for _, c := range out.Courses {
			if c.Slug == slug {
				return
			}
		}

		t.Fatalf("my/courses: %q not found in course list", slug)
	})

	t.Run("unenroll", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete,
			fmt.Sprintf("%s/api/enrollments/%s", globalCfg.APIURL, slug),
			nil, tok)
		mustStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})
}

// TestPathEnrollment covers admin-driven path enrollment and the my/paths endpoint.
func TestPathEnrollment(t *testing.T) {

	// Register a student
	email := uniqueEmail("path-enroll")
	resp := doJSON(t, http.MethodPost, globalCfg.APIURL+"/api/auth/register", map[string]string{
		"email":    email,
		"username": strings.Split(email, "@")[0],
		"password": "Password123!",
	}, "")
	mustStatus(t, resp, http.StatusCreated)

	var reg struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Token string `json:"token"`
	}

	decodeBody(t, resp, &reg)

	pathSlug := "devops-path"

	t.Run("admin_enroll_in_path", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost,
			fmt.Sprintf("%s/api/admin/paths/%s/enrollments", globalCfg.APIURL, pathSlug),
			map[string]string{"userId": reg.User.ID},
			globalCfg.adminToken)
		mustStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})

	t.Run("my_paths_includes_enrollment", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/my/paths", nil, reg.Token)
		mustStatus(t, resp, http.StatusOK)

		var out struct {
			Paths []struct {
				Slug string `json:"slug"`
			} `json:"paths"`
		}

		decodeBody(t, resp, &out)

		for _, p := range out.Paths {
			if p.Slug == pathSlug {
				return
			}
		}

		t.Fatalf("my/paths: %q not found in path list", pathSlug)
	})

	t.Run("admin_unenroll_from_path", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete,
			fmt.Sprintf("%s/api/admin/paths/%s/enrollments/%s", globalCfg.APIURL, pathSlug, reg.User.ID),
			nil, globalCfg.adminToken)
		mustStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})
}

// TestPublicSettings checks the public settings endpoint (no auth required).
func TestPublicSettings(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/settings/public", nil, "")
	mustStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestHealth checks the health endpoint the user routes share.
func TestUserServiceHealth(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/health", nil, "")
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Status string `json:"status"`
	}

	decodeBody(t, resp, &out)

	if out.Status != "ok" {
		t.Fatalf("health: expected status ok, got %q", out.Status)
	}
}
