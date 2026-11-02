package content

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ── Fetch Quiz from Git ──────────────────────────────────────────────

// FetchQuizContent clones a git repo at src/ref, reads the YAML file at path,
// and returns the parsed in-memory Quiz.
func FetchQuizContent(ctx context.Context, gc *GitCache, src, ref, path, token string) (*Quiz, error) {
	data, err := gc.FetchModuleContent(ctx, src, ref, path, token)
	if err != nil {
		return nil, fmt.Errorf("fetch quiz content: %w", err)
	}

	var quizDoc QuizYAML

	err = yaml.Unmarshal(data, &quizDoc)
	if err != nil {
		return nil, fmt.Errorf("parse quiz yaml: %w", err)
	}

	if quizDoc.ID == "" {
		quizDoc.ID = path
	}

	if quizDoc.Title == "" {
		quizDoc.Title = quizDoc.ID
	}

	return quizDoc.ToQuiz(), nil
}

// ── Quiz YAML (deserialization) ───────────────────────────────────────

// QuizYAML is the top-level YAML representation of a quiz module.
type QuizYAML struct {
	ID          string           `yaml:"id"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Version     string           `yaml:"version"`
	Covers      []CoverEntryYAML `yaml:"covers"`

	PassingScore int              `yaml:"passingScore"`
	Cooldown     CooldownSpecYAML `yaml:"cooldown"`

	MaxAttemptsPerQuestion *int `yaml:"maxAttemptsPerQuestion"`

	LockOnMaxAttempts bool           `yaml:"lockOnMaxAttempts"`
	Questions         []QuestionYAML `yaml:"questions"`
}

// CoverEntryYAML declares which course modules a quiz's questions cover.
type CoverEntryYAML struct {
	Course  string   `yaml:"course"`
	Modules []string `yaml:"modules"`
}

// CooldownSpecYAML configures the retry-cooldown strategy for a quiz.
type CooldownSpecYAML struct {
	Strategy string `yaml:"strategy"`

	BaseSeconds int     `yaml:"baseSeconds"`
	Multiplier  float64 `yaml:"multiplier"`

	MaxSeconds int `yaml:"maxSeconds"`
}

// QuestionYAML is the YAML representation of a single quiz question.
type QuestionYAML struct {
	ID         string       `yaml:"id"`
	Type       string       `yaml:"type"`
	Difficulty string       `yaml:"difficulty"`
	Points     int          `yaml:"points"`
	Question   string       `yaml:"question"`
	Answers    []AnswerYAML `yaml:"answers"`

	CorrectAnswer *bool           `yaml:"correctAnswer"`
	Items         []OrderItemYAML `yaml:"items"`

	CorrectOrder []string `yaml:"correctOrder"`

	PartialScoring *PartialScoringYAML `yaml:"partialScoring"`
	Feedback       FeedbackYAML        `yaml:"feedback"`
}

// AnswerYAML is a single answer option for a single/multiple-choice
// question.
type AnswerYAML struct {
	ID      string `yaml:"id"`
	Text    string `yaml:"text"`
	Correct bool   `yaml:"correct"`
}

// OrderItemYAML is a single item to be ordered in an order-type question.
type OrderItemYAML struct {
	ID   string `yaml:"id"`
	Text string `yaml:"text"`
}

// PartialScoringYAML configures whether partial credit is awarded for
// multiple-choice questions.
type PartialScoringYAML struct {
	Enabled bool `yaml:"enabled"`

	AllowNegative bool `yaml:"allowNegative"`
}

// FeedbackYAML holds the feedback text and source references shown after
// answering a question.
type FeedbackYAML struct {
	Wrong   string `yaml:"wrong"`
	Correct string `yaml:"correct"`

	SourceRefs []SourceRefYAML `yaml:"sourceRefs"`
}

// SourceRefYAML points to the course material backing a piece of feedback.
type SourceRefYAML struct {
	Course   string `yaml:"course"`
	Module   string `yaml:"module"`
	Anchor   string `yaml:"anchor"`
	Priority int    `yaml:"priority"`
}

// ── In-Memory Quiz ────────────────────────────────────────────────────

// Quiz is the in-memory representation of a quiz module.
type Quiz struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Covers      []CoverEntry `json:"covers"`

	PassingScore int          `json:"passingScore"`
	Cooldown     CooldownSpec `json:"cooldown"`

	MaxAttemptsPerQuestion *int `json:"maxAttemptsPerQuestion,omitempty"`

	LockOnMaxAttempts bool       `json:"lockOnMaxAttempts"`
	Questions         []Question `json:"questions"`
}

// CoverEntry declares which course modules a quiz's questions cover.
type CoverEntry struct {
	Course  string   `json:"course"`
	Modules []string `json:"modules"`
}

// CooldownSpec configures the retry-cooldown strategy for a quiz.
type CooldownSpec struct {
	Strategy string `json:"strategy"`

	BaseSeconds int     `json:"baseSeconds"`
	Multiplier  float64 `json:"multiplier"`

	MaxSeconds int `json:"maxSeconds"`
}

// Question is the in-memory representation of a single quiz question.
type Question struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Difficulty string   `json:"difficulty,omitempty"`
	Points     int      `json:"points"`
	Question   string   `json:"question"`
	Answers    []Answer `json:"answers,omitempty"`

	CorrectAnswer *bool       `json:"correctAnswer,omitempty"`
	Items         []OrderItem `json:"items,omitempty"`

	CorrectOrder []string `json:"correctOrder,omitempty"`

	PartialScoring *PartialScoring `json:"partialScoring,omitempty"`
	Feedback       Feedback        `json:"feedback,omitempty"`
}

// Answer is a single answer option for a single/multiple-choice question.
type Answer struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Correct bool   `json:"correct,omitempty"`
}

// OrderItem is a single item to be ordered in an order-type question.
type OrderItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// PartialScoring configures whether partial credit is awarded for
// multiple-choice questions.
type PartialScoring struct {
	Enabled bool `json:"enabled"`

	AllowNegative bool `json:"allowNegative"`
}

// Feedback holds the feedback text and source references shown after
// answering a question.
type Feedback struct {
	Wrong   string `json:"wrong,omitempty"`
	Correct string `json:"correct,omitempty"`

	SourceRefs []SourceRef `json:"sourceRefs,omitempty"`
}

// SourceRef points to the course material backing a piece of feedback.
type SourceRef struct {
	Course   string `json:"course"`
	Module   string `json:"module"`
	Anchor   string `json:"anchor"`
	Priority int    `json:"priority"`
}

// ── Quiz → In-Memory ──────────────────────────────────────────────────

// ToQuiz converts the YAML-deserialized quiz document into its in-memory
// representation.
func (qy QuizYAML) ToQuiz() *Quiz {
	quiz := &Quiz{
		ID:                     qy.ID,
		Title:                  qy.Title,
		Description:            qy.Description,
		Version:                qy.Version,
		PassingScore:           qy.PassingScore,
		Cooldown:               CooldownSpec(qy.Cooldown),
		MaxAttemptsPerQuestion: qy.MaxAttemptsPerQuestion,
		LockOnMaxAttempts:      qy.LockOnMaxAttempts,
	}
	for _, c := range qy.Covers {
		quiz.Covers = append(quiz.Covers, CoverEntry(c))
	}

	for _, questionYAML := range qy.Questions {
		quiz.Questions = append(quiz.Questions, convertQuestion(questionYAML))
	}

	return quiz
}

// convertQuestion converts a single YAML question into its in-memory form,
// applying default Points and Difficulty when unset.
func convertQuestion(rawQuestion QuestionYAML) Question {
	question := Question{
		ID:            rawQuestion.ID,
		Type:          rawQuestion.Type,
		Difficulty:    rawQuestion.Difficulty,
		Points:        rawQuestion.Points,
		Question:      rawQuestion.Question,
		CorrectAnswer: rawQuestion.CorrectAnswer,
		CorrectOrder:  rawQuestion.CorrectOrder,
		Answers:       make([]Answer, len(rawQuestion.Answers)),
		Items:         make([]OrderItem, len(rawQuestion.Items)),
	}
	for i, a := range rawQuestion.Answers {
		question.Answers[i] = Answer(a)
	}

	for i, it := range rawQuestion.Items {
		question.Items[i] = OrderItem(it)
	}

	if rawQuestion.PartialScoring != nil {
		question.PartialScoring = &PartialScoring{
			Enabled:       rawQuestion.PartialScoring.Enabled,
			AllowNegative: rawQuestion.PartialScoring.AllowNegative,
		}
	}

	question.Feedback = Feedback{
		Wrong:   rawQuestion.Feedback.Wrong,
		Correct: rawQuestion.Feedback.Correct,
	}
	for _, sr := range rawQuestion.Feedback.SourceRefs {
		question.Feedback.SourceRefs = append(question.Feedback.SourceRefs, SourceRef(sr))
	}

	if question.Points == 0 {
		question.Points = 1
	}

	if question.Difficulty == "" {
		question.Difficulty = DifficultyMedium
	}

	return question
}
