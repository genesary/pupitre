package handlers

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// CourseCompletion is the request body sent to the user-service when a
// course is completed. Skills, Difficulty and SkillTotalCourses are forwarded
// so user-service can update the learner's per-skill expertise level.
type CourseCompletion struct {
	UserID            string              `json:"userId"`
	CourseSlug        string              `json:"courseSlug"`
	Skills            map[string]struct{} `json:"skills,omitempty"`
	Difficulty        string              `json:"difficulty,omitempty"`
	SkillTotalCourses map[string]int      `json:"skillTotalCourses,omitempty"`
}

// lessonSummary is the public API representation of a lesson within a
// lesson list.
type lessonSummary struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Order  int    `json:"order"`
	Viewed bool   `json:"viewed"`
}

// lessonDetail is the public API representation of a single lesson,
// including its full content.
type lessonDetail struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Order   int    `json:"order"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Viewed  bool   `json:"viewed"`
}

// autoEnroll silently enrolls the user in a public course through the
// users domain. Failures are only logged — the course is accessible
// regardless, so they must not affect the response.
func (s *State) autoEnroll(userID, courseSlug string) {
	s.detached("auto-enroll", func(ctx context.Context, users UserProgress) error {
		return users.AutoEnroll(ctx, userID, courseSlug)
	}, zap.String("userID", userID), zap.String("courseSlug", courseSlug))
}

// ListLessons godoc
// @Summary   List lessons for a course
// @Tags      Lessons
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Failure   403   {object}  map[string]string
// @Failure   404   {object}  map[string]string
// @Router    /api/courses/{slug}/lessons [get].
func (s *State) ListLessons(writer http.ResponseWriter, req *http.Request) {
	view, ok := s.learnerView(writer, req, lessonAccessDeniedMessage)
	if !ok {
		return
	}

	out := make([]lessonSummary, 0, len(view.modules))
	for pos, mod := range view.modules {
		out = append(out, lessonSummary{
			Slug:   mod.Slug(),
			Title:  mod.Name,
			Order:  pos + 1,
			Viewed: view.progress.Viewed[mod.Slug()],
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{lessonsJSONKey: out})
}

// lessonAccessDeniedMessage is returned when a learner asks for the lessons
// of a private course they are not enrolled in.
const lessonAccessDeniedMessage = "Enroll in this course to access lessons"

// moduleLessonBody resolves the display body for a module-backed lesson,
// fetching remote git content when configured.
func (s *State) moduleLessonBody(ctx context.Context, mod content.Module) string {
	body := mod.Content()
	if mod.Type != moduleTypeText {
		body = content.ReplicatedPath(ctx, mod, s.Config.UploadsDir)
	}

	if mod.HasGitContent() {
		data, err := s.GitCache.FetchModuleContent(ctx, mod.Src, mod.Ref, mod.Path, s.tokenForRepo(mod.Src))
		if err != nil {
			body = "⚠️ This content is not yet available. Please check back later.\n\n_Failed to load from remote repository._"
		} else {
			body = string(data)
		}
	}

	return body
}

// findModuleLesson looks up a module-backed lesson by slug among modules,
// returning its zero-based position when found.
func findModuleLesson(modules []content.Module, lessonSlug string) (content.Module, int, bool) {
	for pos, mod := range modules {
		if mod.Slug() == lessonSlug {
			return mod, pos, true
		}
	}

	return content.Module{}, 0, false
}

// GetLesson godoc
// @Summary   Get a lesson by slug
// @Tags      Lessons
// @Security  BearerAuth
// @Produce   json
// @Param     slug         path  string  true  "Course slug"
// @Param     lessonSlug  path  string  true  "Lesson slug"
// @Success   200   {object}  map[string]interface{}
// @Failure   403   {object}  map[string]string
// @Failure   404   {object}  map[string]string
// @Router    /api/courses/{slug}/lessons/{lessonSlug} [get].
func (s *State) GetLesson(writer http.ResponseWriter, req *http.Request) {
	lessonSlug := param(req, "lessonSlug")

	view, ok := s.learnerView(writer, req, lessonAccessDeniedMessage)
	if !ok {
		return
	}

	mod, pos, found := findModuleLesson(view.modules, lessonSlug)
	if !found {
		s.Error(writer, http.StatusNotFound, "Lesson not found")

		return
	}

	if mod.Type == moduleTypeQuiz {
		s.Error(writer, http.StatusNotFound, "Quiz modules use a separate endpoint")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{lessonJSONKey: lessonDetail{
		Slug:    mod.Slug(),
		Title:   mod.Name,
		Order:   pos + 1,
		Type:    mod.Type,
		Content: s.moduleLessonBody(req.Context(), mod),
		Viewed:  view.progress.Viewed[mod.Slug()],
	}})
}

// postLessonComplete notifies the user-service that lessonSlug was
// completed by userID, writing an HTTP error and returning false on any
// failure.
func (s *State) postLessonComplete(
	writer http.ResponseWriter, req *http.Request, userID, courseSlug, lessonSlug string,
) bool {
	err := s.users().MarkLessonComplete(req.Context(), userID, courseSlug, lessonSlug)
	if err != nil {
		zap.L().Error("failed to mark lesson complete",
			zap.String("courseSlug", courseSlug), zap.String("lessonSlug", lessonSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to mark lesson as complete")

		return false
	}

	return true
}

// notifyCourseComplete notifies the user-service that userID completed
// course. Failures are only logged: the lesson itself was already
// recorded as complete, so they must not affect the HTTP response.
func (s *State) notifyCourseComplete(ctx context.Context, course *content.Course, userID string) {
	courseSlug := course.Slug

	skillMap := make(map[string]struct{}, len(course.Skills))
	for _, sk := range course.Skills {
		skillMap[sk] = struct{}{}
	}

	payload := CourseCompletion{
		UserID:     userID,
		CourseSlug: courseSlug,
		Skills:     skillMap,
		Difficulty: course.Difficulty,
		// One grouped query over the denormalized course skills replaces
		// scanning the whole catalog to count who teaches what.
		SkillTotalCourses: s.skillTotalsFor(ctx, course),
	}

	err := s.users().MarkCourseComplete(ctx, payload)
	if err != nil {
		zap.L().Error("failed to mark course complete",
			zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
	}
}

// MarkLessonComplete godoc
// @Summary   Mark a lesson as complete
// @Tags      Lessons
// @Security  BearerAuth
// @Produce   json
// @Param     slug         path  string  true  "Course slug"
// @Param     lessonSlug  path  string  true  "Lesson slug"
// @Success   200   {object}  map[string]string
// @Failure   403   {object}  map[string]string
// @Failure   404   {object}  map[string]string
// @Router    /api/courses/{slug}/lessons/{lessonSlug}/complete [post].
func (s *State) MarkLessonComplete(writer http.ResponseWriter, req *http.Request) {
	lessonSlug := param(req, "lessonSlug")

	view, ok := s.learnerView(writer, req, "Not enrolled")
	if !ok {
		return
	}

	_, _, found := findModuleLesson(view.modules, lessonSlug)
	if !found {
		s.Error(writer, http.StatusNotFound, "Lesson not found")

		return
	}

	if !s.postLessonComplete(writer, req, view.userID, view.slug, lessonSlug) {
		return
	}

	// The progress loaded at the start of this request predates the write
	// just made, which is exactly what the justViewedLesson argument is
	// for — no second read is needed to credit it.
	if courseCompleted(view, "", lessonSlug) {
		s.notifyCourseComplete(req.Context(), view.course, view.userID)
	}

	s.JSON(writer, http.StatusOK, map[string]string{messageJSONKey: lessonCompleteMessage})
}
