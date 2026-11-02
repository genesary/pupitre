import { createBrowserRouter, Navigate, type RouteObject } from 'react-router';

import { Layout } from '@pupitre/core/components/Layout';
import { NotFound } from '@pupitre/core/components/NotFound';
import { RequireRole } from '@pupitre/core/components/RequireAuth';
import { AdminLayout } from '@/routes/AdminLayout';
import { Attendance } from '@/routes/Attendance';
import { AdminCourses } from '@/routes/admin/AdminCourses';
import { AdminGroups } from '@/routes/admin/AdminGroups';
import { AdminLeaderboard } from '@/routes/admin/AdminLeaderboard';
import { AdminPaths } from '@/routes/admin/AdminPaths';
import { AdminUsers } from '@/routes/admin/AdminUsers';
import { Dashboard } from '@/routes/admin/Dashboard';
import { Exports } from '@/routes/admin/Exports';
import { Labs } from '@/routes/admin/Labs';
import { Monitoring } from '@/routes/admin/Monitoring';
import { Patterns } from '@/routes/admin/Patterns';
import { Settings } from '@/routes/admin/Settings';
import { ManagerGroups } from '@/routes/manager/ManagerGroups';
import { ManagerLayout } from '@/routes/manager/ManagerLayout';
import { ManagerPaths } from '@/routes/manager/ManagerPaths';
import { ManagerUsers } from '@/routes/manager/ManagerUsers';

// Route table of the admin app (docs/specs/002-roles-and-guards.md): /admin/*
// and /manager/*. Every other path belongs to the learner app: links to it are
// page loads (SiteLink in @pupitre/core).

const adminOnlyRoutes: RouteObject[] = [
  { index: true, element: <Dashboard /> },
  { path: 'monitoring', element: <Monitoring /> },
  { path: 'leaderboard', element: <AdminLeaderboard /> },
  { path: 'users', element: <AdminUsers /> },
  { path: 'courses', element: <AdminCourses /> },
  { path: 'paths', element: <AdminPaths /> },
  { path: 'groups', element: <AdminGroups /> },
  { path: 'patterns', element: <Patterns /> },
  { path: 'settings', element: <Settings /> },
  { path: 'labs', element: <Labs /> },
];

const routes: RouteObject[] = [
  {
    path: '/',
    element: <Layout />,
    children: [
      {
        path: 'manager',
        element: (
          <RequireRole roles={['manager']}>
            <ManagerLayout />
          </RequireRole>
        ),
        children: [
          { index: true, element: <Navigate to="/manager/users" replace /> },
          { path: 'users', element: <ManagerUsers /> },
          { path: 'groups', element: <ManagerGroups /> },
          { path: 'paths', element: <ManagerPaths /> },
        ],
      },

      // The attendance page has no admin sidebar, like the Astro page.
      {
        path: 'admin/courses/sessions/:slug/:sessionId',
        element: (
          <RequireRole roles={['admin', 'manager']}>
            <Attendance />
          </RequireRole>
        ),
      },

      {
        path: 'admin',
        element: (
          <RequireRole roles={['admin', 'manager']}>
            <AdminLayout />
          </RequireRole>
        ),
        children: [
          { path: 'exports', element: <Exports /> },
          { element: <RequireRole roles={['admin']} />, children: adminOnlyRoutes },
        ],
      },

      { path: '*', element: <NotFound /> },
    ],
  },
];

export function createRouter() {
  return createBrowserRouter(routes);
}
