import type { Badge, CheckStep, CoursePrerequisite, CourseSession, SourceRef } from '../lib/api-types';

// Definitions stored by the Go course-service (definition.Course, content.Module,
// content.Question), as the mock mode keeps them. Only the fields the mock reads.

export type QuestionDefinition = {
  id: string;
  type: 'single' | 'multiple' | 'boolean' | 'order';
  difficulty?: string;
  points?: number;
  question: string;
  answers?: { id: string; text: string; correct?: boolean }[];
  correctAnswer?: boolean;
  items?: { id: string; text: string }[];
  correctOrder?: string[];
  feedback?: { wrong?: string; correct?: string; sourceRefs?: SourceRef[] };
};

export type ModuleDefinition = {
  name: string;
  type: 'text' | 'video' | 'image' | 'quiz' | 'lab' | 'modules';
  content?: string;
  src?: string;
  ref?: string;
  path?: string;
  labUrl?: string;
  hidden?: boolean;
  inline?: boolean;
  prerequisites?: string[];
  questions?: QuestionDefinition[];
  passingScore?: number;
  cooldown?: { strategy: string; baseSeconds: number; multiplier: number; maxSeconds: number };
  maxAttemptsPerQuestion?: number;
  lockOnMaxAttempts?: boolean;
  checkProvider?: 'local' | 'gitlab';
  checkType?: string;
  checkParams?: Record<string, unknown>;
  steps?: CheckStep[];
  skills?: string[];
};

export type CourseDefinition = {
  slug: string;
  spec: {
    title: string;
    description?: string;
    public?: boolean;
    hidden?: boolean;
    category?: string;
    difficulty?: string;
    xpRequired?: number;
    inPerson?: boolean;
    badge?: Badge;
    modules?: ModuleDefinition[];
    prerequisites?: CoursePrerequisite[];
    sessions?: CourseSession[];
  };
};

export type PathDefinition = {
  slug: string;
  spec: {
    title: string;
    description?: string;
    kind?: 'course' | 'skill';
    level?: string;
    courses?: string[];
    skills?: string[];
  };
};
