import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { Link, Navigate, useParams } from 'react-router';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import { getCourse, getModule, listModules, submitQuiz } from '@pupitre/core/lib/api';
import type { Cooldown, ModuleDetail, ModuleSummary } from '@pupitre/core/lib/api-types';
import { hostOf, moduleHref } from '@/lib/module-links';

import { antiCopy } from '@/lib/anti-copy';
import { CooldownBanner, InlineText, QuestionOptions } from '@/components/quiz/QuizForm';
import { QuizReview } from '@/components/quiz/QuizResultPanel';
import { isAnswered, useQuiz } from '@/lib/use-quiz';

// /courses/:slug/quiz/:index (docs/specs/013-quizzes.md), markup of pupitre's
// pages/courses/[slug]/quiz/[index].astro.

function QuizScreen({ slug, courseTitle, quiz, modules }: { slug: string; courseTitle: string; quiz: ModuleDetail; modules: ModuleSummary[] }) {
  const queryClient = useQueryClient();
  const questions = quiz.questions ?? [];
  const state = useQuiz({ slug, index: quiz.index, questions, serverCooldowns: quiz.cooldowns });
  const [focused, setFocused] = useState(0);
  const cards = useRef<(HTMLDivElement | null)[]>([]);

  // The palette follows the question in view, like the Astro page.
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setFocused(Number((entry.target as HTMLElement).dataset.idx));
        }
      },
      { rootMargin: '-40% 0px -55% 0px' },
    );
    cards.current.forEach((card) => card && observer.observe(card));
    return () => observer.disconnect();
  }, [questions.length, state.result]);

  const submit = useMutation({
    mutationFn: () => submitQuiz(slug, quiz.index, state.payload()),
    onSuccess: (result) => {
      state.setResult(result);
      state.startCooldowns(result.cooldowns);
      window.scrollTo({ top: 0 });
      void queryClient.invalidateQueries({ queryKey: ['course', slug, 'modules'] });
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 425) {
        state.startCooldowns(error.body?.cooldowns as Record<string, Cooldown> | undefined);
      }
      toast.error('Erreur lors de la soumission');
    },
  });

  const answeredPct = questions.length > 0 ? Math.round((state.answeredCount / questions.length) * 100) : 0;
  const submitLabel = submit.isPending ? 'Envoi…' : 'Soumettre le quiz';
  const result = state.result;

  if (result) {
    const pct = result.maxScore > 0 ? Math.round((result.totalScore / result.maxScore) * 100) : 0;
    const passingScore = quiz.quizConfig?.passingScore;
    const next = modules.find((m) => m.index > quiz.index && !m.locked);
    const subtitle = result.passed
      ? `Félicitations — ${result.totalScore} pts sur ${result.maxScore}.`
      : passingScore
        ? `Score insuffisant (${pct}% < ${passingScore}%). Réessayez !`
        : `Score : ${result.totalScore}/${result.maxScore}`;
    return (
      <div className="result-screen">
        <div className={`result-hero${result.passed ? '' : ' fail'}`}>
          <div className={`score-circle${result.passed ? '' : ' fail'}`}>
            <div className="score-num">{pct}%</div>
            <div className="score-den">
              {result.totalScore}/{result.maxScore}
            </div>
          </div>
          <div className="flex-1">
            <h2 className="mb-1.5">{result.passed ? 'Quiz réussi !' : 'Quiz échoué'}</h2>
            <p className="text-soft text-sm result-subtitle">{subtitle}</p>
            <div className="result-badges">
              <span className={`badge ${result.passed ? 'badge-green' : 'badge-red'}`}>{result.passed ? 'Réussi' : 'Échoué'}</span>
              <span className="badge badge-blue">
                {result.totalScore}/{result.maxScore} pts
              </span>
            </div>
          </div>
          <img src="/pioupitre.png" alt="" className="w-20 h-20 object-contain flex-none" />
        </div>
        <div className="result-actions">
          {result.passed && next && (
            <Link to={moduleHref(slug, next)} className="btn btn-primary">
              Module suivant ▸
            </Link>
          )}
          {!result.passed && state.canRetry && (
            <button
              className="btn btn-primary"
              onClick={() => {
                state.reset();
                window.scrollTo({ top: 0 });
              }}
            >
              Recommencer
            </button>
          )}
          <Link to={`/courses/${slug}`} className="btn btn-secondary">
            Retour au cours
          </Link>
        </div>
        {result.questionResults.length > 0 && (
          <div className="result-review">
            <h3 className="quiz-review-heading">Correction</h3>
            <QuizReview questions={questions} results={result.questionResults} />
          </div>
        )}
      </div>
    );
  }

  return (
    <div {...antiCopy}>
      <div className="quiz-breadcrumb">
        <Link to="/courses">Catalogue</Link>
        <span>›</span>
        <Link to={`/courses/${slug}`}>{courseTitle}</Link>
        <span>›</span>
        <span>{quiz.name}</span>
      </div>
      <div className="quiz-page-layout select-none">
        <div className="quiz-main">
          <h1 className="quiz-title-head">{quiz.name}</h1>
          <p className="text-soft text-sm quiz-subtitle-text">
            {questions.length} question{questions.length !== 1 ? 's' : ''}
          </p>
          <div>
            {questions.map((question, i) => (
              <div
                key={question.id}
                id={`q-card-${question.id}`}
                data-idx={i}
                ref={(el) => {
                  cards.current[i] = el;
                }}
                className={`quiz-q-card${isAnswered(question, state.answers[question.id] ?? null) ? ' has-answer' : ''}`}
              >
                <div className="quiz-q-header">
                  <span className="qnum-chip">Q{i + 1}</span>
                  <InlineText as="div" className="quiz-q-text" text={question.question} />
                  <span className="quiz-q-points">
                    {question.points} pt{question.points > 1 ? 's' : ''}
                  </span>
                </div>
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
          <button className="btn btn-primary btn-lg w-full mt-2" disabled={submit.isPending} onClick={() => submit.mutate()}>
            {submitLabel}
          </button>
        </div>

        <div className="quiz-palette">
          <div className="palette-label">Navigation</div>
          <div className="palette-chips">
            {questions.map((question, i) => (
              <button
                key={question.id}
                type="button"
                aria-label={`Question ${i + 1}`}
                className={`palette-chip${i === focused ? ' current' : isAnswered(question, state.answers[question.id] ?? null) ? ' answered' : ''}`}
                onClick={() => {
                  setFocused(i);
                  cards.current[i]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                }}
              >
                {i + 1}
              </button>
            ))}
          </div>
          <div className="palette-progress-wrap">
            <div className="palette-progress-row">
              <span>Répondues</span>
              <span>
                {state.answeredCount} / {questions.length}
              </span>
            </div>
            <div className="progress-bar">
              <div className="progress-bar-fill" style={{ width: `${answeredPct}%` }} />
            </div>
          </div>
          <button className="btn btn-primary btn-lg w-full" disabled={submit.isPending} onClick={() => submit.mutate()}>
            {submitLabel}
          </button>
          <p className="text-soft text-[.78rem]">Cliquez sur un numéro pour accéder directement à la question.</p>
        </div>
      </div>
    </div>
  );
}

export function QuizPage() {
  const { slug = '', index: rawIndex = '' } = useParams();
  const index = Number(rawIndex);
  const valid = Number.isInteger(index) && index >= 0;

  const course = useQuery({ queryKey: ['course', slug], queryFn: () => getCourse(slug) });
  const modules = useQuery({ queryKey: ['course', slug, 'modules'], queryFn: () => listModules(slug), enabled: valid });
  const quiz = useQuery({ queryKey: ['course', slug, 'module', index], queryFn: () => getModule(slug, index), enabled: valid });

  if (!valid) return <Navigate to={`/courses/${slug}`} replace />;

  const forbidden = [modules.error, quiz.error].some((e) => e instanceof ApiError && e.status === 403);
  const current = modules.data?.modules.find((m) => m.index === index);
  if (forbidden || current?.locked) return <Navigate to={`/courses/${slug}`} replace />;

  if (quiz.data && quiz.data.type !== 'quiz') return <Navigate to={`/courses/${slug}/lessons/${index}`} replace />;
  if (quiz.data?.inline && modules.data) {
    const host = hostOf(modules.data.modules, quiz.data);
    return <Navigate to={host ? `/courses/${slug}/lessons/${host.index}` : `/courses/${slug}`} replace />;
  }

  if (quiz.isError || modules.isError) {
    return (
      <div className="text-center py-16">
        <h2>Quiz introuvable</h2>
        <Link to="/courses" className="btn btn-primary mt-4 inline-flex">
          Catalogue
        </Link>
      </div>
    );
  }

  if (!quiz.data || !modules.data) {
    return (
      <div className="loading-state">
        <div className="spinner" />
        <span>Chargement du quiz…</span>
      </div>
    );
  }

  return (
    <QuizScreen
      key={`${slug}-${index}`}
      slug={slug}
      courseTitle={course.data?.title ?? slug}
      quiz={quiz.data}
      modules={modules.data.modules}
    />
  );
}
