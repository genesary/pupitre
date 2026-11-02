//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// courseToken obtains a fresh student JWT for the course tests.
func courseToken(t *testing.T) string {
	t.Helper()

	email := uniqueEmail("course-tok")
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

	return reg.Token
}

// TestCourseServiceHealth checks the health endpoint the course routes share.
func TestCourseServiceHealth(t *testing.T) {

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

// TestListCourses verifies that the public course list returns a valid JSON array.
func TestListCourses(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/courses", nil, "")
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Courses []struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		} `json:"courses"`
	}

	decodeBody(t, resp, &out)

	if out.Courses == nil {
		t.Fatal("list courses: expected courses array in response")
	}
}

// TestGetCourse checks that a known course is retrievable.
func TestGetCourse(t *testing.T) {

	slug := "linux-intro"

	resp := doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/courses/%s", globalCfg.APIURL, slug), nil, "")
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Slug string `json:"slug"`
	}

	decodeBody(t, resp, &out)

	if out.Slug != slug {
		t.Fatalf("get course: expected slug %q, got %q", slug, out.Slug)
	}
}

// TestGetCourseNotFound checks that a non-existent course returns 404.
func TestGetCourseNotFound(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/courses/does-not-exist-xyz", nil, "")
	mustStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

// TestListModulesRequiresAuth checks that listing modules requires a JWT.
func TestListModulesRequiresAuth(t *testing.T) {

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/courses/linux-intro/modules", globalCfg.APIURL),
		nil, "")
	mustStatus(t, resp, http.StatusUnauthorized)
	resp.Body.Close()
}

// TestListModulesAuthenticated verifies that a valid token can list modules.
func TestListModulesAuthenticated(t *testing.T) {

	tok := courseToken(t)

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/courses/linux-intro/modules", globalCfg.APIURL),
		nil, tok)
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Modules []struct {
			Index int    `json:"index"`
			Title string `json:"title"`
		} `json:"modules"`
	}

	decodeBody(t, resp, &out)

	if out.Modules == nil {
		t.Fatal("list modules: expected modules array in response")
	}
}

// TestListLessonsRequiresAuth checks that the lessons endpoint rejects unauthenticated requests.
func TestListLessonsRequiresAuth(t *testing.T) {

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/courses/linux-intro/lessons", globalCfg.APIURL),
		nil, "")
	mustStatus(t, resp, http.StatusUnauthorized)
	resp.Body.Close()
}

// TestListPaths checks the public path list endpoint.
func TestListPaths(t *testing.T) {

	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/api/paths", nil, "")
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Paths []struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		} `json:"paths"`
	}

	decodeBody(t, resp, &out)

	if out.Paths == nil {
		t.Fatal("list paths: expected paths array in response")
	}
}
