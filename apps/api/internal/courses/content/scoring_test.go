package content

import (
	"testing"
)

// makeQuestion builds a Question fixture with the given id, type, and points.
func makeQuestion(id, typ string, points int) Question {
	return Question{
		ID:     id,
		Type:   typ,
		Points: points,
		Feedback: Feedback{
			Correct: "Correct!",
			Wrong:   "Wrong!",
		},
	}
}

// ─── ScoreSingle ───────────────────────────────────────────────────────────

// TestScoreSingle_NilAnswer checks score single nil answer.
func TestScoreSingle_NilAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "single", 5)
	q.Answers = []Answer{{ID: "a", Text: "A", Correct: true}}

	r := ScoreSingle(q, nil)
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for nil answer")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}

	if r.PointsMax != 5 {
		t.Errorf("expected max 5, got %d", r.PointsMax)
	}
}

// TestScoreSingle_CorrectAnswer checks score single correct answer.
func TestScoreSingle_CorrectAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "single", 10)
	q.Answers = []Answer{
		{ID: "a", Text: "A", Correct: false},
		{ID: "b", Text: "B", Correct: true},
	}

	r := ScoreSingle(q, new("b"))
	if !r.IsCorrect {
		t.Error("expected IsCorrect=true")
	}

	if r.PointsEarned != 10 {
		t.Errorf("expected 10 points, got %d", r.PointsEarned)
	}
}

// TestScoreSingle_WrongAnswer checks score single wrong answer.
func TestScoreSingle_WrongAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "single", 10)
	q.Answers = []Answer{
		{ID: "a", Text: "A", Correct: false},
		{ID: "b", Text: "B", Correct: true},
	}

	r := ScoreSingle(q, new("a"))
	if r.IsCorrect {
		t.Error("expected IsCorrect=false")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}

	if r.CorrectAnswer != "b" {
		t.Errorf("expected correct answer=b, got %v", r.CorrectAnswer)
	}
}

// TestScoreSingle_UnknownAnswer checks score single unknown answer.
func TestScoreSingle_UnknownAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "single", 5)
	q.Answers = []Answer{{ID: "a", Text: "A", Correct: true}}

	r := ScoreSingle(q, new("z"))
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for unknown answer")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}
}

// ─── ScoreMultiple ─────────────────────────────────────────────────────────

// TestScoreMultiple_EmptyAnswers checks score multiple empty answers.
func TestScoreMultiple_EmptyAnswers(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "multiple", 6)
	q.Answers = []Answer{{ID: "a", Correct: true}, {ID: "b", Correct: true}}

	r := ScoreMultiple(q, nil)
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for nil answers")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}
}

// TestScoreMultiple_AllCorrect checks score multiple all correct.
func TestScoreMultiple_AllCorrect(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "multiple", 6)
	q.Answers = []Answer{
		{ID: "a", Correct: true},
		{ID: "b", Correct: true},
		{ID: "c", Correct: false},
	}

	r := ScoreMultiple(q, []string{"a", "b"})
	if !r.IsCorrect {
		t.Error("expected IsCorrect=true")
	}

	if r.PointsEarned != 6 {
		t.Errorf("expected 6 points, got %d", r.PointsEarned)
	}
}

// TestScoreMultiple_PartialWithFalsePositive checks partial, false positive.
func TestScoreMultiple_PartialWithFalsePositive(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "multiple", 10)
	q.Answers = []Answer{
		{ID: "a", Correct: true},
		{ID: "b", Correct: true},
		{ID: "c", Correct: false},
	}
	q.PartialScoring = &PartialScoring{Enabled: true}
	// Selected a (correct) + c (wrong) → 1 correct, 1 false positive → score = 10*(1-1)/2 = 0
	r := ScoreMultiple(q, []string{"a", "c"})
	if r.IsCorrect {
		t.Error("expected IsCorrect=false")
	}
}

// TestScoreMultiple_PartialNoNegative checks partial scoring, no negative.
func TestScoreMultiple_PartialNoNegative(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "multiple", 10)
	q.Answers = []Answer{
		{ID: "a", Correct: true},
		{ID: "b", Correct: true},
		{ID: "c", Correct: false},
	}
	q.PartialScoring = &PartialScoring{Enabled: true, AllowNegative: false}
	// All wrong selected → score cannot be negative
	r := ScoreMultiple(q, []string{"c"})
	if r.PointsEarned < 0 {
		t.Errorf("partial scoring should not go negative, got %d", r.PointsEarned)
	}
}

// TestScoreMultiple_WrongAll_NoPartial checks all-wrong without partial credit.
func TestScoreMultiple_WrongAll_NoPartial(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "multiple", 6)
	q.Answers = []Answer{
		{ID: "a", Correct: true},
		{ID: "b", Correct: false},
	}

	r := ScoreMultiple(q, []string{"b"})
	if r.IsCorrect {
		t.Error("expected IsCorrect=false")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points without partial scoring, got %d", r.PointsEarned)
	}
}

// ─── ScoreBoolean ──────────────────────────────────────────────────────────

// TestScoreBoolean_NilAnswer checks score boolean nil answer.
func TestScoreBoolean_NilAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "boolean", 4)
	q.CorrectAnswer = new(true)

	r := ScoreBoolean(q, nil)
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for nil")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}
}

// TestScoreBoolean_Correct checks score boolean correct.
func TestScoreBoolean_Correct(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "boolean", 4)
	q.CorrectAnswer = new(true)

	r := ScoreBoolean(q, new(true))
	if !r.IsCorrect {
		t.Error("expected IsCorrect=true")
	}

	if r.PointsEarned != 4 {
		t.Errorf("expected 4 points, got %d", r.PointsEarned)
	}
}

// TestScoreBoolean_Wrong checks score boolean wrong.
func TestScoreBoolean_Wrong(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "boolean", 4)
	q.CorrectAnswer = new(true)

	r := ScoreBoolean(q, new(false))
	if r.IsCorrect {
		t.Error("expected IsCorrect=false")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}
}

// TestScoreBoolean_NilCorrectAnswer checks score boolean nil correct answer.
func TestScoreBoolean_NilCorrectAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "boolean", 4)
	q.CorrectAnswer = nil

	r := ScoreBoolean(q, new(true))
	if r.IsCorrect {
		t.Error("expected IsCorrect=false when CorrectAnswer is nil")
	}
}

// TestScoreSingle_NoCorrectAnswerDefined checks missing correct answer.
func TestScoreSingle_NoCorrectAnswerDefined(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "single", 5)
	// No answer has Correct: true → correctAnswerID returns ""
	q.Answers = []Answer{
		{ID: "a", Text: "A", Correct: false},
		{ID: "b", Text: "B", Correct: false},
	}

	r := ScoreSingle(q, new("a"))
	if r.IsCorrect {
		t.Error("expected IsCorrect=false when no correct answer defined")
	}
}

// ─── ScoreOrder ────────────────────────────────────────────────────────────

// TestScoreOrder_EmptyAnswer checks score order empty answer.
func TestScoreOrder_EmptyAnswer(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "order", 5)
	q.CorrectOrder = []string{"a", "b", "c"}

	r := ScoreOrder(q, nil)
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for nil order")
	}
}

// TestScoreOrder_Correct checks score order correct.
func TestScoreOrder_Correct(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "order", 5)
	q.CorrectOrder = []string{"a", "b", "c"}

	r := ScoreOrder(q, []string{"a", "b", "c"})
	if !r.IsCorrect {
		t.Error("expected IsCorrect=true")
	}

	if r.PointsEarned != 5 {
		t.Errorf("expected 5 points, got %d", r.PointsEarned)
	}
}

// TestScoreOrder_WrongOrder checks score order wrong order.
func TestScoreOrder_WrongOrder(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "order", 5)
	q.CorrectOrder = []string{"a", "b", "c"}

	r := ScoreOrder(q, []string{"b", "a", "c"})
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for wrong order")
	}

	if r.PointsEarned != 0 {
		t.Errorf("expected 0 points, got %d", r.PointsEarned)
	}
}

// TestScoreOrder_WrongLength checks score order wrong length.
func TestScoreOrder_WrongLength(t *testing.T) {
	t.Parallel()

	q := makeQuestion("q1", "order", 5)
	q.CorrectOrder = []string{"a", "b", "c"}

	r := ScoreOrder(q, []string{"a", "b"})
	if r.IsCorrect {
		t.Error("expected IsCorrect=false for wrong length")
	}
}

// ─── ScoreQuiz ─────────────────────────────────────────────────────────────

// TestScoreQuiz_Mixed checks score quiz mixed.
func TestScoreQuiz_Mixed(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 60,
		Questions: []Question{
			{
				ID:     "q1",
				Type:   "single",
				Points: 10,
				Answers: []Answer{
					{ID: "a", Correct: true},
					{ID: "b", Correct: false},
				},
				Feedback: Feedback{Correct: "yes", Wrong: "no"},
			},
			{
				ID:            "q2",
				Type:          "boolean",
				Points:        10,
				CorrectAnswer: new(false),
				Feedback:      Feedback{Correct: "yes", Wrong: "no"},
			},
		},
	}

	answers := map[string]UserAnswer{
		"q1": {Single: new("a")},
		"q2": {Boolean: new(false)},
	}

	result := ScoreQuiz(quiz, answers)
	if result.TotalScore != 20 {
		t.Errorf("expected total score=20, got %d", result.TotalScore)
	}

	if result.MaxScore != 20 {
		t.Errorf("expected max=20, got %d", result.MaxScore)
	}

	if !result.Passed {
		t.Error("expected Passed=true (100%% >= 60%%)")
	}
}

// TestScoreQuiz_NotPassed checks score quiz not passed.
func TestScoreQuiz_NotPassed(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 80,
		Questions: []Question{
			{
				ID:     "q1",
				Type:   "single",
				Points: 10,
				Answers: []Answer{
					{ID: "a", Correct: true},
				},
				Feedback: Feedback{Wrong: "no"},
			},
		},
	}
	answers := map[string]UserAnswer{
		"q1": {Single: nil}, // wrong
	}

	result := ScoreQuiz(quiz, answers)
	if result.Passed {
		t.Error("expected Passed=false")
	}
}

// TestScoreQuiz_UnknownType checks score quiz unknown type.
func TestScoreQuiz_UnknownType(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 50,
		Questions: []Question{
			{
				ID:     "q1",
				Type:   "unknown-type",
				Points: 5,
			},
		},
	}
	answers := map[string]UserAnswer{"q1": {}}

	result := ScoreQuiz(quiz, answers)
	if result.TotalScore != 0 {
		t.Errorf("expected 0 for unknown type, got %d", result.TotalScore)
	}
}

// TestScoreQuiz_EmptyAnswers checks score quiz empty answers.
func TestScoreQuiz_EmptyAnswers(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 50,
		Questions:    []Question{},
	}

	result := ScoreQuiz(quiz, map[string]UserAnswer{})
	if result.MaxScore != 0 {
		t.Errorf("expected MaxScore=0, got %d", result.MaxScore)
	}

	if result.Passed {
		t.Error("expected Passed=false when MaxScore=0")
	}
}

// TestScoreQuiz_OrderQuestion checks score quiz order question.
func TestScoreQuiz_OrderQuestion(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 50,
		Questions: []Question{
			{
				ID:           "q1",
				Type:         "order",
				Points:       10,
				CorrectOrder: []string{"x", "y", "z"},
				Feedback:     Feedback{Correct: "good", Wrong: "bad"},
			},
		},
	}
	answers := map[string]UserAnswer{
		"q1": {Order: []string{"x", "y", "z"}},
	}

	result := ScoreQuiz(quiz, answers)
	if result.TotalScore != 10 {
		t.Errorf("expected 10, got %d", result.TotalScore)
	}
}

// TestScoreQuiz_MultipleQuestion checks score quiz multiple question.
func TestScoreQuiz_MultipleQuestion(t *testing.T) {
	t.Parallel()

	quiz := &Quiz{
		PassingScore: 50,
		Questions: []Question{
			{
				ID:     "q1",
				Type:   "multiple",
				Points: 6,
				Answers: []Answer{
					{ID: "a", Correct: true},
					{ID: "b", Correct: true},
					{ID: "c", Correct: false},
				},
				Feedback: Feedback{Correct: "yes", Wrong: "no"},
			},
		},
	}
	answers := map[string]UserAnswer{
		"q1": {Multiple: []string{"a", "b"}},
	}

	result := ScoreQuiz(quiz, answers)
	if result.TotalScore != 6 {
		t.Errorf("expected 6, got %d", result.TotalScore)
	}

	if !result.Passed {
		t.Error("expected Passed=true")
	}
}
