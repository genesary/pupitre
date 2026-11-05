package content

import "math"

// Question type identifiers understood by ScoreQuiz.
const (
	questionTypeSingle   = "single"
	questionTypeMultiple = "multiple"
	questionTypeBoolean  = "boolean"
	questionTypeOrder    = "order"
)

// scoringPercentageScale converts a score fraction into a percentage.
const scoringPercentageScale = 100

// ── Answer Submission ──────────────────────────────────────────────────

// UserAnswer represents a user's answer to a single question.
type UserAnswer struct {
	Single   *string  `json:"single,omitempty"`   // type single: answer ID
	Multiple []string `json:"multiple,omitempty"` // type multiple: answer IDs
	Boolean  *bool    `json:"boolean,omitempty"`  // type boolean
	Order    []string `json:"order,omitempty"`    // type order: item IDs in user order
}

// QuestionResult holds the scoring outcome for one question.
type QuestionResult struct {
	QuestionID string `json:"questionId"`
	Type       string `json:"type"`

	IsCorrect bool `json:"isCorrect"`

	PointsEarned int `json:"pointsEarned"`

	PointsMax     int         `json:"pointsMax"`
	CorrectAnswer any         `json:"correctAnswer,omitempty"`
	Feedback      string      `json:"feedback,omitempty"`
	SourceRefs    []SourceRef `json:"sourceRefs,omitempty"`
}

// QuizResult holds the overall scoring outcome.
type QuizResult struct {
	TotalScore int `json:"totalScore"`

	MaxScore int  `json:"maxScore"`
	Passed   bool `json:"passed"`

	QuestionResults []QuestionResult `json:"questionResults"`
}

// SubmitRequest represents a user's answers to a quiz submission.
type SubmitRequest struct {
	Answers map[string]UserAnswer `json:"answers"`
}

// ── Scoring Functions ──────────────────────────────────────────────────

// ScoreSingle scores a single-choice question against answer.
func ScoreSingle(question Question, answer *string) QuestionResult {
	result := QuestionResult{
		QuestionID: question.ID,
		Type:       question.Type,
		PointsMax:  question.Points,
	}
	if answer == nil {
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs

		return result
	}

	for _, a := range question.Answers {
		if a.ID == *answer {
			if a.Correct {
				result.IsCorrect = true
				result.PointsEarned = question.Points
				result.CorrectAnswer = a.ID
				result.Feedback = question.Feedback.Correct
			} else {
				result.Feedback = question.Feedback.Wrong
				result.CorrectAnswer = correctAnswerID(question.Answers)
				result.SourceRefs = question.Feedback.SourceRefs
			}

			return result
		}
	}

	result.Feedback = question.Feedback.Wrong
	result.CorrectAnswer = correctAnswerID(question.Answers)
	result.SourceRefs = question.Feedback.SourceRefs

	return result
}

// tallyMultipleChoice counts how many of the selected answers are correct
// (correctSelected) versus incorrect (falsePositives), and returns the set
// of correct answer IDs for question.
func tallyMultipleChoice(question Question, answers []string) (correctIDs map[string]bool, correctSelected, falsePositives int) { //nolint:nonamedreturns // gocritic(unnamedResult) wants names here
	correctIDs = make(map[string]bool)

	for _, a := range question.Answers {
		if a.Correct {
			correctIDs[a.ID] = true
		}
	}

	selected := make(map[string]bool)
	for _, id := range answers {
		selected[id] = true
	}

	for id := range selected {
		if correctIDs[id] {
			correctSelected++
		} else {
			falsePositives++
		}
	}

	return correctIDs, correctSelected, falsePositives
}

// partialMultipleChoiceScore computes the partial-credit points earned for
// a multiple-choice question that was not answered fully correctly.
func partialMultipleChoiceScore(question Question, correctCount, correctSelected, falsePositives int) int {
	if question.PartialScoring == nil || !question.PartialScoring.Enabled {
		return 0
	}

	raw := float64(question.Points) * float64(correctSelected-falsePositives) / float64(correctCount)
	if !question.PartialScoring.AllowNegative && raw < 0 {
		raw = 0
	}

	return int(math.Round(raw))
}

// ScoreMultiple scores a multiple-choice question against answers.
func ScoreMultiple(question Question, answers []string) QuestionResult {
	result := QuestionResult{
		QuestionID: question.ID,
		Type:       question.Type,
		PointsMax:  question.Points,
	}
	if len(answers) == 0 {
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs

		return result
	}

	correctIDs, correctSelected, falsePositives := tallyMultipleChoice(question, answers)
	allCorrect := correctSelected == len(correctIDs) && falsePositives == 0

	if allCorrect {
		result.IsCorrect = true
		result.PointsEarned = question.Points
		result.Feedback = question.Feedback.Correct
		result.CorrectAnswer = correctIDs

		return result
	}

	result.PointsEarned = partialMultipleChoiceScore(question, len(correctIDs), correctSelected, falsePositives)
	result.Feedback = question.Feedback.Wrong
	result.CorrectAnswer = correctIDs
	result.SourceRefs = question.Feedback.SourceRefs

	return result
}

// ScoreBoolean scores a true/false question against answer.
func ScoreBoolean(question Question, answer *bool) QuestionResult {
	result := QuestionResult{
		QuestionID: question.ID,
		Type:       question.Type,
		PointsMax:  question.Points,
	}
	if answer == nil {
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs

		return result
	}

	if question.CorrectAnswer != nil && *answer == *question.CorrectAnswer {
		result.IsCorrect = true
		result.PointsEarned = question.Points
		result.CorrectAnswer = *question.CorrectAnswer
		result.Feedback = question.Feedback.Correct
	} else {
		result.CorrectAnswer = question.CorrectAnswer
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs
	}

	return result
}

// ScoreOrder scores an ordering question against order.
func ScoreOrder(question Question, order []string) QuestionResult {
	result := QuestionResult{
		QuestionID: question.ID,
		Type:       question.Type,
		PointsMax:  question.Points,
	}
	if len(order) == 0 {
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs

		return result
	}

	correct := true

	for i, id := range order {
		if i >= len(question.CorrectOrder) || id != question.CorrectOrder[i] {
			correct = false

			break
		}
	}

	if correct && len(order) == len(question.CorrectOrder) {
		result.IsCorrect = true
		result.PointsEarned = question.Points
		result.CorrectAnswer = question.CorrectOrder
		result.Feedback = question.Feedback.Correct
	} else {
		result.CorrectAnswer = question.CorrectOrder
		result.Feedback = question.Feedback.Wrong
		result.SourceRefs = question.Feedback.SourceRefs
	}

	return result
}

// ── Helpers ────────────────────────────────────────────────────────────

// correctAnswerID returns the ID of the first correct answer in answers.
func correctAnswerID(answers []Answer) string {
	for _, a := range answers {
		if a.Correct {
			return a.ID
		}
	}

	return ""
}

// ScoreQuiz evaluates all answers and returns the full result.
func ScoreQuiz(quiz *Quiz, answers map[string]UserAnswer) QuizResult {
	result := QuizResult{}

	for _, question := range quiz.Questions {
		userAnswer := answers[question.ID]

		var questionResult QuestionResult

		switch question.Type {
		case questionTypeSingle:
			questionResult = ScoreSingle(question, userAnswer.Single)
		case questionTypeMultiple:
			questionResult = ScoreMultiple(question, userAnswer.Multiple)
		case questionTypeBoolean:
			questionResult = ScoreBoolean(question, userAnswer.Boolean)
		case questionTypeOrder:
			questionResult = ScoreOrder(question, userAnswer.Order)
		default:
			questionResult = QuestionResult{QuestionID: question.ID, Type: question.Type, PointsMax: question.Points}
		}

		result.TotalScore += questionResult.PointsEarned
		result.MaxScore += questionResult.PointsMax
		result.QuestionResults = append(result.QuestionResults, questionResult)
	}

	if result.MaxScore > 0 {
		pct := float64(result.TotalScore) / float64(result.MaxScore) * scoringPercentageScale
		result.Passed = pct >= float64(quiz.PassingScore)
	}

	return result
}
