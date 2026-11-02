import { describe, expect, it } from 'vitest';

import { changedValues, SECRET_MASK, SETTING_SECTIONS, settingWarnings, toValues } from '@/lib/settings';

const oidc = SETTING_SECTIONS.find((s) => s.id === 'oidc')!;

describe('platform settings (spec 046)', () => {
  it('does not send an untouched secret', () => {
    const initial = toValues([
      { key: 'oidc_client_secret', value: SECRET_MASK },
      { key: 'oidc_client_id', value: 'pupitre' },
    ]);
    const edited = { ...initial, oidc_client_id: 'pupitre-2' };
    expect(changedValues(oidc, initial, edited)).toEqual({ oidc_client_id: 'pupitre-2' });
  });

  it('sends a secret once a new value is typed', () => {
    const initial = toValues([{ key: 'oidc_client_secret', value: SECRET_MASK }]);
    expect(changedValues(oidc, initial, { ...initial, oidc_client_secret: 's3cret' })).toEqual({ oidc_client_secret: 's3cret' });
    expect(changedValues(oidc, initial, { ...initial, oidc_client_secret: '' })).toEqual({});
  });

  it('only sends the keys of the section', () => {
    const initial = toValues([]);
    const edited = { ...initial, oidc_enabled: 'true', registration_enabled: 'false' };
    expect(changedValues(oidc, initial, edited)).toEqual({ oidc_enabled: 'true' });
  });

  it('warns about the three local-authentication defaults and a disabled OIDC', () => {
    expect(settingWarnings(toValues([])).map((w) => w.key)).toEqual([
      'registration_enabled',
      'sso_local_login_enabled',
      'profile_allow_username_change',
      'oidc_enabled',
    ]);
    const safe = toValues([
      { key: 'registration_enabled', value: 'false' },
      { key: 'sso_local_login_enabled', value: 'false' },
      { key: 'profile_allow_username_change', value: 'false' },
      { key: 'oidc_enabled', value: 'true' },
    ]);
    expect(settingWarnings(safe)).toEqual([]);
  });
});
