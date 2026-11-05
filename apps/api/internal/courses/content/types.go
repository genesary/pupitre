package content

import (
	"slices"
	"strings"
	"unicode"
)

// Module type identifiers used across course content and quiz handling.
const (
	moduleTypeVideo = "video"
	moduleTypeImage = "image"
	moduleTypeLab   = "lab"
	moduleTypeQuiz  = "quiz"

	// ModuleTypeText is the module type assumed when a module omits one.
	ModuleTypeText = "text"
)

// DifficultyMedium is the default question/course difficulty, assumed when
// a question omits one.
const DifficultyMedium = "medium"

// Path is a sequential learning path composed of course slugs or skill tags.
type Path struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Kind        string   `json:"kind"`              // "course" | "skill"
	Level       string   `json:"level,omitempty"`   // "beginner" | "intermediate" | "advanced"
	Courses     []string `json:"courses,omitempty"` // ordered — index N+1 unlocks after N completed
	Skills      []string `json:"skills,omitempty"`  // ordered skill tags (kind=skill paths)
}

// CoursePrerequisite describes one condition to be met before enrolling.
// If only Course is set, enrollment in that course is required.
// If MinScore > 0, the user must have earned at least that many points.
// If Modules is non-empty, every listed module slug must be passed.
type CoursePrerequisite struct {
	Course string `json:"course" yaml:"course"`

	MinScore int      `json:"minScore,omitempty" yaml:"minScore,omitempty"`
	Modules  []string `json:"modules,omitempty"  yaml:"modules,omitempty"`
}

// Badge is the reward earned when a learner completes a course.
type Badge struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// Session describes one scheduled in-person session of a course.
type Session struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Location string `json:"location,omitempty"`
	Capacity int    `json:"capacity,omitempty"`
}

// Course is the domain representation of a course, assembled from the
// courses/course_modules/course_prerequisites/course_sessions tables.
//
// Modules is only populated by the repository reads that ask for it —
// catalog listings leave it nil and report the module count via
// ModuleCount instead, so listing never has to read module rows.
type Course struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Difficulty  string `json:"difficulty"`

	IsPublic      bool                 `json:"isPublic"`
	Hidden        bool                 `json:"hidden,omitempty"`
	Prerequisites []CoursePrerequisite `json:"prerequisites,omitempty"`
	Modules       []Module             `json:"modules,omitempty"`
	ModuleCount   int                  `json:"moduleCount"`
	Skills        []string             `json:"skills,omitempty"` // aggregated from all modules
	Scope         string               `json:"scope,omitempty"`
	Badge         *Badge               `json:"badge,omitempty"`
	InPerson      bool                 `json:"inPerson,omitempty"`
	Sessions      []Session            `json:"sessions,omitempty"`
	XPRequired    int                  `json:"xpRequired,omitempty"`
}

// Module is a course element as stored in course_modules.
type Module struct {
	Name string `json:"name"`
	Type string `json:"type"` // video, text, image, quiz, lab
	Src  string `json:"src,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Path string `json:"path,omitempty"`

	LabURL        string     `json:"labUrl,omitempty"`
	InlineContent string     `json:"content,omitempty"`
	Replication   bool       `json:"replication,omitempty"`
	Hidden        bool       `json:"hidden,omitempty"`
	Inline        bool       `json:"inline,omitempty"`        // quiz rendered inside the previous module
	Prerequisites []string   `json:"prerequisites,omitempty"` // module slugs that must be completed first
	Questions     []Question `json:"questions,omitempty"`

	PassingScore int          `json:"passingScore,omitempty"`
	Cooldown     CooldownSpec `json:"cooldown,omitempty"`

	MaxAttemptsPerQuestion *int `json:"maxAttemptsPerQuestion,omitempty"`

	LockOnMaxAttempts bool `json:"lockOnMaxAttempts,omitempty"`

	CheckProvider string `json:"checkProvider,omitempty"` // "local" | "gitlab"

	CheckType string `json:"checkType,omitempty"`

	CheckParams map[string]any `json:"checkParams,omitempty"`
	Steps       []CheckStep    `json:"steps,omitempty"`
	Skills      []string       `json:"skills,omitempty"` // competency tags this module teaches
}

// CheckStep is one verifiable step inside a lab module.
type CheckStep struct {
	Title string `json:"title" yaml:"title"`

	CheckType string `json:"checkType" yaml:"checkType"`

	CheckParams map[string]any `json:"checkParams,omitempty" yaml:"checkParams,omitempty"`
}

// Slug returns a DNS-compliant slug derived from the module name.
func (m Module) Slug() string {
	return SlugifyModuleName(m.Name)
}

// SlugifyModuleName derives a module's DNS-compliant slug from its display
// name. The repository persists the result so lookups by slug do not have
// to recompute it for every module of a course.
func SlugifyModuleName(name string) string {
	var builder strings.Builder

	for _, c := range name {
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' || c == '_':
			builder.WriteRune(c)
		default:
			builder.WriteRune('-')
		}
	}

	return strings.ToLower(strings.Trim(builder.String(), "-"))
}

// Content resolves and returns the module content.
// video/image: returns Src, a direct URL.
// lab without inline content: returns Src, an external lab URL.
// text/quiz with git: needs FetchModuleContent (returns empty here, caller
// must fetch it separately).
func (m Module) Content() string {
	switch m.Type {
	case moduleTypeVideo, moduleTypeImage:
		return m.Src
	case moduleTypeLab:
		if m.InlineContent != "" {
			return m.InlineContent
		}

		return m.Src
	default:
		return m.InlineContent
	}
}

// HasQuestions returns true if the module has inline quiz questions.
func (m Module) HasQuestions() bool {
	return m.Type == moduleTypeQuiz && len(m.Questions) > 0
}

// HasGitContent returns true if the module references git repo content.
func (m Module) HasGitContent() bool {
	return m.Src != "" && m.Ref != "" && m.Path != ""
}

// ModuleIndexEntry is one entry in a module index YAML file (type: modules).
type ModuleIndexEntry struct {
	Name          string   `yaml:"name"`
	Type          string   `yaml:"type,omitempty"`
	Src           string   `yaml:"src,omitempty"`
	Ref           string   `yaml:"ref,omitempty"`
	Path          string   `yaml:"path"`
	Hidden        bool     `yaml:"hidden,omitempty"`
	Prerequisites []string `yaml:"prerequisites,omitempty"`
}

// AggregateSkills returns a deduplicated, sorted list of every skill tag
// declared across the given modules. The result is denormalized onto the
// course row so catalog filtering never has to join course_modules.
func AggregateSkills(modules []Module) []string {
	seen := make(map[string]struct{})

	for _, m := range modules {
		for _, s := range m.Skills {
			if s != "" {
				seen[s] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}

	slices.Sort(out)

	return out
}
