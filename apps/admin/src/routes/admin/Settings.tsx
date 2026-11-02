import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { adminSettings, updateSettings, type PlatformSetting } from '@pupitre/core/lib/api';
import {
  changedValues,
  SECRET_MASK,
  SETTING_SECTIONS,
  settingWarnings,
  toValues,
  type SettingField,
  type SettingSection,
  type SettingValues,
} from '@/lib/settings';

// /admin/settings (docs/specs/046-admin-settings.md), markup of pupitre's
// admin/settings/index.astro. Each section saves only its changed keys; the
// Astro page sent every key back, masked secrets included (defect A24).

type Settings = { settings: PlatformSetting[] };

function useSaveSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateSettings,
    onSuccess: (data) => {
      queryClient.setQueryData<Settings>(['admin', 'settings'], data);
      // The public settings drive the sign-in page.
      void queryClient.invalidateQueries({ queryKey: ['settings', 'public'] });
      toast.success('Paramètres enregistrés');
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
}

function SecretInput({ field, value, onChange }: { field: SettingField; value: string; onChange: (value: string) => void }) {
  const [editing, setEditing] = useState(value !== SECRET_MASK);
  if (!editing) {
    return (
      <div className="flex gap-2 items-center">
        <input className="input flex-1" id={`f-${field.key}`} type="password" value={SECRET_MASK} disabled readOnly />
        <button
          type="button"
          className="btn btn-secondary btn-sm"
          onClick={() => {
            onChange('');
            setEditing(true);
          }}
        >
          Modifier
        </button>
      </div>
    );
  }
  return (
    <input
      className="input"
      id={`f-${field.key}`}
      type="password"
      autoComplete="new-password"
      placeholder="Nouvelle valeur"
      value={value === SECRET_MASK ? '' : value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

function FieldControl({ field, value, onChange }: { field: SettingField; value: string; onChange: (value: string) => void }) {
  if (field.kind === 'toggle') {
    const on = value === 'true';
    return (
      <div className="setting-row">
        <div className="setting-label">
          <p id={`l-${field.key}`}>{field.label}</p>
          {field.help && <small>{field.help}</small>}
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={on}
          aria-labelledby={`l-${field.key}`}
          className={`toggle${on ? ' on' : ''}`}
          onClick={() => onChange(on ? 'false' : 'true')}
        >
          <span className="toggle-thumb" />
        </button>
      </div>
    );
  }
  return (
    <div>
      <label className="label" htmlFor={`f-${field.key}`}>
        {field.label}
      </label>
      {field.help && <small className="text-soft settings-help">{field.help}</small>}
      {field.kind === 'secret' ? (
        <SecretInput field={field} value={value} onChange={onChange} />
      ) : (
        <input
          className={`input${field.kind === 'number' ? ' w-20 text-center' : ''}`}
          id={`f-${field.key}`}
          type={field.kind === 'number' ? 'number' : 'text'}
          min={field.kind === 'number' ? 4 : undefined}
          max={field.kind === 'number' ? 128 : undefined}
          placeholder={field.placeholder}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </div>
  );
}

function SectionCard({ section, initial }: { section: SettingSection; initial: SettingValues }) {
  const [values, setValues] = useState(initial);
  const save = useSaveSettings();
  const changes = changedValues(section, initial, values);
  const dirty = Object.keys(changes).length > 0;

  const body = (
    <div className="flex flex-col gap-4">
      {section.fields.map((field) => (
        <FieldControl
          key={field.key}
          field={field}
          value={values[field.key] ?? ''}
          onChange={(value) => setValues((current) => ({ ...current, [field.key]: value }))}
        />
      ))}
      {section.id === 'sso' && values.sso_local_login_enabled === 'false' && values.oidc_enabled !== 'true' && (
        <div className="warn-box">⚠️ La connexion locale est désactivée : vérifiez que la connexion OIDC est activée.</div>
      )}
      {section.id === 'oidc' && values.oidc_enabled === 'true' && !values.oidc_provider_url?.trim() && (
        <div className="warn-box">⚠️ OIDC est activé mais aucune Provider URL n'est définie.</div>
      )}
      {section.id === 'oidc' && (
        <div className="info-box">Les groupes OIDC sont mappés aux rôles depuis la page Groupes.</div>
      )}
      <div className="flex justify-end">
        <button className="btn btn-primary" disabled={!dirty || save.isPending} onClick={() => save.mutate(changes)}>
          {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
        </button>
      </div>
    </div>
  );

  if (section.folded) {
    return (
      <details className="card mb-5">
        <summary className="cursor-pointer">
          <h2 className="inline">{section.title}</h2>
          <span className="text-soft text-sm ml-2">— {section.folded}</span>
        </summary>
        <div className="mt-4">{body}</div>
      </details>
    );
  }

  return (
    <div className="card mb-5">
      <h2>{section.title}</h2>
      {section.intro && <p className="text-soft text-sm -mt-2.5 mb-3.5">{section.intro}</p>}
      {body}
    </div>
  );
}

export function Settings() {
  const settings = useQuery({ queryKey: ['admin', 'settings'], queryFn: adminSettings });
  const fix = useSaveSettings();
  const values = toValues(settings.data?.settings ?? []);
  const warnings = settingWarnings(values);

  return (
    <>
      <div className="flex items-center justify-between mb-7">
        <div>
          <h1 className="text-2xl">Paramètres</h1>
          <p className="text-soft text-sm mt-1">Configuration runtime — les changements prennent effet immédiatement.</p>
        </div>
      </div>

      {settings.isPending ? (
        <div className="loading-state !py-12 !px-0">
          <div className="spinner" />
          <span className="text-soft text-sm">Chargement…</span>
        </div>
      ) : settings.isError ? (
        <p className="text-bad p-6">{settings.error.message}</p>
      ) : (
        <div className="max-w-[680px]">
          {warnings.length > 0 && (
            <div className="warn-box mb-5" role="alert">
              <p className="font-semibold mb-2">⚠️ Réglages à corriger pour une connexion par Keycloak uniquement :</p>
              <ul className="flex flex-col gap-2">
                {warnings.map((warning) => (
                  <li key={warning.key} className="flex items-center justify-between gap-3">
                    <span>{warning.message}</span>
                    <button
                      className="btn btn-secondary btn-sm shrink-0"
                      disabled={fix.isPending}
                      onClick={() => fix.mutate({ [warning.key]: warning.fix })}
                    >
                      Corriger
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {/* Remounted when the server values change, so each form starts from them. */}
          {SETTING_SECTIONS.map((section) => (
            <SectionCard key={`${section.id}-${settings.dataUpdatedAt}`} section={section} initial={values} />
          ))}
        </div>
      )}
    </>
  );
}
