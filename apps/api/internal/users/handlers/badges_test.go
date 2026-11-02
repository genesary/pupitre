package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// awardBadge grants a badge to userID for courseSlug on the fake repo,
// failing the test on error.
func awardBadge(t *testing.T, repo *fake.BadgeRepository, userID, courseSlug string) {
	t.Helper()

	err := repo.Award(t.Context(), userID, courseSlug)
	if err != nil {
		t.Fatalf("Award: %v", err)
	}
}

// TestMyBadges_Empty returns an empty badge list for a user with no awards.
func TestMyBadges_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp[badgeResponseKey]; !ok {
		t.Errorf("expected %q key in response", badgeResponseKey)
	}
}

// TestMyBadges_WithAward lists a badge the current user has earned.
func TestMyBadges_WithAward(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	repos.Badges = badges
	awardBadge(t, badges, "user-uuid-1", "linux-intro")

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rows := htSliceField(t, resp, badgeResponseKey)
	if len(rows) != 1 {
		t.Fatalf("expected 1 badge, got %d", len(rows))
	}

	if got := htMapField(t, rows[0])["courseSlug"]; got != "linux-intro" {
		t.Errorf("courseSlug = %v, want linux-intro", got)
	}
}

// TestMyBadges_DBError surfaces a repository failure as 500.
func TestMyBadges_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	badges.Err = errors.New("db down")
	repos.Badges = badges

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestMyBadges_Unauthenticated is rejected without a token.
func TestMyBadges_Unauthenticated(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestUserBadges_OK lists the badges for an arbitrary user id.
func TestUserBadges_OK(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	repos.Badges = badges
	awardBadge(t, badges, "some-user", "docker-fundamentals")

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/users/some-user/badges", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rows := htSliceField(t, resp, badgeResponseKey)
	if len(rows) != 1 {
		t.Errorf("expected 1 badge, got %d", len(rows))
	}
}

// TestUserBadges_DBError surfaces a repository failure as 500.
func TestUserBadges_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	badges.Err = errors.New("db down")
	repos.Badges = badges

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/users/some-user/badges", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestBadgeStats_OK reports the earner count for a course badge.
func TestBadgeStats_OK(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	repos.Badges = badges
	awardBadge(t, badges, "u1", "kubernetes-basics")
	awardBadge(t, badges, "u2", "kubernetes-basics")

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/badges/kubernetes-basics", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got := resp["earnedBy"]; got != float64(2) {
		t.Errorf("earnedBy = %v, want 2", got)
	}
}

// TestBadgeStats_DBError surfaces a repository failure as 500.
func TestBadgeStats_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	badges.Err = errors.New("db down")
	repos.Badges = badges

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/badges/whatever", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestBadgeLeaderboard_OK returns a leaderboard payload.
func TestBadgeLeaderboard_OK(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/leaderboard", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp[leaderboardResponseKey]; !ok {
		t.Errorf("expected %q key", leaderboardResponseKey)
	}
}

// TestBadgeLeaderboard_DBError surfaces a repository failure as 500.
func TestBadgeLeaderboard_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	badges := fake.NewBadgeRepository()
	badges.Err = errors.New("db down")
	repos.Badges = badges

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/leaderboard", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestPublicUser_InvalidID rejects a non-UUID path parameter.
func TestPublicUser_InvalidID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/users/not-a-uuid", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestPublicUser_NotFound returns 404 for an unknown user.
func TestPublicUser_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/users/"+uuid.NewString(), "", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestPublicUser_OK returns the public projection of a known user.
func TestPublicUser_OK(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	avatar := "https://cdn.example/a.png"
	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID:        id,
		Username:  "publicjane",
		Email:     "jane@example.com",
		AvatarURL: &avatar,
		IsActive:  true,
	})

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/users/"+id.String(), "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["username"] != "publicjane" {
		t.Errorf("username = %v, want publicjane", resp["username"])
	}
}

// TestBadgeLeaderboard_RankedWithIcons exercises the ranking loop and icon
// enrichment (buildIconCache) against a mock course-service.
func TestBadgeLeaderboard_RankedWithIcons(t *testing.T) {
	t.Parallel()

	courseSvc, _ := newCatalogServer(t, catalogFixture{Courses: map[string]CourseInfo{
		"linux-intro":   courseFixture("linux-intro", "Linux", "Tux", "🐧"),
		"docker-basics": courseFixture("docker-basics", "Docker", "Whale", "🐳"),
	}})

	avatar := "https://cdn/a.png"
	badges := fake.NewBadgeRepository()
	badges.LeaderboardRows = []repository.BadgeLeaderboardRow{
		{UserID: "u1", Username: "ace", AvatarURL: &avatar, Count: 2, Slugs: []string{"linux-intro", "docker-basics"}},
		{UserID: "u2", Username: "novice", Count: 1, Slugs: []string{"linux-intro"}},
	}

	repos := fake.NewRepositories()
	repos.Badges = badges
	s := &State{Repos: repos, Config: &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		LeaderboardMaxEntries: 10,
	}, Courses: courseSourceAt(courseSvc.URL)}
	r := BuildRouter(s, s.Config, false)

	rec := htDo(t, r, http.MethodGet, "/api/leaderboard", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Leaderboard []struct {
			Rank     int      `json:"rank"`
			Username string   `json:"username"`
			Count    int64    `json:"count"`
			Icons    []string `json:"icons"`
		} `json:"leaderboard"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Leaderboard) != 2 {
		t.Fatalf("want 2 rows, got %+v", resp.Leaderboard)
	}

	if resp.Leaderboard[0].Rank != 1 || resp.Leaderboard[0].Username != "ace" {
		t.Errorf("row 0 = %+v", resp.Leaderboard[0])
	}

	if len(resp.Leaderboard[0].Icons) != 2 || resp.Leaderboard[0].Icons[0] != "🐧" {
		t.Errorf("icons not enriched: %+v", resp.Leaderboard[0].Icons)
	}
}

// TestMyBadges_EnrichedFromCourseService fills courseTitle/name/icon from a
// mock course-service, covering the success branch of enrichBadgeRows.
func TestMyBadges_EnrichedFromCourseService(t *testing.T) {
	t.Parallel()

	courseSvc, _ := newCatalogServer(t, catalogFixture{Courses: map[string]CourseInfo{
		"linux-intro": courseFixture("linux-intro", "Linux Intro", "Tux", "🐧"),
	}})

	badges := fake.NewBadgeRepository()
	awardBadge(t, badges, "user-uuid-1", "linux-intro")

	repos := fake.NewRepositories()
	repos.Badges = badges
	s := &State{Repos: repos, Config: &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
	}, Courses: courseSourceAt(courseSvc.URL)}
	r := BuildRouter(s, s.Config, false)

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rows := htSliceField(t, resp, badgeResponseKey)
	if len(rows) != 1 {
		t.Fatalf("want 1 badge, got %d", len(rows))
	}

	row := htMapField(t, rows[0])
	if row["courseTitle"] != "Linux Intro" || row["name"] != "Tux" {
		t.Errorf("badge not enriched: %+v", row)
	}
}

// TestMyBadges_CourseServiceErrorIsTolerated returns the raw badge when
// course-service enrichment fails (fetchCourseServiceJSON non-200 path).
func TestMyBadges_CourseServiceErrorIsTolerated(t *testing.T) {
	t.Parallel()

	courseSvc, _ := newCatalogServer(t, catalogFixture{Fail: true})

	badges := fake.NewBadgeRepository()
	awardBadge(t, badges, "user-uuid-1", "linux-intro")

	repos := fake.NewRepositories()
	repos.Badges = badges
	s := &State{Repos: repos, Config: &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
	}, Courses: courseSourceAt(courseSvc.URL)}
	r := BuildRouter(s, s.Config, false)

	rec := htDo(t, r, http.MethodGet, "/api/my/badges", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 despite enrichment failure, got %d", rec.Code)
	}

	var resp map[string]any

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	rows := htSliceField(t, resp, badgeResponseKey)
	if len(rows) != 1 || htMapField(t, rows[0])["courseSlug"] != "linux-intro" {
		t.Errorf("expected the un-enriched badge to still be returned: %+v", rows)
	}
}
