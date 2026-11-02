// Platform settings of docs/specs/046-admin-settings.md: sections, fields and
// the rules that keep a Keycloak-only platform safe.

export const SECRET_MASK = '********';

export type SettingField = {
  key: string;
  kind: 'toggle' | 'text' | 'number' | 'secret';
  label: string;
  help?: string;
  placeholder?: string;
  /** Value assumed when the server has no row for the key (pupitre's defaults). */
  fallback?: string;
};

export type SettingSection = {
  id: string;
  title: string;
  intro?: string;
  /** Shown folded, with this note: settings without effect with Keycloak only. */
  folded?: string;
  fields: SettingField[];
};

export const SETTING_SECTIONS: SettingSection[] = [
  {
    id: 'sso',
    title: '🔐 SSO & Authentification',
    fields: [
      {
        key: 'sso_local_login_enabled',
        kind: 'toggle',
        label: 'Autoriser la connexion locale',
        help: 'Email + mot de passe. À désactiver : la connexion passe par Keycloak uniquement.',
        fallback: 'true',
      },
    ],
  },
  {
    id: 'registration',
    title: '📝 Inscription',
    fields: [
      {
        key: 'registration_enabled',
        kind: 'toggle',
        label: 'Inscription ouverte',
        help: 'À désactiver : les comptes sont créés par Keycloak.',
        fallback: 'true',
      },
      {
        key: 'registration_email_whitelist',
        kind: 'text',
        label: 'Domaines autorisés (whitelist)',
        help: 'Séparés par des virgules. Laisser vide pour tout autoriser.',
        placeholder: 'company.com, partner.org',
      },
    ],
  },
  {
    id: 'password',
    title: '🔑 Politique de mot de passe',
    folded: 'Sans effet avec Keycloak : les mots de passe sont gérés par Keycloak.',
    fields: [
      { key: 'password_min_length', kind: 'number', label: 'Longueur minimale', fallback: '8' },
      { key: 'password_require_uppercase', kind: 'toggle', label: 'Exiger une majuscule', help: 'Au moins un caractère A–Z.', fallback: 'false' },
      { key: 'password_require_number', kind: 'toggle', label: 'Exiger un chiffre', help: 'Au moins un chiffre 0–9.', fallback: 'false' },
    ],
  },
  {
    id: 'profiles',
    title: '👤 Profils utilisateurs',
    fields: [
      {
        key: 'profile_allow_username_change',
        kind: 'toggle',
        label: 'Autoriser le changement de pseudo',
        help: 'À désactiver : le pseudo vient de Keycloak.',
        fallback: 'true',
      },
    ],
  },
  {
    id: 'oidc',
    title: '🌐 OIDC (OpenID Connect)',
    intro: 'Connexion par Keycloak.',
    fields: [
      {
        key: 'oidc_enabled',
        kind: 'toggle',
        label: "Activer l'authentification OIDC",
        help: 'Affiche le bouton de connexion Keycloak.',
        fallback: 'false',
      },
      { key: 'oidc_provider_url', kind: 'text', label: 'URL de découverte du provider', placeholder: 'https://keycloak.example.com/realms/pupitre' },
      { key: 'oidc_issuer_url', kind: 'text', label: 'Override issuer URL (optionnel)', help: 'Uniquement si la Provider URL est une adresse interne.' },
      { key: 'oidc_redirect_base', kind: 'text', label: 'URL de base pour redirect (optionnel)', placeholder: 'https://elearning.pupitre.io' },
      { key: 'oidc_browser_base_url', kind: 'text', label: 'URL navigateur (optionnel)', help: 'Seulement si la Provider URL est interne.' },
      { key: 'oidc_client_id', kind: 'text', label: 'Client ID', placeholder: 'client-id' },
      { key: 'oidc_client_secret', kind: 'secret', label: 'Client Secret' },
      {
        key: 'oidc_scopes',
        kind: 'text',
        label: 'Scopes',
        help: 'Séparés par des espaces. Inclure groups pour la synchronisation des groupes.',
        placeholder: 'openid email profile groups',
      },
      { key: 'oidc_group_claim', kind: 'text', label: 'Claim des groupes', placeholder: 'groups' },
      { key: 'oidc_group_admins', kind: 'text', label: 'Groupes administrateurs', help: 'Groupes Keycloak dont les membres sont administrateurs.' },
    ],
  },
  {
    id: 'ldap',
    title: '🗂️ LDAP / Active Directory',
    folded: 'Non utilisé avec Keycloak.',
    fields: [
      { key: 'ldap_enabled', kind: 'toggle', label: "Activer l'authentification LDAP", fallback: 'false' },
      { key: 'ldap_server_url', kind: 'text', label: 'URL du serveur', placeholder: 'ldap://ldap.example.com:389' },
      { key: 'ldap_bind_dn', kind: 'text', label: 'DN du compte de service' },
      { key: 'ldap_bind_password', kind: 'secret', label: 'Mot de passe du compte' },
      { key: 'ldap_user_base_dn', kind: 'text', label: 'User base DN' },
      { key: 'ldap_user_filter', kind: 'text', label: 'Filtre de recherche utilisateur', placeholder: '(mail=%s)' },
      { key: 'ldap_group_base_dn', kind: 'text', label: 'Group base DN (optionnel)' },
      { key: 'ldap_group_filter', kind: 'text', label: 'Filtre de recherche groupe' },
    ],
  },
  {
    id: 'gitlab',
    title: 'GitLab',
    fields: [
      { key: 'gitlab_url', kind: 'text', label: 'URL instance GitLab', help: 'Utilisée par les labs.', placeholder: 'https://gitlab.com' },
    ],
  },
];

export type SettingValues = Record<string, string>;

/** Server rows to values, with pupitre's defaults for missing keys. */
export function toValues(rows: { key: string; value: string }[]): SettingValues {
  const values: SettingValues = {};
  for (const field of SETTING_SECTIONS.flatMap((s) => s.fields)) values[field.key] = field.fallback ?? '';
  for (const row of rows) values[row.key] = row.value;
  return values;
}

/**
 * The keys of a section that changed. A secret is sent only when a new value
 * was typed: sending the mask back would overwrite the real secret (defect A24).
 */
export function changedValues(section: SettingSection, initial: SettingValues, edited: SettingValues): SettingValues {
  const out: SettingValues = {};
  for (const field of section.fields) {
    const value = edited[field.key] ?? '';
    if (value === (initial[field.key] ?? '')) continue;
    if (field.kind === 'secret' && (value === '' || value === SECRET_MASK)) continue;
    out[field.key] = value;
  }
  return out;
}

export type SettingWarning = { key: string; message: string; fix: string };

/** Settings that are dangerous for a Keycloak-only platform (finding H7). */
export function settingWarnings(values: SettingValues): SettingWarning[] {
  const warnings: SettingWarning[] = [];
  if (values.registration_enabled !== 'false') {
    warnings.push({ key: 'registration_enabled', message: "L'inscription locale est ouverte.", fix: 'false' });
  }
  if (values.sso_local_login_enabled !== 'false') {
    warnings.push({ key: 'sso_local_login_enabled', message: 'La connexion locale (email + mot de passe) est autorisée.', fix: 'false' });
  }
  if (values.profile_allow_username_change !== 'false') {
    warnings.push({ key: 'profile_allow_username_change', message: 'Les utilisateurs peuvent changer leur pseudo.', fix: 'false' });
  }
  if (values.oidc_enabled !== 'true') {
    warnings.push({ key: 'oidc_enabled', message: "La connexion OIDC (Keycloak) n'est pas activée.", fix: 'true' });
  }
  return warnings;
}
