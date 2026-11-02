import { SiteLink } from './SiteLink';

// Markup of the "Accès refusé" block of the pupitre admin pages.
export function AccessDenied() {
  return (
    <div className="forbidden-page">
      <div className="forbidden-content">
        <div className="forbidden-icon">🔒</div>
        <h2 className="forbidden-title">Accès refusé</h2>
        <p className="text-soft text-sm forbidden-text">
          Vous n'avez pas les droits nécessaires pour accéder à cette page.
        </p>
        <SiteLink to="/courses" className="btn btn-primary">
          Retour au catalogue
        </SiteLink>
      </div>
    </div>
  );
}
