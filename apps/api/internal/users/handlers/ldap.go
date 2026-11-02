package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	ldap "github.com/go-ldap/ldap/v3"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// ldapTimeout bounds how long an LDAP login attempt may take end-to-end.
const ldapTimeout = 5 * time.Minute

// ldapSettings holds the LDAP configuration read from the settings table.
type ldapSettings struct {
	Enabled      bool
	ServerURL    string
	BindDN       string
	BindPassword string
	UserBaseDN   string
	UserFilter   string
	GroupBaseDN  string
	GroupFilter  string
}

// loadLDAPSettings reads the LDAP configuration and validates it is usable.
func (s *State) loadLDAPSettings(ctx context.Context) (ldapSettings, error) {
	cfg := ldapSettings{
		Enabled:      repository.ReadSetting(ctx, s.Repos.Settings, "ldap_enabled", "false") == authSettingTrue,
		ServerURL:    repository.ReadSetting(ctx, s.Repos.Settings, "ldap_server_url", ""),
		BindDN:       repository.ReadSetting(ctx, s.Repos.Settings, "ldap_bind_dn", ""),
		BindPassword: repository.ReadSetting(ctx, s.Repos.Settings, "ldap_bind_password", ""),
		UserBaseDN:   repository.ReadSetting(ctx, s.Repos.Settings, "ldap_user_base_dn", ""),
		UserFilter:   repository.ReadSetting(ctx, s.Repos.Settings, "ldap_user_filter", "(mail=%s)"),
		GroupBaseDN:  repository.ReadSetting(ctx, s.Repos.Settings, "ldap_group_base_dn", ""),
		GroupFilter:  repository.ReadSetting(ctx, s.Repos.Settings, "ldap_group_filter", "(|(member=%s)(uniqueMember=%s)(memberUid=%s))"),
	}
	if !cfg.Enabled {
		return cfg, errors.New("LDAP authentication is not enabled")
	}

	if cfg.ServerURL == "" || cfg.UserBaseDN == "" {
		return cfg, errors.New("LDAP not fully configured (ldap_server_url and ldap_user_base_dn required)")
	}

	return cfg, nil
}

// LDAPLogin godoc
// @Summary  Authenticate via LDAP and receive a JWT
// @Tags     LDAP
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "email and password"
// @Success  200   {object}  authResponse
// @Failure  401   {object}  map[string]string
// @Router   /api/auth/ldap/login [post].
func (s *State) LDAPLogin(writer http.ResponseWriter, httpReq *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := decode(httpReq, &req)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if req.Email == "" || req.Password == "" {
		s.Error(writer, http.StatusBadRequest, "email and password are required")

		return
	}

	ctx, cancel := context.WithTimeout(httpReq.Context(), ldapTimeout)
	defer cancel()

	cfg, err := s.loadLDAPSettings(ctx)
	if err != nil {
		zap.L().Error("load LDAP settings", zap.Error(err))
		s.Error(writer, http.StatusBadRequest, "LDAP not configured")

		return
	}

	conn, connected := s.dialLDAPServer(writer, cfg)
	if !connected {
		return
	}
	defer func() { _ = conn.Close() }()

	entry, authenticated := s.bindLDAPUser(writer, conn, cfg, req.Email, req.Password)
	if !authenticated {
		return
	}

	name := ldapDisplayName(entry, req.Email)
	groups := ldapUserGroups(conn, cfg, entry.DN)

	s.completeSSOLogin(ctx, writer, req.Email, name, nil, nil, "ldap", entry.DN, groups, nil)
}

// dialLDAPServer connects to the LDAP server and binds the service account
// when configured, writing an error response and returning ok=false on failure.
func (s *State) dialLDAPServer(writer http.ResponseWriter, cfg ldapSettings) (*ldap.Conn, bool) {
	conn, err := ldap.DialURL(cfg.ServerURL)
	if err != nil {
		zap.L().Error("LDAP dial failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "LDAP server call failed")

		return nil, false
	}

	if cfg.BindDN != "" {
		err = conn.Bind(cfg.BindDN, cfg.BindPassword)
		if err != nil {
			_ = conn.Close()

			zap.L().Error("LDAP service bind failed", zap.Error(err))
			s.Error(writer, http.StatusInternalServerError, "LDAP server call failed")

			return nil, false
		}
	}

	return conn, true
}

// bindLDAPUser searches for the user by email and binds as them to verify
// their password, writing an error response and returning ok=false on failure.
func (s *State) bindLDAPUser(
	writer http.ResponseWriter, conn *ldap.Conn, cfg ldapSettings, email, password string,
) (*ldap.Entry, bool) {
	filter := fmt.Sprintf(cfg.UserFilter, ldap.EscapeFilter(email))
	searchReq := ldap.NewSearchRequest(
		cfg.UserBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"dn", "cn", "mail", "displayName", "givenName", "sn"},
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil || len(result.Entries) == 0 {
		s.Error(writer, http.StatusUnauthorized, "Invalid email or password")

		return nil, false
	}

	entry := result.Entries[0]

	err = conn.Bind(entry.DN, password)
	if err != nil {
		s.Error(writer, http.StatusUnauthorized, "Invalid email or password")

		return nil, false
	}

	if cfg.BindDN != "" {
		_ = conn.Bind(cfg.BindDN, cfg.BindPassword)
	}

	return entry, true
}

// ldapDisplayName derives a human display name from an LDAP entry, falling
// back to fallback (typically the email used to look it up) when unset.
func ldapDisplayName(entry *ldap.Entry, fallback string) string {
	name := entry.GetAttributeValue("displayName")
	if name == "" {
		name = entry.GetAttributeValue("cn")
	}

	if name == "" {
		first := entry.GetAttributeValue("givenName")
		last := entry.GetAttributeValue("sn")
		name = strings.TrimSpace(first + " " + last)
	}

	if name == "" {
		name = fallback
	}

	return name
}

// ldapUserGroups looks up the group names the user at userDN belongs to,
// returning nil when group lookup is not configured or the search fails.
func ldapUserGroups(conn *ldap.Conn, cfg ldapSettings, userDN string) []string {
	if cfg.GroupBaseDN == "" {
		return nil
	}

	escapedDN := ldap.EscapeFilter(userDN)
	groupFilter := fmt.Sprintf(cfg.GroupFilter, escapedDN, escapedDN, escapedDN)

	groupReq := ldap.NewSearchRequest(
		cfg.GroupBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, 0, false,
		groupFilter,
		[]string{"cn"},
		nil,
	)

	result, err := conn.Search(groupReq)
	if err != nil {
		return nil
	}

	var groups []string

	for _, g := range result.Entries {
		if cn := g.GetAttributeValue("cn"); cn != "" {
			groups = append(groups, cn)
		}
	}

	return groups
}
