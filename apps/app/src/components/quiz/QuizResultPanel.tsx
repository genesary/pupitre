import type { QuestionResult, QuizQuestion } from '@pupitre/core/lib/api-types';

import { InlineText } from '@/components/quiz/QuizForm';

// The « Correction » of a submitted quiz (docs/specs/013-quizzes.md).

/** « Correction ». The API sends no correct answer (docs/specs/190-backend-defects.md B17). */
export function QuizReview({ questions, results }: { questions: QuizQuestion[]; results: QuestionResult[] }) {
  return (
    <>
      {results.map((result, i) => {
        const question = questions.find((q) => q.id === result.questionId);
        return (
          <div key={result.questionId} className={`review-item ${result.isCorrect ? 'correct' : 'wrong'} mb-2`}>
            <div className="review-item-inner">
              <span className={`review-status-icon ${result.isCorrect ? 'correct' : 'wrong'}`}>{result.isCorrect ? '✓' : '✗'}</span>
              <div className="review-item-body">
                <p className="review-q-text">
                  {i + 1}. <InlineText text={question?.question ?? ''} />
                </p>
                <p className="review-q-score">
                  {result.pointsEarned}/{result.pointsMax} pt{result.pointsMax > 1 ? 's' : ''}
                </p>
                {result.feedback && <InlineText as="p" className="review-q-feedback" text={result.feedback} />}
              </div>
            </div>
          </div>
        );
      })}
    </>
  );
}
