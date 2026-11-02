package handlers

import (
	"net/http"
)

// viewedLessons returns the set of lesson slugs the user has completed in a
// course, keyed by lesson slug with value true.
func viewedLessons(s *State, r *http.Request, courseSlug, userID string) map[string]bool {
	slugs, err := s.Repos.LessonProgress.ViewedSlugs(r.Context(), userID, courseSlug)
	if err != nil {
		return nil
	}

	lessonsViewed := make(map[string]bool)
	for _, slug := range slugs {
		lessonsViewed[slug] = true
	}

	return lessonsViewed
}
