// Loading and error states shared by the pages (docs/specs/005-ui-theme-navigation.md),
// with the markup of the Astro pages.

export function LoadingState({ label = 'Chargement…' }: { label?: string }) {
  return (
    <div className="loading-state">
      <div className="spinner" />
      <span>{label}</span>
    </div>
  );
}

export function LoadError({ error, onRetry }: { error: Error; onRetry: () => void }) {
  return (
    <div className="text-center py-16 px-6">
      <div className="text-[2.5rem] mb-3">⚠️</div>
      <h2 className="text-[1.1rem] mb-2">Impossible de charger la page</h2>
      <p className="text-ink-soft text-[.88rem] mb-5">{error.message}</p>
      <button className="btn btn-primary" onClick={onRetry}>
        Réessayer
      </button>
    </div>
  );
}
