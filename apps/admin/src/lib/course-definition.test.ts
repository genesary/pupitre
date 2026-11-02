import { afterEach, describe, expect, it, vi } from 'vitest';

import { courseDefinition, updateCourse } from '@pupitre/core/lib/api';
import {
  cleanModule,
  DefinitionChangedError,
  metaOf,
  removeModule,
  sameSpec,
  saveDefinition,
  setModule,
  toYaml,
  withMeta,
} from '@/lib/course-definition';

vi.mock('@pupitre/core/lib/api', () => ({ courseDefinition: vi.fn(), updateCourse: vi.fn() }));

const spec = {
  title: 'Linux',
  public: true,
  modules: [
    { name: 'Intro', type: 'text', content: '## Intro' },
    { name: 'Quiz', type: 'quiz', questions: [{ id: 'q1' }] },
  ],
  sessions: { s1: { title: 'Paris' } },
};

describe('course definition (spec 042)', () => {
  afterEach(() => vi.clearAllMocks());

  it('compares specs whatever the key order', () => {
    expect(sameSpec({ title: 'a', public: true }, { public: true, title: 'a' })).toBe(true);
    expect(sameSpec({ title: 'a' }, { title: 'b' })).toBe(false);
  });

  it('writes the change on top of the definition read again', async () => {
    vi.mocked(courseDefinition).mockResolvedValue({ slug: 'linux', spec });
    vi.mocked(updateCourse).mockResolvedValue({ slug: 'linux' });
    const written = await saveDefinition('linux', spec, (s) => removeModule(s, 0));
    expect(updateCourse).toHaveBeenCalledWith('linux', { ...spec, modules: [spec.modules[1]] });
    expect(written.sessions).toEqual(spec.sessions);
  });

  it('refuses to overwrite a definition changed since it was opened', async () => {
    vi.mocked(courseDefinition).mockResolvedValue({ slug: 'linux', spec: { ...spec, title: 'Linux 2' } });
    await expect(saveDefinition('linux', spec, (s) => s)).rejects.toBeInstanceOf(DefinitionChangedError);
    expect(updateCourse).not.toHaveBeenCalled();
  });

  it('changes the metadata and keeps the modules and unknown fields', () => {
    const next = withMeta(spec, { ...metaOf(spec), title: ' Linux avancé ', xpRequired: 200, badgeName: 'Pingouin', badgeIcon: '🐧' });
    expect(next.title).toBe('Linux avancé');
    expect(next.xpRequired).toBe(200);
    expect(next.badge).toEqual({ name: 'Pingouin', icon: '🐧' });
    expect(next.modules).toBe(spec.modules);
    expect(next.sessions).toBe(spec.sessions);
    expect('hidden' in next).toBe(false);
  });

  it('keeps either the markdown or the git source of a module', () => {
    expect(cleanModule({ name: 'A', type: 'text', content: '# x', src: 'https://git.example/repo', ref: '', path: 'a.md' })).toEqual({
      name: 'A',
      type: 'text',
      src: 'https://git.example/repo',
      ref: 'main',
      path: 'a.md',
    });
    expect(cleanModule({ name: 'B', type: 'text', content: '# y', src: '', ref: 'main', path: '', inline: true })).toEqual({
      name: 'B',
      type: 'text',
      content: '# y',
    });
  });

  it('adds and replaces modules by position', () => {
    const added = setModule(spec, null, { name: 'Lab', type: 'lab' });
    expect(added.modules?.map((m) => m.name)).toEqual(['Intro', 'Quiz', 'Lab']);
    expect(setModule(spec, 0, { name: 'Intro 2', type: 'text' }).modules?.[0]?.name).toBe('Intro 2');
  });

  it('writes YAML with block strings', () => {
    expect(toYaml({ title: 'Linux', public: true, modules: [{ name: 'Intro', content: 'line 1\nline 2' }] })).toBe(
      ['title: Linux', 'public: true', 'modules:', '  - name: Intro', '    content: |', '      line 1', '      line 2'].join('\n'),
    );
    expect(toYaml({ title: 'Quiz: bases', n: '01' })).toBe('title: "Quiz: bases"\nn: "01"');
  });
});
