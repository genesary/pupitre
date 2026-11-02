package handlers

import (
	"net/http"
	"sync"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Platform setting keys read from more than one file; pulled out to satisfy
// goconst.
const (
	settingKeyRegistrationEnabled        = "registration_enabled"
	settingKeySSOLocalLoginEnabled       = "sso_local_login_enabled"
	settingKeyPasswordMinLength          = "password_min_length"
	settingKeyPasswordRequireUppercase   = "password_require_uppercase"
	settingKeyPasswordRequireNumber      = "password_require_number"
	settingKeyOIDCEnabled                = "oidc_enabled"
	settingKeyLDAPEnabled                = "ldap_enabled"
	settingKeyOIDCClientSecret           = "oidc_client_secret"
	settingKeyLDAPBindPassword           = "ldap_bind_password"
	settingRedactedValue                 = "********"
	settingKeyRegistrationEmailWhitelist = "registration_email_whitelist"
	settingKeyProfileAllowUsernameChange = "profile_allow_username_change"
	settingKeyOIDCProviderURL            = "oidc_provider_url"
	settingKeyOIDCIssuerURL              = "oidc_issuer_url"
	settingKeyOIDCRedirectBase           = "oidc_redirect_base"
	settingKeyOIDCBrowserBaseURL         = "oidc_browser_base_url"
	settingKeyOIDCClientID               = "oidc_client_id"
	settingKeyOIDCScopes                 = "oidc_scopes"
	settingKeyOIDCGroupClaim             = "oidc_group_claim"
	settingKeyOIDCGroupAdmins            = "oidc_group_admins"
	settingKeyLDAPServerURL              = "ldap_server_url"
	settingKeyLDAPBindDN                 = "ldap_bind_dn"
	settingKeyLDAPUserBaseDN             = "ldap_user_base_dn"
	settingKeyLDAPUserFilter             = "ldap_user_filter"
)

// keySet is a concurrency-safe string set. A RWMutex guards the underlying
// map so handler goroutines can read concurrently without data races.
type keySet struct {
	mu   sync.RWMutex
	keys map[string]struct{}
}

// contains reports whether key is in the set.
func (s *keySet) contains(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.keys[key]

	return ok
}

// allowedSettingKeys is the set of platform setting keys clients may write.
var allowedSettingKeys = &keySet{ //nolint:gochecknoglobals // handler-wide allow-list for writable settings, guarded by keySet.mu
	keys: map[string]struct{}{
		"gitlab_url":                         {},
		settingKeyRegistrationEnabled:        {},
		settingKeyRegistrationEmailWhitelist: {},
		settingKeyPasswordMinLength:          {},
		settingKeyPasswordRequireUppercase:   {},
		settingKeyPasswordRequireNumber:      {},
		settingKeyProfileAllowUsernameChange: {},
		settingKeySSOLocalLoginEnabled:       {},
		// OIDC — oidc_insecure_skip_verify is intentionally absent: it must not be
		// toggled at runtime via the API (deploy-time env var only).
		settingKeyOIDCEnabled:        {},
		settingKeyOIDCProviderURL:    {},
		settingKeyOIDCIssuerURL:      {},
		settingKeyOIDCRedirectBase:   {},
		settingKeyOIDCBrowserBaseURL: {},
		settingKeyOIDCClientID:       {},
		settingKeyOIDCClientSecret:   {},
		settingKeyOIDCScopes:         {},
		settingKeyOIDCGroupClaim:     {},
		settingKeyOIDCGroupAdmins:    {},
		// LDAP
		settingKeyLDAPEnabled:      {},
		settingKeyLDAPServerURL:    {},
		settingKeyLDAPBindDN:       {},
		settingKeyLDAPBindPassword: {},
		settingKeyLDAPUserBaseDN:   {},
		settingKeyLDAPUserFilter:   {},
		"ldap_group_base_dn":       {},
		"ldap_group_filter":        {},
	},
}

// secretSettingKeys holds setting keys whose values must be redacted in API
// responses to avoid leaking credentials stored in platform_settings.
var secretSettingKeys = &keySet{ //nolint:gochecknoglobals // handler-wide redaction set, guarded by keySet.mu
	keys: map[string]struct{}{
		settingKeyOIDCClientSecret: {},
		settingKeyLDAPBindPassword: {},
	},
}

// PublicSettings godoc
// @Summary  Get public platform settings
// @Tags     Settings
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /api/settings/public [get].
func (s *State) PublicSettings(writer http.ResponseWriter, request *http.Request) {
	keys := []string{
		settingKeyRegistrationEnabled, settingKeySSOLocalLoginEnabled,
		settingKeyPasswordMinLength, settingKeyPasswordRequireUppercase, settingKeyPasswordRequireNumber,
		settingKeyOIDCEnabled, settingKeyLDAPEnabled,
	}

	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = repository.ReadSetting(request.Context(), s.Repos.Settings, key, "")
	}

	s.JSON(writer, http.StatusOK, out)
}

// GetSettings godoc
// @Summary   Get all platform settings (admin)
// @Tags      Admin - Settings
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/settings [get].
func (s *State) GetSettings(writer http.ResponseWriter, request *http.Request) {
	rows, err := s.Repos.Settings.List(request.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type setting struct {
		Key         string  `json:"key"`
		Value       string  `json:"value"`
		Description *string `json:"description"`
	}

	settings := make([]setting, 0, len(rows))

	for _, row := range rows {
		current := setting{Key: row.Key, Value: row.Value, Description: row.Description}

		if secretSettingKeys.contains(current.Key) && current.Value != "" {
			current.Value = settingRedactedValue
		}

		settings = append(settings, current)
	}

	s.JSON(writer, http.StatusOK, map[string]any{"settings": settings})
}

// UpdateSettings godoc
// @Summary   Update platform settings (admin)
// @Tags      Admin - Settings
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  map[string]string  true  "Key/value settings map"
// @Success   200   {object}  map[string]interface{}
// @Failure   400   {object}  map[string]string
// @Router    /api/admin/settings [put].
func (s *State) UpdateSettings(writer http.ResponseWriter, request *http.Request) {
	var body map[string]any

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if len(body) == 0 {
		s.Error(writer, http.StatusBadRequest, "No settings provided")

		return
	}

	for key, value := range body {
		if !allowedSettingKeys.contains(key) {
			s.Error(writer, http.StatusBadRequest, "Unknown setting key: '"+key+"'")

			return
		}

		val, ok := value.(string)
		if !ok {
			s.Error(writer, http.StatusBadRequest, "Setting '"+key+"' must be a string")

			return
		}

		err := s.Repos.Settings.Upsert(request.Context(), key, val)
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Database error")

			return
		}
	}

	s.GetSettings(writer, request)
}
