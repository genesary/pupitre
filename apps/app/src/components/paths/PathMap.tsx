import { Link } from 'react-router';

import type { MyPath, ProgressStatus } from '@pupitre/core/lib/api-types';
import { setCourseReturn } from '@/lib/course-return';
import { MAP, nodePosition, roadPath, viewBox, wrapLabel } from '@/lib/path-map';
import { useTheme } from '@pupitre/ui/lib/theme';

// The SVG progress map of a learning path (docs/specs/020-learner-space.md),
// drawn like pupitre's my-paths.astro, with its light and dark colours.

type StepTheme = { fill: string; text: string; badge: string; badgeText: string; label: string };

const THEMES: Record<'light' | 'dark', Record<ProgressStatus, StepTheme>> = {
  light: {
    completed: { fill: '#bbf7d0', text: '#15803d', badge: '#dcfce7', badgeText: '#15803d', label: 'Terminé' },
    available: { fill: '#bfdbfe', text: '#1d4ed8', badge: '#dbeafe', badgeText: '#1d4ed8', label: 'Disponible' },
    locked: { fill: '#f1f5f9', text: '#94a3b8', badge: '#f1f5f9', badgeText: '#94a3b8', label: 'Verrouillé' },
  },
  dark: {
    completed: { fill: '#0d2218', text: '#4ade80', badge: '#0d2218', badgeText: '#4ade80', label: 'Terminé' },
    available: { fill: '#162336', text: '#60a5fa', badge: '#162336', badgeText: '#60a5fa', label: 'Disponible' },
    locked: { fill: '#1a1d24', text: '#8b92a0', badge: '#1a1d24', badgeText: '#8b92a0', label: 'Verrouillé' },
  },
};

const SHORT_NAMES: Record<string, string> = {
  'linux-intro': 'Linux',
  'docker-fundamentals': 'Docker',
  'kubernetes-basics': 'Kubernetes',
  'networking-basics': 'Réseaux',
  'cybersecurity-intro': 'Cybersécurité',
};

function stepName(slug: string): string {
  return (
    SHORT_NAMES[slug] ??
    slug
      .split('-')
      .map((word) => (word[0]?.toUpperCase() ?? '') + word.slice(1))
      .join(' ')
  );
}

const FONT = 'system-ui,sans-serif';
const BADGE_WIDTH = 64;
const BADGE_HEIGHT = 14;
const LINE_HEIGHT = 11;
// The width of a step's click area: the 16-character label lines are the widest part.
const HIT_WIDTH = 100;

export function PathMap({ path }: { path: MyPath }) {
  const dark = useTheme() === 'dark';
  const theme = THEMES[dark ? 'dark' : 'light'];
  const circleFill = dark ? '#1e2128' : 'white';
  const roadStroke = dark ? '#252830' : '#e2e8f0';
  const nameFill = dark ? '#dde1ea' : '#334155';

  const steps = path.courses ?? [];
  const isSkillPath = path.kind === 'skill';
  const total = steps.length;
  const done = steps.filter((s) => s.status === 'completed').length;
  const allDone = total > 0 && done === total;
  const points = steps.map((_, i) => nodePosition(i));
  const { width, height } = viewBox(total);

  const progressEnd = Math.min(done < total ? done : done - 1, total - 1);
  const showProgress = done > 0 || steps[0]?.status === 'available';
  const progress = showProgress ? roadPath(points.slice(0, progressEnd + 1)) : '';
  const progressColour = allDone ? '#4ade80' : dark ? '#3b82f6' : '#93c5fd';

  return (
    <svg width="100%" viewBox={`0 0 ${width} ${height}`} xmlns="http://www.w3.org/2000/svg" role="img" aria-label={`Progression : ${done}/${total}`}>
      <path d={roadPath(points)} fill="none" stroke={roadStroke} strokeWidth={MAP.road} strokeLinecap="round" strokeLinejoin="round" />
      {progress && (
        <path d={progress} fill="none" stroke={progressColour} strokeWidth={MAP.road} strokeLinecap="round" strokeLinejoin="round" />
      )}
      {steps.map((step, i) => {
        const { x, y } = points[i]!;
        const th = theme[step.status] ?? theme.locked;
        const locked = step.status === 'locked';
        const nameY = y + MAP.nodeRadius + 10;
        const badgeY = nameY + 2 * LINE_HEIGHT + 3;
        const r = MAP.nodeRadius;
        const node = (
          <>
            {/* An invisible area over the whole step, so a click between the circle and the
                name still opens it: the gap between the shapes caught no click. */}
            <rect
              x={x - HIT_WIDTH / 2}
              y={y - r - 3}
              width={HIT_WIDTH}
              height={badgeY + BADGE_HEIGHT - (y - r - 3)}
              fill="transparent"
            />
            {step.status === 'available' && (
              <circle cx={x} cy={y} r={r + 5} fill="none" stroke={th.fill} strokeWidth="2.5" opacity="0">
                <animate attributeName="r" values={`${r + 3};${r + 20}`} dur="2s" repeatCount="indefinite" />
                <animate attributeName="opacity" values="0.35;0" dur="2s" repeatCount="indefinite" />
              </circle>
            )}
            <circle cx={x} cy={y} r={r + 3} fill={circleFill} />
            <circle cx={x} cy={y} r={r} fill={th.fill} />
            <text
              x={x}
              y={y}
              textAnchor="middle"
              dominantBaseline="middle"
              fontSize={step.status === 'completed' ? 14 : 12}
              fontWeight="800"
              fill={th.text}
              fontFamily={FONT}
            >
              {step.status === 'completed' ? '✓' : i + 1}
            </text>
            {locked && (
              <>
                <circle cx={x + r - 5} cy={y - r + 5} r="9" fill={circleFill} stroke={roadStroke} strokeWidth="1" />
                <text x={x + r - 5} y={y - r + 5} textAnchor="middle" dominantBaseline="middle" fontSize="9">
                  🔒
                </text>
              </>
            )}
            {wrapLabel(stepName(step.slug), 16).map((line, li) => (
              <text
                key={li}
                x={x}
                y={nameY + li * LINE_HEIGHT}
                textAnchor="middle"
                dominantBaseline="hanging"
                fontSize="9"
                fontWeight="500"
                fill={nameFill}
                fontFamily={FONT}
              >
                {line}
              </text>
            ))}
            <rect x={x - BADGE_WIDTH / 2} y={badgeY} width={BADGE_WIDTH} height={BADGE_HEIGHT} rx={BADGE_HEIGHT / 2} fill={th.badge} />
            <text
              x={x}
              y={badgeY + BADGE_HEIGHT / 2}
              textAnchor="middle"
              dominantBaseline="middle"
              fontSize="7"
              fontWeight="600"
              fill={th.badgeText}
              fontFamily={FONT}
            >
              {th.label}
            </text>
          </>
        );
        if (locked) return <g key={step.slug}>{node}</g>;
        return (
          <Link
            key={step.slug}
            to={isSkillPath ? `/skills/${step.slug}` : `/courses/${step.slug}`}
            className="cursor-pointer"
            aria-label={`${stepName(step.slug)} — ${th.label}`}
            onClick={() => {
              if (!isSkillPath) setCourseReturn(`/my-paths?path=${encodeURIComponent(path.slug)}`);
            }}
          >
            {node}
          </Link>
        );
      })}
    </svg>
  );
}
