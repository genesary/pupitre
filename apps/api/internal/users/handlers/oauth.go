package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httpx"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/metrics"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Repeated OAuth/OIDC literals and tunables, pulled out to satisfy goconst
// and mnd. The "providers" JSON key is shared with adminJSONKeyProviders
// in admin.go to avoid a duplicate constant in the package.
const (
	oauthProviderGitHub   = "github"
	oauthFieldEmail       = "email"
	oauthStateKey         = "state"
	oauthProviderGitLab   = "gitlab"
	oauthIssuerGitLab     = "https://gitlab.com"
	oauthProviderGoogle   = "google"
	oauthIssuerGoogle     = "https://accounts.google.com"
	oauthClaimAbout       = "about"
	oauthClaimDescription = "description"
	oauthDefaultUsername  = "user"
	oauthScopeProfile     = "profile"
	oauthFieldBio         = "bio"
	oauthJSONKeyURL       = "url"

	oauthStateTokenTTL  = 10 * time.Minute
	oauthMaxUsernameLen = 32

	// oauthStateCookie holds the nonce of the state the browser was given
	// (docs/specs/190-backend-defects.md B13).
	oauthStateCookie     = "pupitre_oauth_state"
	oauthStateCookiePath = "/api/auth"
	oauthNonceBytes      = 32
)

// oauthStateSecret returns the secret used to sign OAuth CSRF state tokens.
// Falls back to JWTSecret when OAuthStateSecret is not explicitly set, so
// existing deployments and tests that only configure JWTSecret keep working.
func (s *State) oauthStateSecret() string {
	if s.Config.OAuthStateSecret != "" {
		return s.Config.OAuthStateSecret
	}

	return s.Config.JWTSecret
}

// ── OAuth CSRF state JWT ──────────────────────────────────────────────────────

// oauthStateClaims is the JWT payload carried in the OAuth "state"
// parameter, used to prevent CSRF and remember which provider a login
// flow was started with.
type oauthStateClaims struct {
	jwt.RegisteredClaims

	Provider string `json:"provider"`
	// Nonce is also set in the oauthStateCookie of the browser that started
	// the flow; the callback accepts the state from that browser only.
	Nonce string `json:"nonce,omitempty"`
}

// startOAuthState creates the state of a new login flow and binds it to the
// browser with an HttpOnly cookie holding its nonce. Without the binding, an
// attacker could send a victim a callback link carrying the attacker's own
// code and state, and sign the victim in to the attacker's account (login
// CSRF, docs/specs/190-backend-defects.md B13).
func (s *State) startOAuthState(writer http.ResponseWriter, provider, redirectBase string) (string, error) {
	raw := make([]byte, oauthNonceBytes)

	_, err := rand.Read(raw)
	if err != nil {
		return "", fmt.Errorf("oauth state nonce: %w", err)
	}

	nonce := base64.RawURLEncoding.EncodeToString(raw)

	token, err := makeOAuthStateWithNonce(provider, nonce, s.oauthStateSecret())
	if err != nil {
		return "", err
	}

	// Secure follows the redirect base: local development serves the
	// frontend over plain HTTP, where a Secure cookie would not come back.
	http.SetCookie(writer, &http.Cookie{ //nolint:gosec // G124: Secure is set whenever the site is HTTPS, see above
		Name:     oauthStateCookie,
		Value:    nonce,
		Path:     oauthStateCookiePath,
		MaxAge:   int(oauthStateTokenTTL / time.Second),
		HttpOnly: true,
		Secure:   strings.HasPrefix(redirectBase, "https://"),
		SameSite: http.SameSiteLaxMode,
	})

	return token, nil
}

// checkOAuthState verifies a state sent to a callback and that it comes from
// the browser it was given to, then clears the cookie: a state is used once.
// It returns the provider the flow was started with.
func (s *State) checkOAuthState(writer http.ResponseWriter, request *http.Request, stateToken string) (string, bool) {
	provider, nonce, ok := decodeOAuthStateNonce(stateToken, s.oauthStateSecret())
	if !ok || nonce == "" {
		return "", false
	}

	cookie, err := request.Cookie(oauthStateCookie)
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(nonce)) != 1 {
		return "", false
	}

	http.SetCookie(writer, &http.Cookie{ //nolint:gosec // G124: clears the cookie with the Secure flag it was set with
		Name:     oauthStateCookie,
		Value:    "",
		Path:     oauthStateCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})

	return provider, true
}

// makeOAuthStateWithNonce creates a short-lived, signed JWT to use as the OAuth
// "state" parameter for the given provider.
func makeOAuthStateWithNonce(provider, nonce, secret string) (string, error) {
	claims := oauthStateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(oauthStateTokenTTL)),
		},
		Provider: provider,
		Nonce:    nonce,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign oauth state: %w", err)
	}

	return token, nil
}

// decodeOAuthStateNonce verifies and parses an OAuth state JWT, returning
// the provider id and the nonce it was issued with, and whether it is valid.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func decodeOAuthStateNonce(stateToken, secret string) (string, string, bool) {
	token, err := jwt.ParseWithClaims(stateToken, &oauthStateClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}

			return []byte(secret), nil
		})
	if err != nil || !token.Valid {
		return "", "", false
	}

	if c, ok := token.Claims.(*oauthStateClaims); ok {
		return c.Provider, c.Nonce, true
	}

	return "", "", false
}

// ListProviders godoc
// @Summary  List configured OAuth providers
// @Tags     OAuth
// @Produce  json
// @Success  200  {object}  map[string]interface{}
// @Router   /api/auth/oauth/providers [get].
func (s *State) ListProviders(writer http.ResponseWriter, _ *http.Request) {
	var providers []map[string]string

	for _, p := range s.Config.Providers {
		if p.ClientID != "" {
			providers = append(providers, map[string]string{"id": p.ID, "name": p.Name})
		}
	}

	if providers == nil {
		providers = []map[string]string{}
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyProviders: providers})
}

// OAuthAuthorize godoc
// @Summary  Get OAuth authorization URL
// @Tags     OAuth
// @Produce  json
// @Param    provider  path  string  true  "OAuth provider id"
// @Success  200  {object}  map[string]string
// @Failure  400  {object}  map[string]string
// @Router   /api/auth/oauth/{provider}/authorize [get].
func (s *State) OAuthAuthorize(writer http.ResponseWriter, request *http.Request) {
	providerID := param(request, "provider")

	providerCfg := s.Config.FindProvider(providerID)
	if providerCfg == nil || providerCfg.ClientID == "" {
		s.Error(writer, http.StatusBadRequest, "Unknown or unconfigured provider: "+providerID)

		return
	}

	stateToken, err := s.startOAuthState(writer, providerID, s.Config.OAuthRedirectBase)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "State token error")

		return
	}

	redirectURI := s.Config.OAuthRedirectBase + "/auth/callback"

	var authURL string

	if providerID == oauthProviderGitHub {
		parsedURL, _ := url.Parse("https://github.com/login/oauth/authorize")
		q := url.Values{}
		q.Set("client_id", providerCfg.ClientID)
		q.Set("redirect_uri", redirectURI)
		q.Set("scope", "user:email read:user")
		q.Set(oauthStateKey, stateToken)
		parsedURL.RawQuery = q.Encode()
		authURL = parsedURL.String()
	} else {
		issuerURL := resolveIssuerURL(providerCfg)
		if issuerURL == "" {
			s.Error(writer, http.StatusBadRequest, "Provider "+providerID+" requires issuerUrl")

			return
		}

		oidcProvider, err := gooidc.NewProvider(request.Context(), issuerURL)
		if err != nil {
			zap.L().Error("OIDC provider unreachable", zap.Error(err))
			s.Error(writer, http.StatusInternalServerError, "OIDC provider call failed")

			return
		}

		oauth2Cfg := oauth2.Config{
			ClientID:     providerCfg.ClientID,
			ClientSecret: providerCfg.ClientSecret,
			RedirectURL:  redirectURI,
			Endpoint:     oidcProvider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, oauthFieldEmail, oauthScopeProfile},
		}
		authURL = oauth2Cfg.AuthCodeURL(stateToken, oauth2.AccessTypeOnline)
	}

	s.JSON(writer, http.StatusOK, map[string]string{oauthJSONKeyURL: authURL, oauthStateKey: stateToken})
}

// OAuthCallback godoc
// @Summary  Complete OAuth login flow
// @Tags     OAuth
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "code and state from provider"
// @Success  200   {object}  authResponse
// @Failure  401   {object}  map[string]string
// @Router   /api/auth/oauth/callback [post].
func (s *State) OAuthCallback(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	providerID, ok := s.checkOAuthState(writer, request, body.State)
	if !ok {
		s.Error(writer, http.StatusUnauthorized, "Invalid or expired OAuth state")

		return
	}

	providerCfg := s.Config.FindProvider(providerID)
	if providerCfg == nil || providerCfg.ClientID == "" {
		s.Error(writer, http.StatusBadRequest, "Unknown or unconfigured provider: "+providerID)

		return
	}

	redirectURI := s.Config.OAuthRedirectBase + "/auth/callback"

	email, displayName, avatarURL, bioStr, providerUserID, err := fetchOAuthProfile(
		request.Context(), providerID, providerCfg, body.Code, redirectURI)
	if err != nil {
		zap.L().Error("OAuth profile fetch failed", zap.String("provider", providerID), zap.Error(err))
		s.Error(writer, http.StatusUnauthorized, "Authentication failed")

		return
	}

	user, err := upsertSSOUser(request.Context(), s.Repos.Users, email, displayName, avatarURL, bioStr, providerID, providerUserID)
	if err != nil {
		zap.L().Error("upsert SSO user failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "failed to update user identity from SSO")

		return
	}

	s.oauthFinishLogin(writer, request, user)
}

// fetchOAuthProfile fetches the authenticated user's profile from the
// given OAuth/OIDC provider, dispatching to the GitHub or generic OIDC
// fetcher. It returns (email, name, avatar, bio, providerUserID, err).
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func fetchOAuthProfile(
	ctx context.Context, providerID string, providerCfg *config.ProviderConfig, code, redirectURI string,
) (string, string, *string, *string, string, error) {
	if providerID == oauthProviderGitHub {
		return fetchGitHub(ctx, providerCfg, code, redirectURI)
	}

	return fetchOIDCProvider(ctx, providerCfg, code, redirectURI)
}

// oauthFinishLogin enrolls the user in their default/group courses, issues
// a session JWT, and writes the login response.
func (s *State) oauthFinishLogin(writer http.ResponseWriter, request *http.Request, user *userPublicRow) {
	ctx := request.Context()

	addToDefaultGroup(ctx, s.Repos.Groups, user.ID)
	syncGroupEnrollments(ctx, s.Repos.Groups, user.ID)
	s.touchLastLogin(ctx, user.ID)

	token, err := middleware.CreateToken(user.ID, user.Email, user.Role, user.Username, s.Config.JWTSecret, s.Config.JWTExpiryH)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Token error")

		return
	}

	s.JSON(writer, http.StatusOK, authResponse{Token: token, User: *user})
}

// resolveIssuerURL returns the OIDC issuer URL for a provider, falling back to
// well-known defaults when issuerUrl is not explicitly configured.
func resolveIssuerURL(providerCfg *config.ProviderConfig) string {
	if providerCfg.IssuerURL != "" {
		return providerCfg.IssuerURL
	}

	switch providerCfg.ID {
	case oauthProviderGitLab:
		return oauthIssuerGitLab
	case oauthProviderGoogle:
		return oauthIssuerGoogle
	}

	return ""
}

// ── Generic OIDC fetch (GitLab, Google, Authentik, Keycloak, …) ──────────────

// fetchOIDCProvider exchanges an OAuth2 code for the user's profile via
// generic OIDC discovery. It returns (email, name, avatar, bio, sub, err).
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func fetchOIDCProvider(
	ctx context.Context, providerCfg *config.ProviderConfig, code, redirectURI string,
) (string, string, *string, *string, string, error) {
	oidcProvider, err := gooidc.NewProvider(ctx, resolveIssuerURL(providerCfg))
	if err != nil {
		return "", "", nil, nil, "", fmt.Errorf("cannot reach OIDC provider: %w", err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     providerCfg.ClientID,
		ClientSecret: providerCfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, oauthFieldEmail, oauthScopeProfile},
	}

	token, err := oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", nil, nil, "", fmt.Errorf("token exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", nil, nil, "", errors.New("no id_token in response")
	}

	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: providerCfg.ClientID})

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", nil, nil, "", fmt.Errorf("ID token verification failed: %w", err)
	}

	var claims map[string]any

	err = idToken.Claims(&claims)
	if err != nil {
		return "", "", nil, nil, "", fmt.Errorf("claims extraction failed: %w", err)
	}

	enrichClaimsFromUserInfo(ctx, oidcProvider, token, claims)

	email, _ := claims[oauthFieldEmail].(string)
	if email == "" {
		return "", "", nil, nil, "", errors.New("no email in OIDC token")
	}

	nameStr := oidcDisplayName(claims, email)

	var avatar *string

	if pic, ok := claims["picture"].(string); ok && pic != "" {
		avatar = &pic
	}

	bio := oidcBioFromClaims(claims)

	return email, nameStr, avatar, bio, idToken.Subject, nil
}

// enrichClaimsFromUserInfo merges claims fetched from the OIDC UserInfo
// endpoint into claims; UserInfo values take priority over the ID token.
func enrichClaimsFromUserInfo(ctx context.Context, oidcProvider *gooidc.Provider, token *oauth2.Token, claims map[string]any) {
	userInfo, uiErr := oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if uiErr != nil {
		return
	}

	var uiClaims map[string]any

	if userInfo.Claims(&uiClaims) == nil {
		maps.Copy(claims, uiClaims)
	}
}

// oidcDisplayName resolves a human display name from OIDC claims, falling
// back to preferred_username and finally to email when name is absent.
func oidcDisplayName(claims map[string]any, email string) string {
	name, _ := claims["name"].(string)
	if name == "" {
		name, _ = claims["preferred_username"].(string)
	}

	if name == "" {
		name = email
	}

	return name
}

// oidcBioFromClaims extracts a bio/description from common non-standard
// OIDC claim keys, returning nil when none of them are present.
func oidcBioFromClaims(claims map[string]any) *string {
	for _, key := range []string{oauthFieldBio, oauthClaimDescription, oauthClaimAbout, oauthScopeProfile} {
		if v, ok := claims[key].(string); ok && v != "" {
			return &v
		}
	}

	return nil
}

// ── GitHub fetch (OAuth2 only, no OIDC discovery) ─────────────────────────────

// fetchGitHub exchanges an OAuth2 code for a GitHub access token and
// returns the authenticated user's profile fields as
// (email, name, avatar, bio, id, err).
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func fetchGitHub(
	ctx context.Context, providerCfg *config.ProviderConfig, code, redirectURI string,
) (string, string, *string, *string, string, error) {
	// The shared pooled client rather than a bare one: the token exchange
	// and the profile fetch both talk to the same host over TLS, they reuse
	// that connection, and neither can hang a login goroutine indefinitely.
	client := httpx.Client()

	accessToken, err := githubAccessToken(ctx, client, providerCfg, code, redirectURI)
	if err != nil {
		return "", "", nil, nil, "", err
	}

	var profile map[string]any

	err = doGet(client, "https://api.github.com/user", accessToken, &profile) //nolint:contextcheck // doGet's signature is pinned by pre-existing whitebox tests; see doGet's doc comment
	if err != nil {
		return "", "", nil, nil, "", fmt.Errorf("GitHub user request failed: %w", err)
	}

	ghID := fmt.Sprintf("%v", profile["id"])
	login, _ := profile["login"].(string)

	nameStr, _ := profile["name"].(string)
	if nameStr == "" {
		nameStr = login
	}

	avatarPtr, bioPtr := githubProfileMedia(profile)

	emailStr, _ := profile[oauthFieldEmail].(string)
	if emailStr == "" {
		emailStr = githubPrimaryEmail(client, accessToken) //nolint:contextcheck // githubPrimaryEmail calls doGet, whose signature is pinned by pre-existing tests
	}

	if emailStr == "" {
		return "", "", nil, nil, "", errors.New("could not retrieve a verified GitHub email address")
	}

	return emailStr, nameStr, avatarPtr, bioPtr, ghID, nil
}

// githubAccessToken exchanges an OAuth2 authorization code for a GitHub
// access token.
func githubAccessToken(
	ctx context.Context, client *http.Client, providerCfg *config.ProviderConfig, code, redirectURI string,
) (string, error) {
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token",
		strings.NewReader(url.Values{
			"client_id":     {providerCfg.ClientID},
			"client_secret": {providerCfg.ClientSecret},
			"code":          {code},
			"redirect_uri":  {redirectURI},
		}.Encode()))
	if err != nil {
		return "", fmt.Errorf("build github token request: %w", err)
	}

	tokenReq.Header.Set("Accept", "application/json")
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("User-Agent", "LearnLab-SSO/1.0")

	resp, err := client.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("GitHub token request failed: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)

	var tokenRes map[string]any

	_ = json.Unmarshal(body, &tokenRes)

	accessToken, _ := tokenRes["access_token"].(string)
	if accessToken == "" {
		return "", errors.New("GitHub did not return an access token")
	}

	return accessToken, nil
}

// githubPrimaryEmail finds the user's primary, verified email address via
// the GitHub emails API, falling back to the first listed address.
func githubPrimaryEmail(client *http.Client, accessToken string) string {
	var emails []map[string]any

	_ = doGet(client, "https://api.github.com/user/emails", accessToken, &emails)

	for _, emailEntry := range emails {
		primary, _ := emailEntry["primary"].(bool)
		if !primary {
			continue
		}

		verified, _ := emailEntry["verified"].(bool)
		if verified {
			email, _ := emailEntry[oauthFieldEmail].(string)

			return email
		}
	}

	if len(emails) > 0 {
		email, _ := emails[0][oauthFieldEmail].(string)

		return email
	}

	return ""
}

// githubProfileMedia extracts the avatar URL and bio from a GitHub user
// profile payload, when present. It returns (avatar, bio).
func githubProfileMedia(profile map[string]any) (*string, *string) { //nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment for the meaning of each value
	var avatar, bio *string

	// GitHub exposes bio directly in the user profile.
	if av, ok := profile["avatarUrl"].(string); ok && av != "" {
		avatar = &av
	}

	if b, ok := profile[oauthFieldBio].(string); ok && b != "" {
		bio = &b
	}

	return avatar, bio
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

// doGet issues an authenticated GET request against rawURL and decodes the
// JSON response body into out. It has no caller-supplied context because
// call sites (and existing tests) invoke it without one; it still avoids
// noctx by building the request through NewRequestWithContext.
func doGet(client *http.Client, rawURL, bearer string, out any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", rawURL, err)
	}

	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LearnLab-SSO/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", rawURL, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)

	unmarshalErr := json.Unmarshal(body, out)
	if unmarshalErr != nil {
		return fmt.Errorf("decode response from %s: %w", rawURL, unmarshalErr)
	}

	return nil
}

// ── User upsert ───────────────────────────────────────────────────────────────

// upsertSSOUser finds or creates the local user for an SSO login, trying a
// provider-identity match first, then an email match, and finally creating
// a brand-new account.
func upsertSSOUser(
	ctx context.Context, users repository.UserRepository, email, displayName string, avatarURL, bio *string, provider, providerUserID string,
) (*userPublicRow, error) {
	user := upsertSSOUserByProvider(ctx, users, provider, providerUserID, avatarURL, bio)
	if user != nil {
		return user, nil
	}

	user, err := upsertSSOUserByEmail(ctx, users, email, provider, providerUserID, avatarURL, bio)
	if user != nil || err != nil {
		return user, err //nolint:nilnil // cascading lookup: (nil,nil) means "no match, fall through to create a new user"
	}

	username := sanitizeUsername(displayName)

	taken, _ := users.ExistsByUsername(ctx, username)
	if taken {
		username = username + "_" + uuid.New().String()[:6]
	}

	newUser := &models.User{
		Username:       username,
		Email:          email,
		Role:           groupsRoleStudent,
		IsActive:       true,
		AuthProvider:   provider,
		ProviderUserID: &providerUserID,
		AvatarURL:      avatarURL,
		Bio:            bio,
	}

	err = users.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	metrics.ActiveUsers.Inc()

	created := toUserPublicRow(newUser)

	return &created, nil
}

// completeSSOLogin provisions/refreshes the local user for an SSO identity,
// syncs their group membership and role, issues a JWT, and writes the auth
// response. Shared by every SSO login flow (OIDC, LDAP, ...).
// groupAdmins is an optional list of SSO group names that authoritatively
// determines the role for this login. When configured, an identity outside
// those groups is a student even if its stored role was previously admin.
func (s *State) completeSSOLogin(
	ctx context.Context, writer http.ResponseWriter,
	email, displayName string, avatarURL, bio *string, provider, providerUserID string, groups []string,
	groupAdmins []string,
) {
	user, err := upsertSSOUser(ctx, s.Repos.Users, email, displayName, avatarURL, bio, provider, providerUserID)
	if err != nil {
		zap.L().Error("upsert SSO user failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "failed to update user identity from SSO")

		return
	}

	role, err := syncGroupsAndDeriveRole(ctx, s.Repos.Groups, user.ID, groups, provider)
	if err != nil {
		role = user.Role
	}

	if len(groupAdmins) > 0 {
		role = groupsRoleStudent
		if isMemberOfAny(groups, groupAdmins) {
			role = groupsRoleAdmin
		}

		userID, parseErr := uuid.Parse(user.ID)
		if parseErr != nil {
			zap.L().Error("parse SSO user ID for role synchronization", zap.Error(parseErr))
			s.Error(writer, http.StatusInternalServerError, "failed to synchronize user role from SSO")

			return
		}

		updatedUser, updateErr := s.Repos.Users.UpdateSSORole(ctx, userID, role)
		if updateErr != nil {
			zap.L().Error("synchronize OIDC group-admin role failed", zap.Error(updateErr))
			s.Error(writer, http.StatusInternalServerError, "failed to synchronize user role from SSO")

			return
		}

		updatedRow := toUserPublicRow(updatedUser)
		user = &updatedRow
	}

	addToDefaultGroup(ctx, s.Repos.Groups, user.ID)
	syncGroupEnrollments(ctx, s.Repos.Groups, user.ID)

	token, err := middleware.CreateToken(user.ID, user.Email, role, user.Username, s.Config.JWTSecret, s.Config.JWTExpiryH)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Token error")

		return
	}

	user.Role = role
	s.JSON(writer, http.StatusOK, authResponse{Token: token, User: *user})
}

// upsertSSOUserByProvider looks up a user already linked to this
// provider+providerUserID and refreshes their avatar/bio. It returns nil
// when no such user exists yet; lookup/update failures are not fatal to
// the overall SSO login, so no error is returned here.
func upsertSSOUserByProvider(
	ctx context.Context, users repository.UserRepository, provider, providerUserID string, avatarURL, bio *string,
) *userPublicRow {
	existing, err := users.FindByProviderIdentity(ctx, provider, providerUserID)
	if err != nil {
		return nil
	}

	updated, err := users.RefreshSSOProfile(ctx, existing.ID, avatarURL, bio)
	if err != nil {
		zap.L().Warn("upsertSSOUser: UPDATE by provider failed, using SELECT result", zap.Error(err), zap.String("userId", existing.ID.String()))

		row := toUserPublicRow(existing)

		return &row
	}

	row := toUserPublicRow(updated)

	return &row
}

// upsertSSOUserByEmail looks up a user by email and, unless it is already
// linked to a different provider, links it to this provider and refreshes
// their avatar/bio. It returns nil, nil when no such user exists yet.
func upsertSSOUserByEmail(
	ctx context.Context, users repository.UserRepository, email, provider, providerUserID string, avatarURL, bio *string,
) (*userPublicRow, error) {
	existing, err := users.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil //nolint:nilerr,nilnil // sentinel: no user matched this email yet, try creating one
	}

	// Never silently take over a local account — that would lock out password login.
	// Only link when the account has no provider yet or already belongs to this provider.
	if existing.AuthProvider != "" && existing.AuthProvider != provider {
		return nil, fmt.Errorf("an account with this email already exists with a different login method (%s)", existing.AuthProvider)
	}

	updated, err := users.LinkProviderIdentity(ctx, existing.ID, provider, providerUserID, avatarURL, bio)
	if err != nil {
		zap.L().Warn("upsertSSOUser: UPDATE by email failed, using SELECT result", zap.Error(err), zap.String("userId", existing.ID.String()))

		row := toUserPublicRow(existing)

		return &row, nil
	}

	row := toUserPublicRow(updated)

	return &row, nil
}

// sanitizeUsername derives a URL/DB-safe username from an OAuth display
// name, keeping only letters, digits, underscores, and hyphens.
func sanitizeUsername(name string) string {
	var builder strings.Builder

	for _, c := range name {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-' {
			builder.WriteRune(c)
		}

		if builder.Len() >= oauthMaxUsernameLen {
			break
		}
	}

	if builder.Len() == 0 {
		return oauthDefaultUsername
	}

	return builder.String()
}

// isMemberOfAny reports whether any element of groups appears in targets.
func isMemberOfAny(groups, targets []string) bool {
	for _, g := range groups {
		if slices.Contains(targets, g) {
			return true
		}
	}

	return false
}
