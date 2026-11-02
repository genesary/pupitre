package handlers

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// badgeResponseKey is the JSON key used for badge arrays in responses.
const badgeResponseKey = "badges"

// badgeDetail enriches a BadgeRow with the course-service badge metadata.
type badgeDetail struct {
	CourseSlug  string    `json:"courseSlug"`
	CourseTitle string    `json:"courseTitle"`
	BadgeName   string    `json:"name"`
	BadgeIcon   string    `json:"icon,omitempty"`
	EarnedAt    time.Time `json:"earnedAt"`
	EarnedBy    int64     `json:"earnedBy"`
}

// badgeSlugs extracts the course slugs of a badge listing, deduplicated.
func badgeSlugs(rows []repository.BadgeRow) []string {
	seen := make(map[string]struct{}, len(rows))

	slugs := make([]string, 0, len(rows))

	for _, row := range rows {
		if _, dup := seen[row.CourseSlug]; dup {
			continue
		}

		seen[row.CourseSlug] = struct{}{}

		slugs = append(slugs, row.CourseSlug)
	}

	return slugs
}

// enrichBadgeRows converts BadgeRows into badgeDetails.
//
// Both enrichments are resolved for the whole listing at once — one
// batched catalog lookup and one grouped count query — where this used to
// issue an HTTP request and a COUNT per badge, in series. A learner with
// thirty badges paid sixty round-trips for one page.
func (s *State) enrichBadgeRows(req *http.Request, rows []repository.BadgeRow) []badgeDetail {
	slugs := badgeSlugs(rows)

	info := s.catalog().Courses(req.Context(), slugs)

	counts, err := s.Repos.Badges.EarnedCounts(req.Context(), slugs)
	if err != nil {
		zap.L().Warn("failed to count badge earners", zap.Int("badges", len(slugs)), zap.Error(err))
	}

	details := make([]badgeDetail, 0, len(rows))

	for _, row := range rows {
		detail := badgeDetail{
			CourseSlug: row.CourseSlug,
			EarnedAt:   row.EarnedAt,
			EarnedBy:   counts[row.CourseSlug],
		}

		if course, found := info[row.CourseSlug]; found {
			detail.CourseTitle = course.Title
			if course.Badge != nil {
				detail.BadgeName = course.Badge.Name
				detail.BadgeIcon = course.Badge.Icon
			}
		}

		details = append(details, detail)
	}

	return details
}

// MyBadges godoc
// @Summary  List badges earned by the current user
// @Tags     Badges
// @Security BearerAuth
// @Produce  json
// @Success  200  {object}  map[string]interface{}
// @Router   /api/my/badges [get].
func (s *State) MyBadges(writer http.ResponseWriter, request *http.Request) {
	claims := s.claims(request)

	rows, err := s.Repos.Badges.MyBadges(request.Context(), claims.Subject)
	if err != nil {
		zap.L().Error("failed to query my badges", zap.String("userID", claims.Subject), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{badgeResponseKey: s.enrichBadgeRows(request, rows)})
}

// PublicUser godoc
// @Summary  Get public profile of a user
// @Tags     Users
// @Produce  json
// @Param    id  path  string  true  "User UUID"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/users/{id} [get].
func (s *State) PublicUser(writer http.ResponseWriter, request *http.Request) {
	userID, err := uuid.Parse(param(request, "id"))
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid user ID")

		return
	}

	found, err := s.Repos.Users.FindByID(request.Context(), userID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		"id":        found.ID,
		"username":  found.Username,
		"avatarUrl": found.AvatarURL,
	})
}

// UserBadges godoc
// @Summary  List badges earned by a user (public)
// @Tags     Badges
// @Produce  json
// @Param    id  path  string  true  "User UUID"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/users/{id}/badges [get].
func (s *State) UserBadges(writer http.ResponseWriter, request *http.Request) {
	userID := param(request, "id")

	rows, err := s.Repos.Badges.UserBadges(request.Context(), userID)
	if err != nil {
		zap.L().Error("failed to query user badges", zap.String("userID", userID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{badgeResponseKey: s.enrichBadgeRows(request, rows)})
}

const (
	// leaderboardResponseKey is the JSON key used in leaderboard responses.
	leaderboardResponseKey = "leaderboard"
)

// leaderboardEntry is one row of the public badge leaderboard response.
type leaderboardEntry struct {
	Rank      int      `json:"rank"`
	UserID    string   `json:"userId"`
	Username  string   `json:"username"`
	AvatarURL *string  `json:"avatarUrl"`
	Count     int64    `json:"count"`
	Icons     []string `json:"icons"`
}

// defaultBadgeIcon stands in for a course that declares no badge icon.
const defaultBadgeIcon = "🏅"

// buildIconCache resolves the badge icon of every slug in one batched
// catalog lookup, rather than one HTTP request per distinct course on the
// leaderboard.
func (s *State) buildIconCache(req *http.Request, slugs []string) map[string]string {
	info := s.catalog().Courses(req.Context(), slugs)

	cache := make(map[string]string, len(slugs))

	for _, slug := range slugs {
		icon := defaultBadgeIcon

		if course, found := info[slug]; found && course.Badge != nil && course.Badge.Icon != "" {
			icon = course.Badge.Icon
		}

		cache[slug] = icon
	}

	return cache
}

// BadgeLeaderboard godoc
// @Summary  List users ranked by badge count (public)
// @Tags     Badges
// @Produce  json
// @Success  200  {object}  map[string]interface{}
// @Router   /api/leaderboard [get].
func (s *State) BadgeLeaderboard(writer http.ResponseWriter, request *http.Request) {
	rows, err := s.Repos.Badges.Leaderboard(request.Context(), s.Config.LeaderboardMaxEntries)
	if err != nil {
		zap.L().Error("failed to query badge leaderboard", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	// Collect unique slugs for icon fetching.
	seenSlugs := make(map[string]struct{})

	for _, row := range rows {
		for _, slug := range row.Slugs {
			seenSlugs[slug] = struct{}{}
		}
	}

	allSlugs := make([]string, 0, len(seenSlugs))
	for slug := range seenSlugs {
		allSlugs = append(allSlugs, slug)
	}

	iconCache := s.buildIconCache(request, allSlugs)

	entries := make([]leaderboardEntry, 0, len(rows))

	for idx, row := range rows {
		icons := make([]string, 0, len(row.Slugs))

		for _, slug := range row.Slugs {
			icon := iconCache[slug]
			if icon == "" {
				icon = defaultBadgeIcon
			}

			icons = append(icons, icon)
		}

		entries = append(entries, leaderboardEntry{
			Rank:      idx + 1,
			UserID:    row.UserID,
			Username:  row.Username,
			AvatarURL: row.AvatarURL,
			Count:     row.Count,
			Icons:     icons,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{leaderboardResponseKey: entries})
}

// BadgeStats godoc
// @Summary  Get global stats for a course badge
// @Tags     Badges
// @Produce  json
// @Param    slug  path  string  true  "Course slug"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/badges/{slug} [get].
func (s *State) BadgeStats(writer http.ResponseWriter, request *http.Request) {
	courseSlug := param(request, "slug")

	count, err := s.Repos.Badges.EarnedCount(request.Context(), courseSlug)
	if err != nil {
		zap.L().Error("failed to count badge earners", zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{"courseSlug": courseSlug, "earnedBy": count})
}
