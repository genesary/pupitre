import { useCallback, useEffect, useMemo, useState } from 'react';

import type { Cooldown, QuizAnswer, QuizQuestion, QuizResult } from '@pupitre/core/lib/api-types';
import { readItem, removeItem, writeItem } from '@pupitre/ui/lib/storage';

// Quiz state shared by the quiz page and the inline quiz (docs/specs/013-quizzes.md):
// answers, the answered count, the submit payload, and the per-question
// cooldowns that count down every second.

export type AnswerValue = string | string[] | boolean | null;

function initialAnswer(question: QuizQuestion): AnswerValue {
  if (question.type === 'multiple') return [];
  if (question.type === 'order') return (question.items ?? []).map((item) => item.id);
  if (question.type === 'boolean') return null;
  return '';
}

export function isAnswered(question: QuizQuestion, value: AnswerValue): boolean {
  if (question.type === 'single') return Boolean(value);
  if (question.type === 'multiple' || question.type === 'order') return Array.isArray(value) && value.length > 0;
  if (question.type === 'boolean') return value !== null && value !== undefined;
  return false;
}

function toPayload(questions: QuizQuestion[], answers: Record<string, AnswerValue>): Record<string, QuizAnswer> {
  const payload: Record<string, QuizAnswer> = {};
  for (const question of questions) {
    const value = answers[question.id];
    if (question.type === 'single') payload[question.id] = { single: (value as string) || null };
    else if (question.type === 'multiple') payload[question.id] = { multiple: (value as string[]) ?? [] };
    else if (question.type === 'boolean') payload[question.id] = { boolean: value as boolean | null };
    else payload[question.id] = { order: (value as string[]) ?? [] };
  }
  return payload;
}

type LiveCooldown = { remaining: number; locked: boolean };

/** Kept for a reload (the Astro page used a pcd_ cookie); the server value always wins. */
function storageKey(slug: string, index: number) {
  return `pcd_${slug}_${index}`;
}

function loadStored(slug: string, index: number): Record<string, LiveCooldown> {
  try {
    const data = JSON.parse(readItem('session', storageKey(slug, index)) ?? '{}') as Record<string, number>;
    const now = Date.now();
    const out: Record<string, LiveCooldown> = {};
    for (const [id, expiry] of Object.entries(data)) {
      if (expiry === -1) out[id] = { remaining: 0, locked: true };
      else if (expiry > now) out[id] = { remaining: Math.round((expiry - now) / 1000), locked: false };
    }
    return out;
  } catch {
    return {};
  }
}

function store(slug: string, index: number, cooldowns: Record<string, LiveCooldown>) {
  const now = Date.now();
  const data: Record<string, number> = {};
  for (const [id, cd] of Object.entries(cooldowns)) {
    if (cd.locked) data[id] = -1;
    else if (cd.remaining > 0) data[id] = now + cd.remaining * 1000;
  }
  if (Object.keys(data).length === 0) removeItem('session', storageKey(slug, index));
  else writeItem('session', storageKey(slug, index), JSON.stringify(data));
}

function fromServer(cooldowns: Record<string, Cooldown> | undefined): Record<string, LiveCooldown> {
  return Object.fromEntries(
    Object.entries(cooldowns ?? {}).map(([id, cd]) => [id, { remaining: cd.remainingSeconds, locked: cd.locked }]),
  );
}

export function useQuiz(options: {
  slug: string;
  index: number;
  questions: QuizQuestion[];
  serverCooldowns?: Record<string, Cooldown>;
}) {
  const { slug, index, questions, serverCooldowns } = options;
  const fresh = useCallback(
    () => Object.fromEntries(questions.map((q) => [q.id, initialAnswer(q)])) as Record<string, AnswerValue>,
    [questions],
  );
  const [answers, setAnswers] = useState<Record<string, AnswerValue>>(fresh);
  const [result, setResult] = useState<QuizResult | null>(null);
  const [cooldowns, setCooldowns] = useState<Record<string, LiveCooldown>>(() => {
    const server = fromServer(serverCooldowns);
    return Object.keys(server).length > 0 ? server : loadStored(slug, index);
  });

  const anyRunning = Object.values(cooldowns).some((cd) => cd.remaining > 0);
  useEffect(() => {
    if (!anyRunning) return;
    const timer = setInterval(() => {
      setCooldowns((current) =>
        Object.fromEntries(
          Object.entries(current).map(([id, cd]) => [id, { ...cd, remaining: Math.max(0, cd.remaining - 1) }]),
        ),
      );
    }, 1000);
    return () => clearInterval(timer);
  }, [anyRunning]);

  const startCooldowns = useCallback(
    (next: Record<string, Cooldown> | undefined) => {
      const live = fromServer(next);
      store(slug, index, live);
      setCooldowns(live);
    },
    [slug, index],
  );

  const blocked = (questionId: string) => {
    const cd = cooldowns[questionId];
    return Boolean(cd && (cd.locked || cd.remaining > 0));
  };

  const canRetry = Boolean(result && !result.passed) && !Object.values(cooldowns).some((cd) => cd.locked || cd.remaining > 0);

  const answeredCount = useMemo(
    () => questions.filter((q) => isAnswered(q, answers[q.id] ?? null)).length,
    [questions, answers],
  );

  return {
    answers,
    setAnswer: (questionId: string, value: AnswerValue) => setAnswers((current) => ({ ...current, [questionId]: value })),
    answeredCount,
    payload: () => toPayload(questions, answers),
    result,
    setResult,
    reset: () => {
      setResult(null);
      setAnswers(fresh());
    },
    cooldowns,
    startCooldowns,
    blocked,
    canRetry,
  };
}

export type QuizState = ReturnType<typeof useQuiz>;
