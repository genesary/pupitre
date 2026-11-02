import { useState } from 'react';

import type { QuizQuestion } from '@pupitre/core/lib/api-types';
import { SafeHtml } from '@pupitre/ui/components/safe-html';
import { renderInline } from '@pupitre/core/lib/markdown';

import type { AnswerValue } from '@/lib/use-quiz';

// Question options, cooldown banner and review of the pupitre quiz screens
// (pages/courses/[slug]/quiz/[index].astro and the lesson's mini-quiz), same
// markup and classes. Texts are sanitised inline markdown (spec 013).

export function InlineText({ text, as = 'span', className }: { text: string; as?: 'span' | 'div' | 'p'; className?: string }) {
  return <SafeHtml as={as} html={renderInline(text)} className={className} />;
}

type OptionsProps = {
  question: QuizQuestion;
  value: AnswerValue;
  disabled: boolean;
  onChange: (value: AnswerValue) => void;
};

function OrderOptions({ question, value, disabled, onChange }: OptionsProps) {
  const items = question.items ?? [];
  const order = Array.isArray(value) ? value : items.map((item) => item.id);
  const [dragged, setDragged] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);

  const move = (from: number, to: number) => {
    if (to < 0 || to >= order.length) return;
    const next = [...order];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item!);
    onChange(next);
  };

  return (
    <div className="quiz-options">
      {order.map((id, idx) => {
        const item = items.find((i) => i.id === id);
        return (
          <div
            key={id}
            className={`order-item${dragged === id ? ' dragging' : ''}${over === id ? ' drag-over' : ''}`}
            draggable={!disabled}
            onDragStart={() => setDragged(id)}
            onDragOver={(event) => {
              event.preventDefault();
              setOver(id);
            }}
            onDragLeave={() => setOver(null)}
            onDrop={(event) => {
              event.preventDefault();
              setOver(null);
              const from = dragged ? order.indexOf(dragged) : -1;
              setDragged(null);
              if (from >= 0) move(from, idx);
            }}
            onDragEnd={() => {
              setDragged(null);
              setOver(null);
            }}
          >
            <span className="drag-handle">⠿</span>
            <span className="order-num">{idx + 1}.</span>
            <span className="order-text">
              <InlineText text={item?.text ?? id} />
            </span>
            <div className="order-btns">
              <button type="button" className="order-move-btn" aria-label="Monter" disabled={disabled || idx === 0} onClick={() => move(idx, idx - 1)}>
                ↑
              </button>
              <button type="button" className="order-move-btn" aria-label="Descendre" disabled={disabled || idx === order.length - 1} onClick={() => move(idx, idx + 1)}>
                ↓
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function QuestionOptions(props: OptionsProps) {
  const { question, value, disabled, onChange } = props;

  if (question.type === 'single') {
    return (
      <div className="quiz-options">
        {(question.answers ?? []).map((answer) => (
          <button
            key={answer.id}
            type="button"
            className={`quiz-option${value === answer.id ? ' selected' : ''}`}
            disabled={disabled}
            aria-pressed={value === answer.id}
            onClick={() => onChange(answer.id)}
          >
            <span className="radio-circle" />
            <InlineText text={answer.text} />
          </button>
        ))}
      </div>
    );
  }

  if (question.type === 'multiple') {
    const selected = Array.isArray(value) ? value : [];
    return (
      <div className="quiz-options">
        {(question.answers ?? []).map((answer) => {
          const checked = selected.includes(answer.id);
          return (
            <button
              key={answer.id}
              type="button"
              className={`quiz-option${checked ? ' selected' : ''}`}
              disabled={disabled}
              aria-pressed={checked}
              onClick={() => onChange(checked ? selected.filter((id) => id !== answer.id) : [...selected, answer.id])}
            >
              <span className={`quiz-cb ${checked ? 'checked' : ''}`}>{checked && <span className="quiz-cb-check">✓</span>}</span>
              <InlineText text={answer.text} />
            </button>
          );
        })}
      </div>
    );
  }

  if (question.type === 'boolean') {
    return (
      <div className="flex gap-3">
        {[
          { v: true, label: 'Vrai' },
          { v: false, label: 'Faux' },
        ].map((option) => (
          <button
            key={option.label}
            type="button"
            className={`quiz-option flex-1 justify-center${value === option.v ? ' selected' : ''}`}
            disabled={disabled}
            aria-pressed={value === option.v}
            onClick={() => onChange(option.v)}
          >
            {option.label}
          </button>
        ))}
      </div>
    );
  }

  return <OrderOptions {...props} />;
}

export function CooldownBanner({ cooldown }: { cooldown?: { remaining: number; locked: boolean } }) {
  if (!cooldown || (!cooldown.locked && cooldown.remaining <= 0)) return null;
  return (
    <div className="cooldown-banner">
      {cooldown.locked ? '🔒 Question verrouillée' : `⏳ Réessayez dans ${cooldown.remaining}s`}
    </div>
  );
}
