import { useMutation } from '@tanstack/react-query';
import { Fragment, useEffect, useMemo, useRef, useState } from 'react';

import { checkModule, checkStep, recordLocalCheck } from '@pupitre/core/lib/api';
import type { CheckResult, CheckStep, ModuleDetail } from '@pupitre/core/lib/api-types';
import { MarkdownContent } from '@pupitre/core/components/MarkdownContent';
import { checkErrorMessage } from '@/lib/check-error';
import { isTauri, resolveCheck } from '@/lib/local-check';
import type { PatternMap, TocEntry } from '@pupitre/core/lib/markdown';
import { safeExternalUrl } from '@pupitre/ui/lib/security';

import { renderLab } from '@/lib/render-lab';

// Lab modules (docs/specs/014-labs.md), markup of the lab part of pupitre's
// lesson page. `<!-- check:N -->` markers place the step blocks, which are
// React components between sanitised markdown segments (spec 003 S2).

function CheckOutcome({ result, error }: { result?: CheckResult; error?: string }) {
  if (error) return <div className="check-err">{error}</div>;
  if (!result) return null;
  if (result.allow) return <div className="check-ok">✓ Étape validée !</div>;
  const violations = result.violations ?? [];
  return (
    <div className="check-fail">
      <div className="check-fail-title">✗ Pas encore</div>
      {violations.length > 0 && (
        <ul className="check-fail-items">
          {violations.map((violation, i) => (
            <li key={i} className="mb-1">
              {violation}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function LocalOnly({ slug, index }: { slug: string; index: number }) {
  return (
    <div className="check-local">
      <span>Ce lab se vérifie dans Pupitre.</span>
      <a href={`pupitre://courses/${slug}/modules/${index}`} className="check-local-link">
        Ouvrir dans Pupitre →
      </a>
    </div>
  );
}

type RunCheck = (step: CheckStep | null, stepIndex: number) => Promise<CheckResult>;

function useCheck(run: () => Promise<CheckResult>) {
  return useMutation({ mutationFn: run });
}

function StepBlock({ step, index, run, localOnly, slug, moduleIndex, onResult }: {
  step: CheckStep;
  index: number;
  run: RunCheck;
  localOnly: boolean;
  slug: string;
  moduleIndex: number;
  onResult: (ok: boolean) => void;
}) {
  const [shownLocal, setShownLocal] = useState(false);
  const check = useCheck(() => run(step, index));
  return (
    <div className="step-block">
      <div className="step-block-header">
        <span className="step-block-title">✔ {step.title}</span>
        <button
          className="btn btn-primary text-[.88rem] whitespace-nowrap flex-shrink-0"
          disabled={check.isPending}
          onClick={() => {
            if (localOnly) {
              setShownLocal(true);
              return;
            }
            check.mutate(undefined, { onSuccess: (result) => onResult(result.allow) });
          }}
        >
          {check.isPending ? '…' : 'Vérifier cette étape'}
        </button>
      </div>
      <div className="step-result">
        {shownLocal ? (
          <LocalOnly slug={slug} index={moduleIndex} />
        ) : (
          <CheckOutcome result={check.data} error={checkErrorMessage(check.error)} />
        )}
      </div>
    </div>
  );
}

type LabContentProps = {
  slug: string;
  module: ModuleDetail;
  patterns: PatternMap;
  onTocChange: (toc: TocEntry[]) => void;
  /** Every step is validated: complete the lab and move on. */
  onCompleted: () => void;
};

export function LabContent({ slug, module, patterns, onTocChange, onCompleted }: LabContentProps) {
  const steps = useMemo(() => module.steps ?? [], [module.steps]);
  const localOnly = module.checkProvider === 'local' && !isTauri();
  const lab = useMemo(() => renderLab(module.content ?? '', patterns, steps.length), [module.content, patterns, steps.length]);
  const [validated, setValidated] = useState<boolean[]>(() => steps.map(() => false));
  const done = useRef(false);

  useEffect(() => onTocChange(lab.toc), [lab.toc, onTocChange]);

  const run: RunCheck = (step, stepIndex) => {
    if (module.checkProvider === 'gitlab' && step) return checkStep(slug, module.index, stepIndex);
    return resolveCheck(
      { checkProvider: module.checkProvider, checkType: step?.checkType ?? module.checkType, checkParams: step?.checkParams ?? module.checkParams },
      () => checkModule(slug, module.index),
      (result) => recordLocalCheck(slug, module.index, result),
    );
  };

  const allDone = steps.length > 0 && validated.every(Boolean);
  useEffect(() => {
    if (!allDone || done.current) return;
    done.current = true;
    const timer = setTimeout(onCompleted, 1200);
    return () => clearTimeout(timer);
  }, [allDone, onCompleted]);

  const setStep = (i: number, ok: boolean) => setValidated((current) => current.map((v, j) => (j === i ? ok : v)));
  const unplaced = steps.map((_, i) => i).filter((i) => !lab.placed.has(i));
  const stepBlock = (i: number) => (
    <StepBlock key={`step-${i}`} step={steps[i]!} index={i} run={run} localOnly={localOnly} slug={slug} moduleIndex={module.index} onResult={(ok) => setStep(i, ok)} />
  );

  const single = useCheck(() => run(null, -1));
  const [singleLocal, setSingleLocal] = useState(false);

  const labUrl = safeExternalUrl(module.labUrl);

  return (
    <>
      {labUrl && (
        <p className="mb-4">
          <a href={labUrl} target="_blank" rel="noopener noreferrer" className="btn btn-secondary">
            Ouvrir le lab ↗
          </a>
        </p>
      )}
      {lab.segments.map((segment, i) => (
        <Fragment key={i}>
          <MarkdownContent html={segment.html} />
          {segment.step !== null && stepBlock(segment.step)}
        </Fragment>
      ))}
      {unplaced.map(stepBlock)}

      {steps.length > 0 && allDone && <div className="lab-success-banner">✓ Lab terminé ! Félicitations.</div>}

      {steps.length === 0 && module.hasCheck && (
        <>
          <div className="check-bar">
            <span className="check-bar-hint">Une fois le lab terminé, revenez ici pour valider votre travail.</span>
            <button
              className="btn btn-primary"
              disabled={single.isPending}
              onClick={() => (localOnly ? setSingleLocal(true) : single.mutate())}
            >
              {single.isPending ? 'Vérification…' : 'Vérifier mon travail'}
            </button>
          </div>
          <div className="check-result">
            {singleLocal ? <LocalOnly slug={slug} index={module.index} /> : <CheckOutcome result={single.data} error={checkErrorMessage(single.error)} />}
          </div>
        </>
      )}
    </>
  );
}
