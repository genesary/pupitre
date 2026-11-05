import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { adminSettings, updateSettings } from '@pupitre/core/lib/api';
import { Settings } from '@/routes/admin/Settings';

vi.mock('@pupitre/core/lib/api', () => ({ adminSettings: vi.fn(), updateSettings: vi.fn() }));

const rows = (values: Record<string, string>) =>
  Object.entries(values).map(([key, value]) => ({ key, value, description: null }));

function renderPage(values: Record<string, string>) {
  vi.mocked(adminSettings).mockResolvedValue({ settings: rows(values) });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <Settings />
    </QueryClientProvider>,
  );
}

describe('settings', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('keeps what is typed in a section when another one is saved', async () => {
    const user = userEvent.setup();
    renderPage({ registration_email_whitelist: '', gitlab_url: '' });
    vi.mocked(updateSettings).mockResolvedValue({ settings: rows({ registration_email_whitelist: '', gitlab_url: 'https://gitlab.test' }) });

    const whitelist = await screen.findByLabelText('Domaines autorisés (whitelist)');
    await user.type(whitelist, 'company.com');

    await user.type(screen.getByLabelText('URL instance GitLab'), 'https://gitlab.test');
    const card = screen.getByRole('heading', { name: 'GitLab' }).parentElement!;
    await user.click(card.querySelector<HTMLButtonElement>('button.btn-primary')!);

    await waitFor(() => expect(updateSettings).toHaveBeenCalledWith({ gitlab_url: 'https://gitlab.test' }, expect.anything()));
    await waitFor(() => expect(screen.getByLabelText('URL instance GitLab')).toHaveValue('https://gitlab.test'));
    expect(screen.getByLabelText('Domaines autorisés (whitelist)')).toHaveValue('company.com');
  });

  it('shows a saved secret masked again', async () => {
    const user = userEvent.setup();
    renderPage({ oidc_client_secret: '********' });
    vi.mocked(updateSettings).mockResolvedValue({ settings: rows({ oidc_client_secret: '********' }) });

    await screen.findByRole('heading', { name: /OIDC/ });
    const card = screen.getByRole('heading', { name: /OIDC/ }).parentElement!;
    await user.click(card.querySelector<HTMLButtonElement>('button.btn-secondary')!);
    await user.type(screen.getByLabelText('Client Secret'), 'new-secret');
    await user.click(card.querySelector<HTMLButtonElement>('button.btn-primary')!);

    await waitFor(() => expect(screen.getByLabelText('Client Secret')).toBeDisabled());
  });
});
