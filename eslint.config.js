import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['**/dist', '**/coverage', 'test-results', 'playwright-report', 'apps/app/public/mockServiceWorker.js'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2023,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // docs/specs/003-security.md S1: no raw HTML outside <SafeHtml>.
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
          message: 'Render API data through JSX, or through <SafeHtml> for sanitized markdown.',
        },
        {
          selector: "MemberExpression[property.name=/^(innerHTML|outerHTML)$/]",
          message: 'Build DOM with React; never assign innerHTML.',
        },
      ],
      'no-restricted-globals': [
        'error',
        { name: 'localStorage', message: 'Use @pupitre/ui/lib/storage, which never throws.' },
      ],
    },
  },
  {
    // The only places allowed to touch raw HTML or the storage API directly.
    files: [
      'packages/ui/src/components/safe-html.tsx',
      'packages/ui/src/lib/storage.ts',
      'packages/ui/src/components/ui/**',
      'packages/core/src/lib/markdown.ts',
    ],
    rules: { 'no-restricted-syntax': 'off', 'no-restricted-globals': 'off' },
  },
  {
    // shadcn components export their variants next to the component, by design.
    files: ['packages/ui/src/components/ui/**'],
    rules: { 'react-refresh/only-export-components': 'off' },
  },
  {
    files: ['**/vite.config.ts', '**/vitest.config.ts', 'eslint.config.js'],
    languageOptions: { globals: globals.node },
  },
);
