package db

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

const (
	// settingValueTrue is the stored string form of a boolean-true setting.
	settingValueTrue = "true"
	// settingKeyOIDCClientSecret is the platform_settings key for the OIDC
	// client secret, referenced twice: once to seed it, once to redact it
	// from logs.
	settingKeyOIDCClientSecret = "oidc_client_secret"
)

// SeedOIDC bootstraps the OIDC platform settings from deploy-time configuration
// (Helm). It is a no-op unless OIDC_ENABLED=true, which keeps the admin-UI-only
// workflow fully backward compatible: deployments that don't provision OIDC via
// Helm are never touched.
//
// When enabled, the Helm configuration is authoritative — the relevant
// platform_settings rows are upserted on every startup so a `helm upgrade`
// reliably reflects the chart (GitOps). Only non-empty bootstrap values are
// written, so individual fields left blank in the chart fall back to whatever
// is already stored (e.g. the migration defaults or a prior admin edit).
//
// The client secret originates from a mounted Kubernetes Secret (file) or env
// var; it is never hardcoded in the chart values. Rotating it requires a pod
// restart so the new value is re-seeded.
func SeedOIDC(ctx context.Context, settings repository.SettingRepository, cfg *config.Config) error {
	oidcCfg := cfg.OIDC
	if !oidcCfg.Enabled {
		return nil
	}

	// key/value pairs to seed; empty values are skipped below.
	bootstrapValues := map[string]string{
		"oidc_enabled":             settingValueTrue,
		"oidc_provider_url":        oidcCfg.ProviderURL,
		"oidc_issuer_url":          oidcCfg.IssuerURL,
		"oidc_client_id":           oidcCfg.ClientID,
		settingKeyOIDCClientSecret: oidcCfg.ResolveClientSecret(),
		"oidc_scopes":              oidcCfg.Scopes,
		"oidc_group_claim":         oidcCfg.GroupClaim,
		"oidc_group_admins":        oidcCfg.GroupAdmins,
		"oidc_redirect_base":       oidcCfg.RedirectBase,
		"oidc_browser_base_url":    oidcCfg.BrowserBaseURL,
	}
	if oidcCfg.InsecureSkipVerify {
		bootstrapValues["oidc_insecure_skip_verify"] = settingValueTrue
	}

	seeded := make([]string, 0, len(bootstrapValues))
	for key, val := range bootstrapValues {
		if val == "" {
			continue
		}

		err := settings.Upsert(ctx, key, val)
		if err != nil {
			return fmt.Errorf("seed OIDC setting %s: %w", key, err)
		}

		if key != settingKeyOIDCClientSecret {
			seeded = append(seeded, key)
		} else {
			seeded = append(seeded, settingKeyOIDCClientSecret+"(redacted)")
		}
	}

	zap.L().Info("OIDC settings bootstrapped from deploy-time config", zap.Strings("keys", seeded))

	return nil
}
