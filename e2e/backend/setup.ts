import { expect, test as setup } from '@playwright/test';

import { ACCOUNTS, adminCall, signIn } from './helpers';

// Puts the platform in the state the other tests expect, and stays correct
// when it already is: Keycloak's pupitre-admins group maps to the admin role,
// and marc.manager has the manager role, given by hand as an admin does it.
// OIDC_GROUP_ADMINS is not set, so the role survives the next sign-in
// (docs/specs/105-backend-authentication.md).

setup('the admin group maps to the admin role', async ({ request }) => {
  const response = await adminCall(request, 'POST', '/admin/groups/mappings', {
    groupName: 'pupitre-admins',
    platformRole: 'admin',
  });
  expect(response.ok(), `mapping: ${response.status()} ${await response.text()}`).toBeTruthy();
});

setup('marc.manager is a manager', async ({ page, request }) => {
  // The account exists in pupitre once it has signed in.
  await signIn(page, ACCOUNTS.manager, '/courses');

  const users = await adminCall(request, 'GET', '/admin/users');
  const { users: list } = (await users.json()) as { users: { id: string; email: string; role: string }[] };
  const marc = list.find((user) => user.email === ACCOUNTS.manager.email);
  expect(marc, 'marc.manager in /api/admin/users').toBeDefined();

  if (marc!.role !== 'manager') {
    const response = await adminCall(request, 'PUT', `/admin/users/${marc!.id}`, { role: 'manager', isActive: true });
    expect(response.ok(), `role change: ${response.status()}`).toBeTruthy();
  }
});
