package content

import (
	"testing"
)

// TestToQuiz_Basic checks to quiz basic.
func TestToQuiz_Basic(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{
		ID:           "quiz-1",
		Title:        "Test Quiz",
		Description:  "A test quiz",
		PassingScore: 70,
		Questions: []QuestionYAML{
			{
				ID:       "q1",
				Type:     "single",
				Question: "What is 2+2?",
				Points:   2,
				Answers: []AnswerYAML{
					{ID: "a1", Text: "3", Correct: false},
					{ID: "a2", Text: "4", Correct: true},
				},
			},
		},
	}

	quiz := qy.ToQuiz()
	if quiz.ID != "quiz-1" {
		t.Errorf("ID: want quiz-1, got %q", quiz.ID)
	}

	if quiz.Title != "Test Quiz" {
		t.Errorf("Title: want Test Quiz, got %q", quiz.Title)
	}

	if quiz.PassingScore != 70 {
		t.Errorf("PassingScore: want 70, got %d", quiz.PassingScore)
	}

	if len(quiz.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(quiz.Questions))
	}

	q := quiz.Questions[0]
	if q.Points != 2 {
		t.Errorf("Points: want 2, got %d", q.Points)
	}

	if len(q.Answers) != 2 {
		t.Errorf("expected 2 answers, got %d", len(q.Answers))
	}
}

// TestToQuiz_DefaultPointsAndDifficulty checks default points and difficulty.
func TestToQuiz_DefaultPointsAndDifficulty(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{
		Questions: []QuestionYAML{
			{ID: "q1", Type: "boolean"},
		},
	}

	quiz := qy.ToQuiz()
	if quiz.Questions[0].Points != 1 {
		t.Errorf("default Points: want 1, got %d", quiz.Questions[0].Points)
	}

	if quiz.Questions[0].Difficulty != "medium" {
		t.Errorf("default Difficulty: want medium, got %q", quiz.Questions[0].Difficulty)
	}
}

// TestToQuiz_DefaultIDAndTitle checks to quiz default ID and title.
func TestToQuiz_DefaultIDAndTitle(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{} // no ID, no Title

	quiz := qy.ToQuiz()
	if quiz.ID != "" || quiz.Title != "" {
		t.Errorf("expected empty ID and Title for empty input, got %q and %q", quiz.ID, quiz.Title)
	}
}

// TestToQuiz_FillsIDFromPath checks to quiz fills ID from path.
func TestToQuiz_FillsIDFromPath(t *testing.T) {
	t.Parallel()

	// FetchQuizContent sets ID from path when empty — ToQuiz just propagates it
	qy := QuizYAML{ID: "content/quiz.yaml", Title: ""}

	quiz := qy.ToQuiz()
	if quiz.ID != "content/quiz.yaml" {
		t.Errorf("ID: want content/quiz.yaml, got %q", quiz.ID)
	}
}

// TestToQuiz_WithOrderItems checks to quiz with order items.
func TestToQuiz_WithOrderItems(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{
		Questions: []QuestionYAML{
			{
				ID:   "q1",
				Type: "order",
				Items: []OrderItemYAML{
					{ID: "i1", Text: "First"},
					{ID: "i2", Text: "Second"},
				},
			},
		},
	}

	quiz := qy.ToQuiz()
	if len(quiz.Questions[0].Items) != 2 {
		t.Errorf("expected 2 order items, got %d", len(quiz.Questions[0].Items))
	}

	if quiz.Questions[0].Items[0].Text != "First" {
		t.Errorf("expected First, got %q", quiz.Questions[0].Items[0].Text)
	}
}

// TestToQuiz_WithPartialScoring checks to quiz with partial scoring.
func TestToQuiz_WithPartialScoring(t *testing.T) {
	t.Parallel()

	enabled := true
	qy := QuizYAML{
		Questions: []QuestionYAML{
			{
				ID:   "q1",
				Type: "multiple",
				PartialScoring: &PartialScoringYAML{
					Enabled:       enabled,
					AllowNegative: false,
				},
			},
		},
	}

	quiz := qy.ToQuiz()
	if quiz.Questions[0].PartialScoring == nil {
		t.Fatal("expected PartialScoring to be set")
	}

	if !quiz.Questions[0].PartialScoring.Enabled {
		t.Error("expected PartialScoring.Enabled=true")
	}
}

// TestToQuiz_WithFeedback checks to quiz with feedback.
func TestToQuiz_WithFeedback(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{
		Questions: []QuestionYAML{
			{
				ID:   "q1",
				Type: "single",
				Feedback: FeedbackYAML{
					Wrong:   "Try again",
					Correct: "Well done!",
					SourceRefs: []SourceRefYAML{
						{Course: "intro", Module: "basics", Priority: 1},
					},
				},
			},
		},
	}
	quiz := qy.ToQuiz()

	q := quiz.Questions[0]
	if q.Feedback.Wrong != "Try again" {
		t.Errorf("Wrong: want 'Try again', got %q", q.Feedback.Wrong)
	}

	if len(q.Feedback.SourceRefs) != 1 {
		t.Fatalf("expected 1 source ref, got %d", len(q.Feedback.SourceRefs))
	}

	if q.Feedback.SourceRefs[0].Course != "intro" {
		t.Errorf("SourceRef.Course: want intro, got %q", q.Feedback.SourceRefs[0].Course)
	}
}

// TestToQuiz_WithCovers checks to quiz with covers.
func TestToQuiz_WithCovers(t *testing.T) {
	t.Parallel()

	qy := QuizYAML{
		Covers: []CoverEntryYAML{
			{Course: "kubernetes", Modules: []string{"pods", "services"}},
		},
	}

	quiz := qy.ToQuiz()
	if len(quiz.Covers) != 1 {
		t.Fatalf("expected 1 cover entry, got %d", len(quiz.Covers))
	}

	if quiz.Covers[0].Course != "kubernetes" {
		t.Errorf("Cover.Course: want kubernetes, got %q", quiz.Covers[0].Course)
	}

	if len(quiz.Covers[0].Modules) != 2 {
		t.Errorf("Cover.Modules: expected 2, got %d", len(quiz.Covers[0].Modules))
	}
}
