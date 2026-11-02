import { createBrowserRouter, Navigate, type RouteObject } from 'react-router';

import { RequireAuth } from '@pupitre/core/components/RequireAuth';
import { LearnerLayout } from '@/components/tour/LearnerLayout';
import { AuthCallback } from '@/routes/AuthCallback';
import { Login } from '@/routes/Login';
import { Catalogue } from '@/routes/Catalogue';
import { CourseDetail } from '@/routes/CourseDetail';
import { Leaderboard } from '@/routes/Leaderboard';
import { PublicProfile } from '@/routes/PublicProfile';
import { MyCourses } from '@/routes/MyCourses';
import { MyPaths } from '@/routes/MyPaths';
import { MySkills } from '@/routes/MySkills';
import { SkillPage } from '@/routes/SkillPage';
import { MyBadges } from '@/routes/MyBadges';
import { MyXp } from '@/routes/MyXp';
import { MyGroups } from '@/routes/MyGroups';
import { Profile } from '@/routes/Profile';
import { Agenda } from '@/routes/Agenda';
import { NotFound } from '@pupitre/core/components/NotFound';

// Route table of the learner app (docs/specs/002-roles-and-guards.md). The
// administration and the manager space are the admin app (apps/admin).

const learnerRoutes: RouteObject[] = [
  // Loaded on demand: the lesson player and the quiz bring the markdown
  // pipeline, which the catalogue does not need.
  {
    path: 'courses/:slug/lessons/:index',
    lazy: async () => ({ Component: (await import('@/routes/LessonPage')).LessonPage }),
  },
  {
    path: 'courses/:slug/quiz/:index',
    lazy: async () => ({ Component: (await import('@/routes/QuizPage')).QuizPage }),
  },
  { path: 'skills/:slug', element: <SkillPage /> },
  { path: 'my-courses', element: <MyCourses /> },
  { path: 'my-paths', element: <MyPaths /> },
  { path: 'my-skills', element: <MySkills /> },
  { path: 'my-badges', element: <MyBadges /> },
  { path: 'my-xp', element: <MyXp /> },
  { path: 'my-groups', element: <MyGroups /> },
  { path: 'profile', element: <Profile /> },
  { path: 'agenda', element: <Agenda /> },
];

const routes: RouteObject[] = [
  { path: '/login', element: <Login /> },
  { path: '/auth/callback', element: <AuthCallback /> },
  { path: '/register', element: <Navigate to="/login" replace /> },
  {
    path: '/',
    element: <LearnerLayout />,
    children: [
      { index: true, element: <Navigate to="/courses" replace /> },
      { path: 'dashboard', element: <Navigate to="/courses" replace /> },

      // Public pages.
      { path: 'courses', element: <Catalogue /> },
      { path: 'courses/:slug', element: <CourseDetail /> },
      { path: 'leaderboard', element: <Leaderboard /> },
      { path: 'users/:id', element: <PublicProfile /> },

      { element: <RequireAuth />, children: learnerRoutes },

      // /admin/* and /manager/* belong to the admin app (apps/admin), which the
      // dev proxy and the production reverse proxy serve on the same origin.

      { path: '*', element: <NotFound /> },
    ],
  },
];

export function createRouter(extraRoutes: RouteObject[] = []) {
  return createBrowserRouter([...extraRoutes, ...routes]);
}
