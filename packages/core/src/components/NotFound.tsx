import { SiteLink } from './SiteLink';

// Same look as the "Accès refusé" block (the Astro frontend had no 404 page).
export function NotFound() {
  return (
    <div className="forbidden-page">
      <div className="forbidden-content">
        <div className="forbidden-icon">🔍</div>
        <h2 className="forbidden-title">Page introuvable</h2>
        <p className="text-soft text-sm forbidden-text">Cette page n'existe pas ou a été déplacée.</p>
        <SiteLink to="/courses" className="btn btn-primary">
          Retour au catalogue
        </SiteLink>
      </div>
    </div>
  );
}
