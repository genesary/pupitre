package handlers

// courseCompleteLessonSlug is the sentinel lesson slug used to mark an
// entire course as complete (for courses with no individual lessons to
// track, e.g. quiz-only courses).
const courseCompleteLessonSlug = "__complete__"

// CourseCompletion is a completed course, with the skill and difficulty
// metadata that update the learner's per-skill expertise level in one call.
type CourseCompletion struct {
	UserID            string              `json:"userId"`
	CourseSlug        string              `json:"courseSlug"`
	Skills            map[string]struct{} `json:"skills,omitempty"`
	Difficulty        string              `json:"difficulty,omitempty"`
	SkillTotalCourses map[string]int      `json:"skillTotalCourses,omitempty"`
}

// ModuleResult is a quiz or lab result.
type ModuleResult struct {
	UserID string `json:"userId"`

	CourseSlug string `json:"courseSlug"`

	ModuleIndex int `json:"moduleIndex"`

	ModuleSlug string `json:"moduleSlug"`
	ModuleType string `json:"moduleType"`
	Score      int    `json:"score"`

	MaxScore int  `json:"maxScore"`
	Passed   bool `json:"passed"`
}

// moduleProgressRow is one module of a learner's progress in a course.
type moduleProgressRow struct {
	ModuleIndex int `json:"moduleIndex"`

	BestScore int `json:"bestScore"`

	MaxScore int  `json:"maxScore"`
	Passed   bool `json:"passed"`
	Attempts int  `json:"attempts"`
}
