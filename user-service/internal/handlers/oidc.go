package handlers

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/genesary/pupitre/internal/httpx"
	"github.com/genesary/pupitre/internal/utils"
	"github.com/genesary/pupitre/user-service/internal/repository"
)

// oidcProviderKey identifies the dedicated single-instance OIDC login flow,
// as distinct from the generic multi-provider OAuth flow in oauth.go.
const oidcProviderKey = "oidc"

// oidcInvalidStateMsg is returned for every way an OIDC callback can fail to
// prove it belongs to a flow this server started, whether that is the state
// token or the PKCE verifier that backs it.
const oidcInvalidStateMsg = "Invalid or expired OIDC state"

// oidcSettings holds the platform-configured settings for the dedicated OIDC
// provider instance, as read from platform_settings by loadOIDCSettings.
type oidcSettings struct {
	Enabled            bool
	ProviderURL        string
	IssuerURL          string // when set, bypasses issuer check (split-horizon: internal discovery URL ≠ public issuer)
	ClientID           string
	ClientSecret       string
	Scopes             []string
	GroupClaim         string
	GroupAdmins        []string // SSO groups that automatically grant admin role
	RedirectBase       string   // overrides config.OAuthRedirectBase for redirect_uri sent to the provider
	BrowserBaseURL     string   // optional: rewrite internal base URL to this for browser redirects
	InsecureSkipVerify bool     // skip TLS verification for custom CA / self-signed OIDC provider
}

// loadOIDCSettings reads OIDC configuration from platform settings and
// validates that OIDC is enabled and fully configured.
func (s *State) loadOIDCSettings(ctx context.Context) (oidcSettings, error) {
	cfg := oidcSettings{
		Enabled:            repository.ReadSetting(ctx, s.Repos.Settings, "oidc_enabled", "false") == authSettingTrue,
		ProviderURL:        repository.ReadSetting(ctx, s.Repos.Settings, "oidc_provider_url", ""),
		IssuerURL:          repository.ReadSetting(ctx, s.Repos.Settings, "oidc_issuer_url", ""),
		ClientID:           repository.ReadSetting(ctx, s.Repos.Settings, "oidc_client_id", ""),
		ClientSecret:       repository.ReadSetting(ctx, s.Repos.Settings, "oidc_client_secret", ""),
		GroupClaim:         repository.ReadSetting(ctx, s.Repos.Settings, "oidc_group_claim", "groups"),
		GroupAdmins:        utils.SplitTrimmedCommaList(repository.ReadSetting(ctx, s.Repos.Settings, "oidc_group_admins", "")),
		RedirectBase:       repository.ReadSetting(ctx, s.Repos.Settings, "oidc_redirect_base", s.Config.OAuthRedirectBase),
		BrowserBaseURL:     repository.ReadSetting(ctx, s.Repos.Settings, "oidc_browser_base_url", ""),
		InsecureSkipVerify: repository.ReadSetting(ctx, s.Repos.Settings, "oidc_insecure_skip_verify", "false") == authSettingTrue,
	}

	scopes := repository.ReadSetting(ctx, s.Repos.Settings, "oidc_scopes", "openid email profile groups")
	for sc := range strings.SplitSeq(scopes, " ") {
		if sc = strings.TrimSpace(sc); sc != "" {
			cfg.Scopes = append(cfg.Scopes, sc)
		}
	}

	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{gooidc.ScopeOpenID, oauthFieldEmail, oauthScopeProfile}
	}

	if !cfg.Enabled {
		return cfg, errors.New("OIDC is not enabled")
	}

	if cfg.ProviderURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return cfg, errors.New("OIDC not fully configured (provider_url, clientId, clientSecret required)")
	}

	if cfg.InsecureSkipVerify {
		// Forbid when no separate IssuerURL is set: the ProviderURL IS the public
		// issuer, so a real certificate is expected and skipping TLS is wrong.
		if cfg.IssuerURL == "" {
			return cfg, errors.New(
				"oidc_insecure_skip_verify cannot be used without oidc_issuer_url; " +
					"supply a separate issuer URL (split-horizon setup) or disable TLS skip",
			)
		}

		zap.L().Warn("OIDC TLS verification is disabled (oidc_insecure_skip_verify); "+
			"this is only valid for internal/self-signed CAs in non-production environments",
			zap.String("provider_url", cfg.ProviderURL),
		)
	}

	return cfg, nil
}

// oidcContext sets InsecureIssuerURLContext when cfg.IssuerURL is given, so
// discovery (ProviderURL) may differ from the issuer claim in tokens. This
// supports split-horizon setups (e.g. KinD) where internal DNS != public URL.
func oidcContext(ctx context.Context, cfg oidcSettings) context.Context {
	if cfg.InsecureSkipVerify {
		zap.L().Warn("connecting to OIDC provider with TLS certificate verification disabled",
			zap.String("provider_url", cfg.ProviderURL))

		insecureClient := httpx.New(httpx.DefaultTimeout)

		transport := httpx.NewTransport()
		//nolint:gosec // operator opt-in via oidc_insecure_skip_verify setting, for internal/self-signed CAs
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
		insecureClient.Transport = transport
		ctx = gooidc.ClientContext(ctx, insecureClient)
	}

	if cfg.IssuerURL != "" {
		return gooidc.InsecureIssuerURLContext(ctx, cfg.IssuerURL)
	}

	return ctx
}

// extractURLBase returns the scheme+host portion of a URL string.
func extractURLBase(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}

	return parsed.Scheme + "://" + parsed.Host
}

// OIDCAuthorize godoc
// @Summary  Get OIDC authorization URL
// @Tags     OIDC
// @Produce  json
// @Success  200  {object}  map[string]string
// @Failure  400  {object}  map[string]string
// @Router   /api/auth/oidc/authorize [get].
func (s *State) OIDCAuthorize(writer http.ResponseWriter, request *http.Request) {
	cfg, err := s.loadOIDCSettings(request.Context())
	if err != nil {
		zap.L().Error("load OIDC settings", zap.Error(err))
		s.Error(writer, http.StatusBadRequest, "OIDC not configured")

		return
	}

	// The verifier stays in the browser; only its challenge is committed to
	// the state token that travels to the provider and back.
	pkceVerifier := beginPKCE(writer, cfg.RedirectBase)

	stateToken, err := makeOAuthState(oidcProviderKey, oauth2.S256ChallengeFromVerifier(pkceVerifier), s.oauthStateSecret())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "State token error")

		return
	}

	redirectURI := strings.TrimRight(cfg.RedirectBase, "/") + "/auth/callback"

	providerCtx := oidcContext(request.Context(), cfg)

	provider, err := gooidc.NewProvider(providerCtx, cfg.ProviderURL)
	if err != nil {
		zap.L().Error("OIDC provider unreachable", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "OIDC provider call failed")

		return
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	authURL := oauth2Cfg.AuthCodeURL(stateToken, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(pkceVerifier))

	// If oidc_browser_base_url is set, rewrite the internal base URL in authURL
	// to the browser-accessible one (split-horizon: pod uses internal DNS, browser uses external).
	if cfg.BrowserBaseURL != "" {
		internalBase := extractURLBase(cfg.ProviderURL)
		authURL = strings.Replace(authURL, internalBase, strings.TrimRight(cfg.BrowserBaseURL, "/"), 1)
	}

	s.JSON(writer, http.StatusOK, map[string]string{"url": authURL, "state": stateToken})
}

// oidcGroupsFromClaims extracts group names from the configured group claim,
// which may be either a JSON array of strings or a comma-separated string.
func oidcGroupsFromClaims(claims map[string]any, groupClaim string) []string {
	raw, ok := claims[groupClaim]
	if !ok {
		return nil
	}

	var groups []string

	switch rawValue := raw.(type) {
	case []any:
		for _, item := range rawValue {
			if group, ok := item.(string); ok {
				groups = append(groups, group)
			}
		}
	case string:
		groups = utils.SplitTrimmedCommaList(rawValue)
	}

	return groups
}

// exchangeOIDCToken performs the OIDC token exchange and ID-token
// verification for an authorization code, returning the merged claims (ID
// token plus UserInfo endpoint) and the token subject. On failure it writes
// the HTTP error response itself and returns ok=false.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (s *State) exchangeOIDCToken(
	writer http.ResponseWriter, ctx context.Context, cfg oidcSettings, code, pkceVerifier string,
) (map[string]any, string, bool) {
	providerCtx := oidcContext(ctx, cfg)

	oidcProvider, err := gooidc.NewProvider(providerCtx, cfg.ProviderURL)
	if err != nil {
		zap.L().Error("OIDC provider unreachable", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "OIDC provider call failed")

		return nil, "", false
	}

	redirectURI := strings.TrimRight(cfg.RedirectBase, "/") + "/auth/callback"
	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	token, err := oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		zap.L().Error("OIDC token exchange failed", zap.Error(err))
		s.Error(writer, http.StatusUnauthorized, "Token exchange failed")

		return nil, "", false
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		s.Error(writer, http.StatusUnauthorized, "No id_token in response")

		return nil, "", false
	}

	idTokenVerifier := oidcProvider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})

	idToken, err := idTokenVerifier.Verify(providerCtx, rawIDToken)
	if err != nil {
		zap.L().Error("OIDC ID token verification failed", zap.Error(err))
		s.Error(writer, http.StatusUnauthorized, "Token verification failed")

		return nil, "", false
	}

	var claims map[string]any

	err = idToken.Claims(&claims)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Claims extraction failed")

		return nil, "", false
	}

	enrichClaimsFromUserInfo(ctx, oidcProvider, token, claims)

	return claims, idToken.Subject, true
}

// OIDCCallback godoc
// @Summary  Complete OIDC login flow
// @Tags     OIDC
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "code and state"
// @Success  200   {object}  authResponse
// @Failure  401   {object}  map[string]string
// @Router   /api/auth/oidc/callback [post].
func (s *State) OIDCCallback(writer http.ResponseWriter, request *http.Request) {
	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}

	err := decode(request, &req)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	provider, challenge, validState := decodeOAuthState(req.State, s.oauthStateSecret())
	if !validState || provider != oidcProviderKey {
		s.Error(writer, http.StatusUnauthorized, oidcInvalidStateMsg)

		return
	}

	ctx := request.Context()

	cfg, err := s.loadOIDCSettings(ctx)
	if err != nil {
		zap.L().Error("load OIDC settings", zap.Error(err))
		s.Error(writer, http.StatusBadRequest, "OIDC not configured")

		return
	}

	pkceVerifier, hasVerifier := consumePKCEVerifier(writer, request, cfg.RedirectBase, challenge)
	if !hasVerifier {
		s.Error(writer, http.StatusUnauthorized, oidcInvalidStateMsg)

		return
	}

	claims, sub, ok := s.exchangeOIDCToken(writer, ctx, cfg, req.Code, pkceVerifier)
	if !ok {
		return
	}

	email, _ := claims[oauthFieldEmail].(string)
	if email == "" {
		s.Error(writer, http.StatusUnauthorized, "No email in OIDC token")

		return
	}

	name := oidcDisplayName(claims, email)

	var avatarURL *string
	if pic, ok := claims["picture"].(string); ok && pic != "" {
		avatarURL = &pic
	}

	bio := oidcBioFromClaims(claims)
	groups := oidcGroupsFromClaims(claims, cfg.GroupClaim)

	s.completeSSOLogin(ctx, writer, email, name, avatarURL, bio, oidcProviderKey, sub, groups, cfg.GroupAdmins)
}
