import { useMutation } from '@tanstack/react-query';
import { useRef } from 'react';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import { submitQuiz } from '@pupitre/core/lib/api';
import type { Cooldown, InlineQuiz as InlineQuizData } from '@pupitre/core/lib/api-types';

import { antiCopy } from '@/lib/anti-copy';
import { CooldownBanner, InlineText, QuestionOptions } from '@/components/quiz/QuizForm';
import { QuizReview } from '@/components/quiz/QuizResultPanel';
import { useQuiz } from '@/lib/use-quiz';

// The mini-quiz shown at the bottom of a lesson (docs/specs/013-quizzes.md),
// markup of the Astro lesson page's renderMiniQuiz().

type InlineQuizProps = {
  slug: string;
  quiz: InlineQuizData;
  /** Called once when the quiz is passed for the first time. */
  onPassed: () => void;
};

export function InlineQuiz({ slug, quiz, onPassed }: InlineQuizProps) {
  const questions = quiz.questions ?? [];
  const state = useQuiz({ slug, index: quiz.index, questions });
  // Already counted as done on load: a second pass must not notify again.
  const notified = useRef(quiz.passed);

  const submit = useMutation({
    mutationFn: () => submitQuiz(slug, quiz.index, state.payload()),
    onSuccess: (result) => {
      state.setResult(result);
      state.startCooldowns(result.cooldowns);
      if (result.passed && !notified.current) {
        notified.current = true;
        onPassed();
      }
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 425) {
        state.startCooldowns(error.body?.cooldowns as Record<string, Cooldown> | undefined);
      }
      toast.error('Erreur lors de la soumission');
    },
  });

  const result = state.result;
  const pct = result && result.maxScore > 0 ? Math.round((result.totalScore / result.maxScore) * 100) : 0;

  return (
    <div className="mt-2" {...antiCopy}>
      <div className="mini-quiz-block select-none">
        <div className="mini-quiz-header">
          <img src="/pioupitre.png" alt="pioupitre" />
          <div>
            <div className="mini-quiz-title">Mini-quiz · fin de module</div>
            <div className="mini-quiz-subtitle">
              {quiz.name} · {questions.length} question{questions.length !== 1 ? 's' : ''}
            </div>
          </div>
        </div>
        <div>
          {result ? (
            <>
              <div className={`mini-result ${result.passed ? 'pass' : 'fail'}`}>
                <div className={`mini-score-circle ${result.passed ? 'pass' : 'fail'}`}>
                  <div className={`mini-score-pct ${result.passed ? 'pass' : 'fail'}`}>{pct}%</div>
                  <div className="mini-score-fraction">
                    {result.totalScore}/{result.maxScore}
                  </div>
                </div>
                <div className="mini-result-body">
                  <h3 className={`mini-result-title ${result.passed ? 'pass' : 'fail'}`}>
                    {result.passed ? '✓ Quiz réussi !' : '✗ Quiz échoué'}
                  </h3>
                  <p className="mini-result-sub">
                    {result.passed
                      ? `Félicitations — ${result.totalScore} pts sur ${result.maxScore}.`
                      : `Score insuffisant (${pct}%). Réessayez !`}
                  </p>
                </div>
              </div>
              {!result.passed && state.canRetry && (
                <div className="mb-3">
                  <button className="btn btn-primary" onClick={state.reset}>
                    Recommencer
                  </button>
                </div>
              )}
              {result.questionResults.length > 0 && (
                <div className="mt-4">
                  <p className="font-bold text-[.875rem] mb-2.5">Correction</p>
                  <QuizReview questions={questions} results={result.questionResults} />
                </div>
              )}
            </>
          ) : (
            <>
              <div className="stack gap-4 mb-4">
                {questions.map((question, i) => (
                  <div key={question.id}>
                    <p className="mini-q-text">
                      {i + 1}. <InlineText text={question.question} />
                    </p>
                    <CooldownBanner cooldown={state.cooldowns[question.id]} />
                    <QuestionOptions
                      question={question}
                      value={state.answers[question.id] ?? null}
                      disabled={submit.isPending || state.blocked(question.id)}
                      onChange={(value) => state.setAnswer(question.id, value)}
                    />
                  </div>
                ))}
              </div>
              <div className="mini-submit-row">
                <button className="btn btn-primary" disabled={submit.isPending} onClick={() => submit.mutate()}>
                  {submit.isPending ? 'Envoi…' : 'Valider le quiz ▸'}
                </button>
                <span className="mini-answered">
                  {state.answeredCount}/{questions.length} répondues
                </span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
