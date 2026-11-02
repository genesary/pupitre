package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
)

// publicAnswer is the client-facing representation of a quiz answer option;
// whether it is correct is only populated for admin requesters.
type publicAnswer struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Correct bool   `json:"correct,omitempty"`
}

// publicQuestion is the client-facing representation of a quiz question,
// stripped of internal scoring details unless the requester is an admin.
type publicQuestion struct {
	ID             string                  `json:"id"`
	Type           string                  `json:"type"`
	Difficulty     string                  `json:"difficulty,omitempty"`
	Points         int                     `json:"points"`
	Question       string                  `json:"question"`
	Answers        []publicAnswer          `json:"answers,omitempty"`
	Items          []content.OrderItem     `json:"items,omitempty"`
	PartialScoring *content.PartialScoring `json:"partialScoring,omitempty"`
	SourceRefs     []content.SourceRef     `json:"sourceRefs,omitempty"`
}

// sanitizeQuestions converts internal quiz questions into their public,
// client-safe representation, revealing correct answers only when admin
// is true.
func sanitizeQuestions(qq []content.Question, admin bool) []any {
	out := make([]any, len(qq))
	for index, question := range qq {
		pub := publicQuestion{
			ID:             question.ID,
			Type:           question.Type,
			Difficulty:     question.Difficulty,
			Points:         question.Points,
			Question:       question.Question,
			Items:          question.Items,
			PartialScoring: question.PartialScoring,
			SourceRefs:     question.Feedback.SourceRefs,
		}
		for _, a := range question.Answers {
			pa := publicAnswer{ID: a.ID, Text: a.Text}
			if admin {
				pa.Correct = a.Correct
			}

			pub.Answers = append(pub.Answers, pa)
		}

		out[index] = pub
	}

	return out
}

// inlineQuizResponse is the client-facing representation of a quiz module
// embedded inline immediately after the lesson module that precedes it.
type inlineQuizResponse struct {
	Index         int         `json:"index"`
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Questions     []any       `json:"questions,omitempty"`
	QuestionCount int         `json:"questionCount"`
	PassingScore  int         `json:"passingScore"`
	BestScore     int         `json:"bestScore,omitempty"`
	MaxScore      int         `json:"maxScore,omitempty"`
	Passed        bool        `json:"passed"`
	Attempts      int         `json:"attempts,omitempty"`
	QuizConfig    *quizConfig `json:"quizConfig,omitempty"`
}

// moduleResponse is the client-facing representation of a single course
// module: its content, the caller's progress, and (for admins only) the
// underlying source location.
type moduleResponse struct {
	Index         int                      `json:"index"`
	Name          string                   `json:"name"`
	Slug          string                   `json:"slug"`
	Type          string                   `json:"type"`
	Content       string                   `json:"content,omitempty"`
	Viewed        bool                     `json:"viewed"`
	Hidden        bool                     `json:"hidden"`
	Locked        bool                     `json:"locked"`
	Prerequisites []string                 `json:"prerequisites,omitempty"`
	BestScore     int                      `json:"bestScore,omitempty"`
	MaxScore      int                      `json:"maxScore,omitempty"`
	Passed        bool                     `json:"passed,omitempty"`
	Attempts      int                      `json:"attempts,omitempty"`
	QuestionCount int                      `json:"questionCount,omitempty"`
	Questions     []any                    `json:"questions,omitempty"`
	QuizConfig    *quizConfig              `json:"quizConfig,omitempty"`
	Cooldowns     map[string]cooldownState `json:"cooldowns,omitempty"`
	Inline        bool                     `json:"inline,omitempty"`
	InlineQuiz    *inlineQuizResponse      `json:"inlineQuiz,omitempty"`
	HasCheck      bool                     `json:"hasCheck,omitempty"`
	LabURL        string                   `json:"labUrl,omitempty"`
	CheckProvider string                   `json:"checkProvider,omitempty"`
	CheckType     string                   `json:"checkType,omitempty"`
	CheckParams   map[string]any           `json:"checkParams,omitempty"`
	Steps         []content.CheckStep      `json:"steps,omitempty"`
	Skills        []string                 `json:"skills,omitempty"`
	// Admin-only fields (omitted for regular users)
	Src  string `json:"src,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Path string `json:"path,omitempty"`
}

// quizConfig describes a quiz's grading rules as exposed to the client.
type quizConfig struct {
	PassingScore           int      `json:"passingScore"`
	Cooldown               cooldown `json:"cooldown,omitempty"`
	MaxAttemptsPerQuestion *int     `json:"maxAttemptsPerQuestion,omitempty"`
	LockOnMaxAttempts      bool     `json:"lockOnMaxAttempts"`
}

// cooldown describes the backoff strategy applied between graded quiz
// attempts, as exposed to the client.
type cooldown struct {
	Strategy    string  `json:"strategy"`
	BaseSeconds int     `json:"baseSeconds"`
	Multiplier  float64 `json:"multiplier"`
	MaxSeconds  int     `json:"maxSeconds"`
}

// submitResponse is returned to the client after a quiz submission has been
// scored.
type submitResponse struct {
	TotalScore      int                      `json:"totalScore"`
	MaxScore        int                      `json:"maxScore"`
	Passed          bool                     `json:"passed"`
	QuestionResults []questionResultAPI      `json:"questionResults"`
	Cooldowns       map[string]cooldownState `json:"cooldowns,omitempty"`
}

// cooldownState reports the current cooldown/attempt state for one quiz
// question, as exposed to the client.
type cooldownState struct {
	RemainingSeconds int  `json:"remainingSeconds"`
	Attempts         int  `json:"attempts"`
	Locked           bool `json:"locked"`
}

// questionResultAPI is the client-facing result of scoring a single quiz
// question. It never carries the correct answer: a learner could read it,
// then submit again with it (docs/specs/190-backend-defects.md B17).
type questionResultAPI struct {
	QuestionID   string              `json:"questionId"`
	Type         string              `json:"type"`
	IsCorrect    bool                `json:"isCorrect"`
	PointsEarned int                 `json:"pointsEarned"`
	PointsMax    int                 `json:"pointsMax"`
	Feedback     string              `json:"feedback,omitempty"`
	SourceRefs   []content.SourceRef `json:"sourceRefs,omitempty"`
}

// moduleProgressData holds per-module quiz progress fetched from user-service.
type moduleProgressData struct {
	BestScore int
	MaxScore  int
	Passed    bool
	Attempts  int
}

// ModuleResult is the payload sent to user-service to record a quiz
// module's score. Field names mirror user-service's internal API contract
// (user-service/internal/handlers/internal.go).
type ModuleResult struct {
	UserID      string `json:"userId"`
	CourseSlug  string `json:"courseSlug"`
	ModuleIndex int    `json:"moduleIndex"`
	ModuleSlug  string `json:"moduleSlug"`
	ModuleType  string `json:"moduleType"`
	Score       int    `json:"score"`
	MaxScore    int    `json:"maxScore"`
	Passed      bool   `json:"passed"`
}

// recordModuleProgress sends quiz results to user-service asynchronously.
//
// The call is detached from the request lifecycle on purpose: the HTTP
// response has already been (or is about to be) sent, so it runs on a
// fresh, independently-timed context rather than the request's, which may
// already be canceled by the time the goroutine runs.
func (s *State) recordModuleProgress(courseSlug, userID, moduleSlug, moduleType string, idx, score, maxScore int, passed bool) {
	result := ModuleResult{
		UserID:      userID,
		CourseSlug:  courseSlug,
		ModuleIndex: idx,
		ModuleSlug:  moduleSlug,
		ModuleType:  moduleType,
		Score:       score,
		MaxScore:    maxScore,
		Passed:      passed,
	}

	s.detached("record-module", func(ctx context.Context, users UserProgress) error {
		return users.RecordModule(ctx, result)
	}, zap.String("courseSlug", courseSlug), zap.String("moduleSlug", moduleSlug))
}

// completedSlugs builds a set of module slugs the user has completed
// (viewed or passed). modules is the visible module list; viewedMap comes
// from viewedLessons; progressMap comes from fetchModuleProgress.
func completedSlugs(modules []content.Module, viewedMap map[string]bool, progressMap map[int]moduleProgressData) map[string]bool {
	out := make(map[string]bool)
	for slug := range viewedMap {
		out[slug] = true
	}

	for idx, p := range progressMap {
		if p.Passed && idx >= 0 && idx < len(modules) {
			out[modules[idx].Slug()] = true
		}
	}

	return out
}

// isLocked returns true if any prerequisite module slug has not been completed.
func isLocked(prereqs []string, done map[string]bool) bool {
	for _, slug := range prereqs {
		if !done[slug] {
			return true
		}
	}

	return false
}

// coursePrereqSummary holds the total score, passed module slugs, and viewed
// lesson count for a user in a specific course — used to evaluate cross-course
// prerequisites.
type coursePrereqSummary struct {
	TotalScore    int
	PassedModules map[string]bool
	ViewedCount   int // number of lessons marked complete (text/video/image)
}

// emptyCoursePrereqSummary is returned by fetchCoursePrereqSummary whenever
// the user-service call fails, so callers can treat it as "no progress"
// without special-casing errors.
func emptyCoursePrereqSummary() coursePrereqSummary {
	return coursePrereqSummary{PassedModules: map[string]bool{}}
}

// checkCoursePrerequisites returns false if any course-level prerequisite
// declared on the course is not yet satisfied by the user:
//   - If MinScore > 0: the sum of the user's best scores in the prereq
//     course must be >= MinScore.
//   - If Modules is non-empty: every listed module slug must be passed
//     in the prereq course.
//   - If neither is set: only the existence of any score is required
//     (any attempt).
//
// Every prerequisite's progress is fetched in one batched call. Asking
// user-service once per prerequisite made the cost of opening a course
// grow with the length of its prerequisite chain, in serial round-trips.
func (s *State) checkCoursePrerequisites(ctx context.Context, prereqs []content.CoursePrerequisite, userID string) bool {
	slugs := make([]string, 0, len(prereqs))
	for _, prereq := range prereqs {
		slugs = append(slugs, prereq.Course)
	}

	summaries := s.courseSummaries(ctx, userID, slugs)

	for _, prereq := range prereqs {
		summary, found := summaries[prereq.Course]
		if !found {
			summary = emptyCoursePrereqSummary()
		}

		if !prerequisiteSatisfied(prereq, summary) {
			return false
		}
	}

	return true
}

// prerequisiteSatisfied reports whether one course prerequisite is met by
// the learner's recorded progress in that course.
func prerequisiteSatisfied(prereq content.CoursePrerequisite, summary coursePrereqSummary) bool {
	if prereq.MinScore > 0 && summary.TotalScore < prereq.MinScore {
		return false
	}

	for _, modSlug := range prereq.Modules {
		if !summary.PassedModules[modSlug] {
			return false
		}
	}

	// If neither MinScore nor Modules are set, require at least some progress
	// (either quiz score or viewed lessons — covers text-only courses).
	return prereq.MinScore > 0 || len(prereq.Modules) > 0 || summary.TotalScore > 0 || summary.ViewedCount > 0
}

// ListModules godoc
// @Summary   List modules for a course
// @Tags      Modules
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Failure   404   {object}  map[string]string
// @Router    /api/courses/{slug}/modules [get].
func (s *State) ListModules(writer http.ResponseWriter, req *http.Request) {
	view, ok := s.learnerView(writer, req, "Enroll in this course to access it")
	if !ok {
		return
	}

	done := completedSlugs(view.modules, view.progress.Viewed, view.progress.Modules)

	out := make([]moduleResponse, 0, len(view.modules))
	prevDone := true // first module is always available

	for idx, mod := range view.modules {
		explicitLocked := isLocked(mod.Prerequisites, done)
		seqLocked := !mod.Inline && !prevDone
		locked := !view.isAdmin && (explicitLocked || seqLocked)

		out = append(out, buildModuleListEntry(mod, idx, view.isAdmin, locked, view.progress.Viewed, view.progress.Modules))

		if !mod.Inline {
			prevDone = done[mod.Slug()]
		}
	}

	s.JSON(writer, http.StatusOK, map[string]any{"modules": out})
}

// learnerView is everything a module or lesson handler needs about one
// learner and one course, loaded once per request: the course and its
// visible modules, and the learner's progress in it.
type learnerView struct {
	course   *content.Course
	modules  []content.Module
	progress *courseProgress
	claims   *middleware.Claims
	isAdmin  bool
	userID   string
	slug     string
}

// learnerView loads the course named by the request's slug, the learner's
// progress in it, and enforces access, writing the appropriate error
// response and reporting ok=false when the request must not proceed.
//
// Loading progress up front is what collapses the four internal round-trips
// these handlers used to make — enrollment check, viewed lessons, module
// progress, course summary — into one. The enrollment answer arrives with
// the progress, so checking access costs nothing extra.
func (s *State) learnerView(writer http.ResponseWriter, req *http.Request, deniedMsg string) (*learnerView, bool) {
	courseSlug := param(req, "slug")

	claims := s.claims(req)
	if claims == nil {
		s.Error(writer, http.StatusUnauthorized, "Unauthorized")

		return nil, false
	}

	course, found := s.course(writer, req, courseSlug)
	if !found {
		return nil, false
	}

	view := &learnerView{
		course:   course,
		modules:  s.visibleModules(course, req),
		progress: s.courseProgress(req.Context(), courseSlug, claims.Subject),
		claims:   claims,
		isAdmin:  claims.Role == roleAdmin,
		userID:   claims.Subject,
		slug:     courseSlug,
	}

	if !s.ensureModuleAccess(writer, req, view, deniedMsg) {
		return nil, false
	}

	return view, true
}

// ensureModuleAccess auto-enrolls the user in public courses, verifies
// enrollment for private ones, and checks cross-course prerequisites. It
// writes an error response and returns false if access should be denied.
// Admins bypass all of these checks.
func (s *State) ensureModuleAccess(writer http.ResponseWriter, req *http.Request, view *learnerView, deniedMsg string) bool {
	if view.isAdmin {
		return true
	}

	switch {
	case view.course.IsPublic && !view.progress.Enrolled:
		s.autoEnroll(view.userID, view.slug) //nolint:contextcheck // fire-and-forget enrollment, deliberately detached from the request context
	case view.course.IsPublic:
	case !view.progress.Enrolled:
		s.Error(writer, http.StatusForbidden, deniedMsg)

		return false
	}

	if len(view.course.Prerequisites) > 0 &&
		!s.checkCoursePrerequisites(req.Context(), view.course.Prerequisites, view.userID) {
		s.Error(writer, http.StatusForbidden, "Complete prerequisite courses first")

		return false
	}

	return true
}

// resolveModuleIndex parses and bounds-checks a module index path param
// against the number of visible modules in the course.
func resolveModuleIndex(indexStr string, moduleCount int) (int, bool) {
	idx, err := strconv.Atoi(indexStr)
	if err != nil || idx < 0 || idx >= moduleCount {
		return 0, false
	}

	return idx, true
}

// buildModuleListEntry assembles the per-module summary returned by
// ListModules, applying visibility rules for admin-only fields and lab URLs.
func buildModuleListEntry(mod content.Module, idx int, isAdmin, locked bool, viewed map[string]bool, progress map[int]moduleProgressData) moduleResponse {
	prog := progress[idx]

	resp := moduleResponse{
		Index:         idx,
		Name:          mod.Name,
		Slug:          mod.Slug(),
		Type:          mod.Type,
		Viewed:        viewed[mod.Slug()],
		Hidden:        mod.Hidden,
		Inline:        mod.Inline,
		Locked:        locked,
		Prerequisites: mod.Prerequisites,
		BestScore:     prog.BestScore,
		MaxScore:      prog.MaxScore,
		Passed:        prog.Passed,
		Attempts:      prog.Attempts,
		Skills:        mod.Skills,
	}
	if isAdmin {
		resp.Src = mod.Src
		resp.Ref = mod.Ref
		resp.Path = mod.Path
	}
	// Lab modules expose their URLs to everyone — the frontend
	// opens labUrl in a new tab (student-facing GitLab link).
	if mod.Type == moduleTypeLab && !locked {
		resp.Src = mod.Src
		resp.Ref = mod.Ref
		resp.Path = mod.Path
		resp.LabURL = mod.LabURL
	}

	if mod.Type == moduleTypeLab && mod.HasGitContent() {
		resp.HasCheck = true
	}

	if mod.CheckProvider == checkProviderLocal {
		resp.HasCheck = true
		resp.CheckProvider = mod.CheckProvider
	}

	if mod.Type == moduleTypeQuiz {
		if mod.HasQuestions() {
			resp.QuestionCount = len(mod.Questions)
		} else if mod.HasGitContent() {
			resp.QuestionCount = 0 // unknown until fetched
		}
	}

	return resp
}

// GetModule godoc
// @Summary   Get a module by index
// @Tags      Modules
// @Security  BearerAuth
// @Produce   json
// @Param     slug   path  string  true  "Course slug"
// @Param     index  path  int     true  "Module index (0-based)"
// @Success   200    {object}  moduleResponse
// @Failure   404    {object}  map[string]string
// @Router    /api/courses/{slug}/modules/{index} [get].
func (s *State) GetModule(writer http.ResponseWriter, req *http.Request) {
	view, ok := s.learnerView(writer, req, "Enroll in this course to access it")
	if !ok {
		return
	}

	idx, valid := resolveModuleIndex(param(req, "index"), len(view.modules))
	if !valid {
		s.Error(writer, http.StatusNotFound, moduleNotFoundMessage)

		return
	}

	mod := view.modules[idx]
	resp := buildModuleDetailResponse(mod, idx, view.progress)

	if !s.populateModuleContent(req.Context(), writer, &resp, mod, view.isAdmin, view.userID, view.slug, idx) {
		return
	}

	// If the next module is an inline quiz, embed it in the response.
	resp.InlineQuiz = s.buildInlineQuizResponse(req.Context(), view, idx)

	s.JSON(writer, http.StatusOK, resp)
}

// buildModuleDetailResponse assembles the base module detail fields — quiz
// question count, viewed state, and check metadata — shared by GetModule
// before type-specific content is resolved.
func buildModuleDetailResponse(mod content.Module, idx int, progress *courseProgress) moduleResponse {
	resp := moduleResponse{
		Index:  idx,
		Name:   mod.Name,
		Slug:   mod.Slug(),
		Type:   mod.Type,
		Hidden: mod.Hidden,
		Inline: mod.Inline,
		Skills: mod.Skills,
		Viewed: progress.Viewed[mod.Slug()],
	}
	if mod.Type == moduleTypeQuiz && mod.HasQuestions() {
		resp.QuestionCount = len(mod.Questions)
	}

	// Lab modules with git content support the /check endpoint.
	if mod.Type == moduleTypeLab && mod.HasGitContent() {
		resp.HasCheck = true
		resp.LabURL = mod.LabURL
	}
	// Local check modules (Tauri) expose check meta to the frontend.
	if mod.CheckProvider == checkProviderLocal {
		resp.HasCheck = true
		resp.CheckProvider = mod.CheckProvider
		resp.CheckType = mod.CheckType
		resp.CheckParams = mod.CheckParams
		resp.Steps = mod.Steps
	}
	// GitLab step-based labs expose steps so the frontend can render per-step buttons.
	if mod.CheckProvider == checkProviderGitLab && len(mod.Steps) > 0 {
		resp.CheckProvider = mod.CheckProvider
		resp.Steps = mod.Steps
	}

	return resp
}

// populateModuleContent resolves and attaches a module's body content
// (replicated media path, lab/text markdown, or quiz questions) onto resp.
// It writes an error response and returns false if content resolution fails.
func (s *State) populateModuleContent(ctx context.Context, writer http.ResponseWriter, resp *moduleResponse, mod content.Module, isAdmin bool, userID, courseSlug string, idx int) bool {
	switch mod.Type {
	case moduleTypeVideo, moduleTypeImage:
		resp.Content = content.ReplicatedPath(ctx, mod, s.Config.UploadsDir)
	case moduleTypeLab:
		text, err := s.moduleGitOrInlineContent(ctx, mod)
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Failed to fetch lab content")

			return false
		}

		resp.Content = text
	case moduleTypeText:
		text, err := s.moduleGitOrInlineContent(ctx, mod)
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Failed to fetch module content")

			return false
		}

		resp.Content = text
	case moduleTypeQuiz:
		if !s.populateQuizContent(ctx, writer, resp, mod, isAdmin, userID, courseSlug, idx) {
			return false
		}
	}

	return true
}

// moduleGitOrInlineContent returns a module's markdown body, fetching it from
// the git source when the module references external content.
func (s *State) moduleGitOrInlineContent(ctx context.Context, mod content.Module) (string, error) {
	if !mod.HasGitContent() {
		return mod.Content(), nil
	}

	data, err := s.GitCache.FetchModuleContent(ctx, mod.Src, mod.Ref, mod.Path, s.tokenForRepo(mod.Src))
	if err != nil {
		return "", fmt.Errorf("fetch module content: %w", err)
	}

	return string(data), nil
}

// populateQuizContent resolves quiz questions (from git or inline), attaches
// the sanitized questions/config to resp, and reports current per-question
// cooldowns. It writes an error response and returns false on fetch failure.
func (s *State) populateQuizContent(ctx context.Context, writer http.ResponseWriter, resp *moduleResponse, mod content.Module, isAdmin bool, userID, courseSlug string, idx int) bool {
	var quizQuestions []content.Question

	switch {
	case mod.HasGitContent():
		quiz, err := content.FetchQuizContent(ctx, s.GitCache, mod.Src, mod.Ref, mod.Path, s.tokenForRepo(mod.Src))
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Failed to fetch quiz content")

			return false
		}

		quizQuestions = quiz.Questions
		resp.Questions = sanitizeQuestions(quiz.Questions, isAdmin)
		resp.QuizConfig = &quizConfig{
			PassingScore:           quiz.PassingScore,
			Cooldown:               cooldown(quiz.Cooldown),
			MaxAttemptsPerQuestion: quiz.MaxAttemptsPerQuestion,
			LockOnMaxAttempts:      quiz.LockOnMaxAttempts,
		}
	case mod.HasQuestions():
		quizQuestions = mod.Questions
		resp.Questions = sanitizeQuestions(mod.Questions, isAdmin)
		resp.QuizConfig = &quizConfig{
			PassingScore:           mod.PassingScore,
			Cooldown:               cooldown(mod.Cooldown),
			MaxAttemptsPerQuestion: mod.MaxAttemptsPerQuestion,
			LockOnMaxAttempts:      mod.LockOnMaxAttempts,
		}
	}

	resp.Content = ""

	// Include current cooldown state so frontend persists it across refreshes.
	if len(quizQuestions) > 0 {
		if cooldowns := s.moduleCooldowns(ctx, quizQuestions, userID, courseSlug, idx); len(cooldowns) > 0 {
			resp.Cooldowns = cooldowns
		}
	}

	return true
}

// attemptKey builds the persisted-attempt key identifying one learner's
// state on one quiz module.
func attemptKey(userID, courseSlug string, idx int) repository.AttemptKey {
	return repository.AttemptKey{Username: userID, CourseSlug: courseSlug, ModuleIndex: idx}
}

// questionIDs extracts the IDs of the given questions, so attempt state
// for a whole quiz can be read or written in one statement.
func questionIDs(questions []content.Question) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}

	return ids
}

// moduleCooldowns returns the current cooldown/attempt state for each
// question in quizQuestions that has an active cooldown or recorded
// attempts, reading the whole quiz's state in a single query.
func (s *State) moduleCooldowns(ctx context.Context, quizQuestions []content.Question, userID, courseSlug string, idx int) map[string]cooldownState {
	key := attemptKey(userID, courseSlug, idx)

	states, err := s.Repos.QuizAttempts.States(ctx, key, questionIDs(quizQuestions))
	if err != nil {
		zap.L().Error("load quiz cooldowns failed",
			zap.String("courseSlug", courseSlug), zap.Int("moduleIndex", idx), zap.Error(err))

		return nil
	}

	cooldowns := make(map[string]cooldownState, len(states))

	for id, state := range states {
		if state.Remaining <= 0 && state.Attempts == 0 {
			continue
		}

		cooldowns[id] = cooldownState{
			RemainingSeconds: int(state.Remaining.Seconds()),
			Attempts:         state.Attempts,
			Locked:           false,
		}
	}

	return cooldowns
}

// buildInlineQuizResponse embeds the following module's quiz inline in the
// response when that module is marked Inline — this lets the frontend render
// a lesson immediately followed by its check-for-understanding quiz.
func (s *State) buildInlineQuizResponse(ctx context.Context, view *learnerView, idx int) *inlineQuizResponse {
	if idx+1 >= len(view.modules) {
		return nil
	}

	next := view.modules[idx+1]
	if next.Type != moduleTypeQuiz || !next.Inline {
		return nil
	}

	inlineQuiz := &inlineQuizResponse{
		Index:        idx + 1,
		Name:         next.Name,
		Slug:         next.Slug(),
		PassingScore: next.PassingScore,
	}

	quizQuestions := s.populateInlineQuizQuestions(ctx, inlineQuiz, next, view.isAdmin)
	if len(quizQuestions) == 0 {
		return nil
	}

	inlineQuiz.QuestionCount = len(quizQuestions)

	maxScore := 0
	for _, q := range quizQuestions {
		maxScore += q.Points
	}

	inlineQuiz.MaxScore = maxScore

	// The learner's progress was loaded once for the whole request; the
	// inline quiz is just another entry in it, not a second lookup.
	if p, ok := view.progress.Modules[idx+1]; ok {
		inlineQuiz.BestScore = p.BestScore
		inlineQuiz.Attempts = p.Attempts
		inlineQuiz.Passed = p.Passed
	}

	return inlineQuiz
}

// populateInlineQuizQuestions resolves next's quiz questions (from git or
// inline) into inlineQuiz and returns them for scoring/metadata purposes.
func (s *State) populateInlineQuizQuestions(ctx context.Context, inlineQuiz *inlineQuizResponse, next content.Module, isAdmin bool) []content.Question {
	switch {
	case next.HasGitContent():
		quiz, err := content.FetchQuizContent(ctx, s.GitCache, next.Src, next.Ref, next.Path, s.tokenForRepo(next.Src))
		if err != nil {
			return nil
		}

		inlineQuiz.Questions = sanitizeQuestions(quiz.Questions, isAdmin)
		inlineQuiz.PassingScore = quiz.PassingScore
		inlineQuiz.QuizConfig = &quizConfig{
			PassingScore:           quiz.PassingScore,
			Cooldown:               cooldown(quiz.Cooldown),
			MaxAttemptsPerQuestion: quiz.MaxAttemptsPerQuestion,
			LockOnMaxAttempts:      quiz.LockOnMaxAttempts,
		}

		return quiz.Questions
	case next.HasQuestions():
		inlineQuiz.Questions = sanitizeQuestions(next.Questions, isAdmin)
		inlineQuiz.QuizConfig = &quizConfig{
			PassingScore:           next.PassingScore,
			Cooldown:               cooldown(next.Cooldown),
			MaxAttemptsPerQuestion: next.MaxAttemptsPerQuestion,
			LockOnMaxAttempts:      next.LockOnMaxAttempts,
		}

		return next.Questions
	default:
		return nil
	}
}

// SubmitModule godoc
// @Summary   Submit quiz answers for a module
// @Tags      Modules
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug   path  string                true  "Course slug"
// @Param     index  path  int                   true  "Module index (0-based)"
// @Param     body   body  content.SubmitRequest  true  "Quiz answers"
// @Success   200    {object}  submitResponse
// @Failure   400    {object}  map[string]string
// @Failure   404    {object}  map[string]string
// @Router    /api/courses/{slug}/modules/{index}/submit [post].
func (s *State) SubmitModule(writer http.ResponseWriter, req *http.Request) {
	view, ok := s.learnerView(writer, req, "Enroll in this course to access it")
	if !ok {
		return
	}

	idx, indexValid := resolveModuleIndex(param(req, "index"), len(view.modules))
	if !indexValid {
		s.Error(writer, http.StatusNotFound, moduleNotFoundMessage)

		return
	}

	mod := view.modules[idx]
	if mod.Type != moduleTypeQuiz {
		s.Error(writer, http.StatusBadRequest, "Module is not a quiz")

		return
	}

	// Resolve questions: fetch from git repo first, then fallback to inline.
	questions, passingScore, cooldownSpec, quizResolved := s.resolveSubmitQuiz(req.Context(), writer, mod)
	if !quizResolved {
		return
	}

	s.finalizeSubmission(writer, req, view, mod, questions, passingScore, cooldownSpec, idx)
}

// finalizeSubmission decodes a quiz submission body, rejects it if any of
// the resolved questions is still under a cooldown, then scores it, records
// per-question cooldowns and overall module progress, and writes the
// resulting submitResponse to the client. When passing this quiz was the
// last thing the course was waiting for, it also notifies user-service that
// the course is complete.
func (s *State) finalizeSubmission(writer http.ResponseWriter, req *http.Request, view *learnerView, mod content.Module, questions []content.Question, passingScore int, cooldownSpec content.CooldownSpec, idx int) {
	userID, courseSlug := view.userID, view.slug

	submission, accepted := s.acceptSubmission(writer, req, questions, userID, courseSlug, idx)
	if !accepted {
		return
	}

	effectiveQuiz := &content.Quiz{
		Questions:    questions,
		PassingScore: passingScore,
	}

	result := content.ScoreQuiz(effectiveQuiz, submission.Answers)

	// Record cooldowns for wrong answers.
	respCooldowns := s.recordQuestionCooldowns(req.Context(), userID, courseSlug, idx,
		result.QuestionResults, cooldownSpec, mod.MaxAttemptsPerQuestion, mod.LockOnMaxAttempts)

	s.recordModuleProgress(courseSlug, userID, mod.Slug(), mod.Type, idx, result.TotalScore, result.MaxScore, result.Passed) //nolint:contextcheck // fire-and-forget async POST detached from the request context by design

	if result.Passed && courseCompleted(view, mod.Slug(), "") {
		s.notifyCourseComplete(req.Context(), view.course, userID)
	}

	s.JSON(writer, http.StatusOK, submitResponse{
		TotalScore:      result.TotalScore,
		MaxScore:        result.MaxScore,
		Passed:          result.Passed,
		QuestionResults: toQuestionResultsAPI(result.QuestionResults),
		Cooldowns:       respCooldowns,
	})
}

// acceptSubmission decodes a quiz submission and rejects it when any of the
// questions is still under a cooldown. It writes the error response itself
// and reports ok=false when the submission must not be scored.
func (s *State) acceptSubmission(
	writer http.ResponseWriter, req *http.Request,
	questions []content.Question, userID, courseSlug string, idx int,
) (content.SubmitRequest, bool) {
	var submission content.SubmitRequest

	err := json.NewDecoder(req.Body).Decode(&submission)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid request body")

		return submission, false
	}

	cooldowns, err := s.pendingCooldowns(req.Context(), userID, courseSlug, idx, questions)
	if err != nil {
		zap.L().Error("load quiz cooldowns failed",
			zap.String("courseSlug", courseSlug), zap.Int("moduleIndex", idx), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return submission, false
	}

	if len(cooldowns) > 0 {
		s.JSON(writer, http.StatusTooEarly, map[string]any{
			"error":     "Cooldown active for some questions",
			"cooldowns": cooldowns,
		})

		return submission, false
	}

	return submission, true
}

// toQuestionResultsAPI converts scored question results into their
// client-facing form.
func toQuestionResultsAPI(results []content.QuestionResult) []questionResultAPI {
	out := make([]questionResultAPI, len(results))
	for index, questionResult := range results {
		out[index] = questionResultAPI{
			QuestionID:   questionResult.QuestionID,
			Type:         questionResult.Type,
			IsCorrect:    questionResult.IsCorrect,
			PointsEarned: questionResult.PointsEarned,
			PointsMax:    questionResult.PointsMax,
			Feedback:     questionResult.Feedback,
			SourceRefs:   questionResult.SourceRefs,
		}
	}

	return out
}

// resolveSubmitQuiz resolves the questions, passing score, and cooldown spec
// for a quiz module, fetching from git when the module references external
// content and falling back to inline questions otherwise. It writes an error
// response and returns ok=false when no questions can be resolved.
func (s *State) resolveSubmitQuiz(ctx context.Context, writer http.ResponseWriter, mod content.Module) ([]content.Question, int, content.CooldownSpec, bool) {
	switch {
	case mod.HasGitContent():
		quiz, err := content.FetchQuizContent(ctx, s.GitCache, mod.Src, mod.Ref, mod.Path, s.tokenForRepo(mod.Src))
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Failed to fetch quiz content")

			return nil, 0, content.CooldownSpec{}, false
		}

		return quiz.Questions, quiz.PassingScore, quiz.Cooldown, true
	case mod.HasQuestions():
		return mod.Questions, mod.PassingScore, mod.Cooldown, true
	default:
		s.Error(writer, http.StatusNotFound, "No questions found for this quiz")

		return nil, 0, content.CooldownSpec{}, false
	}
}

// pendingCooldowns returns any active per-question cooldowns for userID on
// this quiz module, which should block a resubmission attempt. The whole
// quiz's state is read in one query.
//
// It reports an error rather than an empty map when the lookup fails, so a
// database problem cannot silently wave a submission past its cooldown.
func (s *State) pendingCooldowns(ctx context.Context, userID, courseSlug string, idx int, questions []content.Question) (map[string]cooldownState, error) {
	key := attemptKey(userID, courseSlug, idx)

	states, err := s.Repos.QuizAttempts.States(ctx, key, questionIDs(questions))
	if err != nil {
		return nil, fmt.Errorf("load quiz attempts: %w", err)
	}

	cooldowns := make(map[string]cooldownState)

	for id, state := range states {
		if state.Remaining > 0 {
			cooldowns[id] = cooldownState{
				RemainingSeconds: int(state.Remaining.Seconds()),
				Attempts:         state.Attempts,
			}
		}
	}

	return cooldowns, nil
}

// recordQuestionCooldowns records a cooldown for every incorrectly answered
// question, clears any existing cooldown for the correct ones, and returns
// the resulting per-question cooldown state to report back to the client.
//
// Both halves are batched: one delete for the correct answers and one
// upsert for the wrong ones, however many questions the quiz has.
func (s *State) recordQuestionCooldowns(
	ctx context.Context, userID, courseSlug string, idx int,
	results []content.QuestionResult, spec content.CooldownSpec, maxAttempts *int, lockOnMax bool,
) map[string]cooldownState {
	key := attemptKey(userID, courseSlug, idx)

	var passed, failed []string

	for _, questionResult := range results {
		if questionResult.IsCorrect {
			passed = append(passed, questionResult.QuestionID)
		} else {
			failed = append(failed, questionResult.QuestionID)
		}
	}

	err := s.Repos.QuizAttempts.Clear(ctx, key, passed)
	if err != nil {
		zap.L().Error("clear quiz cooldowns failed",
			zap.String("courseSlug", courseSlug), zap.Int("moduleIndex", idx), zap.Error(err))
	}

	recorded, err := s.Repos.QuizAttempts.RecordFailures(ctx, key, failed, spec, maxAttempts, lockOnMax)
	if err != nil {
		zap.L().Error("record quiz cooldowns failed",
			zap.String("courseSlug", courseSlug), zap.Int("moduleIndex", idx), zap.Error(err))

		return nil
	}

	respCooldowns := make(map[string]cooldownState, len(recorded))
	for id, result := range recorded {
		respCooldowns[id] = cooldownState{
			RemainingSeconds: int(result.Remaining.Seconds()),
			Attempts:         result.Attempts,
			Locked:           result.Locked,
		}
	}

	return respCooldowns
}

// courseCompleted returns true when the learner has finished every module
// of the course: quiz modules have to be passed, all others viewed. Hidden
// modules are ignored — learners never see them, so they cannot hold a
// course back.
//
// A course is only complete once the trailing quiz is passed, and passing it
// completes the course whatever order the modules were taken in: neither the
// last lesson nor the quiz is special-cased.
//
// justPassedQuiz and justViewedLesson name the module completed by the
// current request. Its progress write is either fire-and-forget
// (recordModuleProgress) or racing this read, so it is credited here without
// a round-trip. Pass an empty string for the kind the request did not touch.
//
// It reads the progress the request already loaded rather than fetching it
// again — which is what makes the two writes that call it (marking a lesson
// complete, passing a quiz) cost one internal call each instead of three.
func courseCompleted(view *learnerView, justPassedQuiz, justViewedLesson string) bool {
	for _, mod := range view.modules {
		if mod.Hidden {
			continue
		}

		slug := mod.Slug()

		if mod.Type == moduleTypeQuiz {
			if slug != justPassedQuiz && !view.progress.PassedModules[slug] {
				return false
			}

			continue
		}

		if slug != justViewedLesson && !view.progress.Viewed[slug] {
			return false
		}
	}

	return true
}
