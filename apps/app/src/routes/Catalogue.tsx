import { useQuery } from '@tanstack/react-query';
import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { listCourses, myCourses, mySkills } from '@pupitre/core/lib/api';
import type { Course, MyCourse, SkillLevel } from '@pupitre/core/lib/api-types';
import { useSession } from '@pupitre/core/lib/session';
import { Slider } from '@pupitre/ui/components/ui/slider';
import { ContinueHero } from '@/components/ContinueHero';
import { CourseCard } from '@/components/CourseCard';
import {
  ALL_LEVELS,
  LEVELS,
  levelLabel,
  matchesDifficulty,
  rangeLabel,
  type DifficultyRange,
} from '@/lib/difficulty-range';

import {
  categoryColors,
  categoryLabel,
  DIFFICULTY_LABELS,
  LEVEL_ORDER,
} from '@pupitre/core/lib/course-meta';

// /courses (docs/specs/010-catalogue.md), markup of pupitre's pages/courses/index.astro.

function SearchIcon() {
  return (
    <svg className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft pointer-events-none" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2">
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  );
}

function progressOf(course: MyCourse): number {
  return course.labCount > 0 ? course.completedLabs / course.labCount : 0;
}

/** ?skill= filter: courses of the skill, from the learner's level up. */
function matchesSkill(course: Course, skill: string | null, levels: SkillLevel[]): boolean {
  if (!skill) return true;
  if (!(course.skills ?? []).includes(skill)) return false;
  const entry = levels.find((level) => level.skill === skill);
  if (!entry) return true;
  return (LEVEL_ORDER[course.difficulty] ?? 0) >= (LEVEL_ORDER[entry.level] ?? 1);
}

export function Catalogue() {
  const session = useSession();
  const signedIn = Boolean(session);
  const [params, setParams] = useSearchParams();
  const skill = params.get('skill');

  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [query, setQuery] = useState('');
  const [difficulty, setDifficulty] = useState<DifficultyRange>(ALL_LEVELS);
  const [categories, setCategories] = useState<Set<string>>(new Set());

  // The search applies after 280 ms, like the Astro page.
  useEffect(() => {
    const timer = setTimeout(() => setQuery(deferredSearch), 280);
    return () => clearTimeout(timer);
  }, [deferredSearch]);

  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses, enabled: signedIn });
  const skills = useQuery({ queryKey: ['my', 'skills'], queryFn: mySkills, enabled: signedIn });

  const all = useMemo(() => courses.data?.courses ?? [], [courses.data]);
  const enrolled = useMemo(() => mine.data?.courses ?? [], [mine.data]);
  const enrolledIds = useMemo(() => new Set(enrolled.map((course) => course.slug)), [enrolled]);
  const levels = useMemo(() => skills.data?.skills ?? [], [skills.data]);

  const categorySlugs = useMemo(() => [...new Set(all.map((c) => c.category).filter(Boolean))], [all]);
  const colorOf = useMemo(() => categoryColors(categorySlugs), [categorySlugs]);

  const heroCourse = enrolled.find((c) => progressOf(c) > 0 && progressOf(c) < 1) ?? enrolled[0];

  const visible = useMemo(() => {
    const q = query.toLowerCase().trim();
    return all
      .filter(
        (c) =>
          !q ||
          c.title.toLowerCase().includes(q) ||
          (c.description || '').toLowerCase().includes(q) ||
          (c.category || '').toLowerCase().includes(q),
      )
      .filter((c) => categories.size === 0 || categories.has(c.category))
      .filter((c) => matchesSkill(c, skill, levels))
      .filter((c) => matchesDifficulty(c, difficulty))
      .sort((a, b) => a.title.localeCompare(b.title, 'fr', { sensitivity: 'base' }));
  }, [all, query, categories, skill, levels, difficulty]);

  const filterKey = [query.trim(), difficulty.join('-'), skill ?? '', [...categories].sort().join(',')].join('|');

  const skillEntry = levels.find((level) => level.skill === skill);
  const toggleCategory = (slug: string) =>
    setCategories((current) => {
      const next = new Set(current);
      if (next.has(slug)) next.delete(slug);
      else next.add(slug);
      return next;
    });

  return (
    <div className="cat-outer">
      <div className="page-inner cat-panel">
        <h1 className="cat-page-title">Catalogue</h1>

        <div className="flex items-center gap-2.5 mb-3.5 flex-wrap" data-tour="catalogue-search">
          <div className="relative flex-1 min-w-[160px] max-w-[420px]">
            <SearchIcon />
            <input
              className="input pl-9 w-full h-[38px] text-[.875rem] backdrop-blur-[8px]"
              type="search"
              placeholder="Rechercher un cours…"
              aria-label="Rechercher un cours"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          <div className="cat-diff-range" role="group" aria-label="Difficulté">
            <span className="cat-diff-range-name">Difficulté</span>
            <Slider
              className="cat-diff-range-slider"
              min={1}
              max={LEVELS.length}
              step={1}
              value={difficulty}
              onValueChange={([low = 1, high = LEVELS.length]) => setDifficulty([low, high])}
              thumbLabels={['Difficulté minimale', 'Difficulté maximale']}
              getValueText={levelLabel}
            />
            {/* No aria-live: the focused thumb already announces its level (aria-valuetext). */}
            <span className="cat-diff-range-value">
              {rangeLabel(difficulty)}
            </span>
          </div>
          <span className="cat-count-label">{visible.length > 0 ? `${visible.length} cours` : ''}</span>
        </div>

        {skill && (
          <div className="skill-filter-banner">
            <span>
              🎯 Cours adaptés à ton niveau — {skill}
              {skillEntry ? ` (${DIFFICULTY_LABELS[skillEntry.level] ?? skillEntry.level} et +)` : ''}
            </span>
            <button
              onClick={() => {
                const next = new URLSearchParams(params);
                next.delete('skill');
                setParams(next, { replace: true });
              }}
            >
              ✕ Retirer le filtre
            </button>
          </div>
        )}

        {categorySlugs.length >= 2 && (
          <div className="cat-chips">
            <button className={`chip-reset${categories.size > 0 ? ' has-filter' : ''}`} onClick={() => setCategories(new Set())}>
              Tout
            </button>
            {categorySlugs.map((slug) => {
              const color = colorOf(slug);
              const active = categories.has(slug);
              return (
                <button
                  key={slug}
                  className={`cat-chip${active ? ' active' : ''}`}
                  style={active ? { background: color.dot, borderColor: color.dot } : undefined}
                  aria-pressed={active}
                  onClick={() => toggleCategory(slug)}
                >
                  <span className="chip-dot" style={{ background: active ? '#fff8' : color.dot }} />
                  {categoryLabel(slug)}
                  <span className="chip-count">{all.filter((c) => c.category === slug).length}</span>
                </button>
              );
            })}
          </div>
        )}

        {courses.isPending ? (
          <div className="loading-state">
            <div className="spinner" />
            <span>Chargement des cours…</span>
          </div>
        ) : courses.isError ? (
          <div className="cat-empty">
            <div className="cat-empty-icon">⚠️</div>
            <h3 className="cat-empty-title">Impossible de charger le catalogue</h3>
            <p className="text-soft text-sm mb-4">{courses.error.message}</p>
            <button className="btn btn-primary" onClick={() => courses.refetch()}>
              Réessayer
            </button>
          </div>
        ) : (
          // Marks the end of loading for the guided tour (docs/specs/007-guided-tour.md).
          <div data-tour={signedIn && mine.isPending ? undefined : 'catalogue-loaded'}>
            {heroCourse && <ContinueHero course={heroCourse} />}
            {visible.length === 0 ? (
              <div className="cat-empty">
                <div className="cat-empty-icon">📭</div>
                <h3 className="cat-empty-title">Aucun cours trouvé</h3>
                <p className="text-soft text-sm">Essayez d'ajuster votre recherche ou vos filtres</p>
              </div>
            ) : (
              // Keyed on the filters: a new filter remounts the grid, which replays the cards' entrance.
              <div key={filterKey} className="courses-grid-lg cat-grid">
                {visible.map((course, index) => (
                  <CourseCard
                    key={course.slug}
                    course={course}
                    color={colorOf(course.category)}
                    enrolled={enrolledIds.has(course.slug)}
                    index={index}
                    tour={index === 0}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
