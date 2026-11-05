package handlers

import (
	"testing"

	"github.com/go-ldap/ldap/v3"
)

// TestLdapDisplayName walks the displayName → cn → givenName+sn → fallback
// precedence chain.
func TestLdapDisplayName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		attrs map[string][]string
		want  string
	}{
		{"displayName wins", map[string][]string{
			"displayName": {"Ada Lovelace"}, "cn": {"alovelace"}, "givenName": {"Ada"}, "sn": {"Lovelace"},
		}, "Ada Lovelace"},
		{"cn when no displayName", map[string][]string{
			"cn": {"alovelace"}, "givenName": {"Ada"}, "sn": {"Lovelace"},
		}, "alovelace"},
		{"given + sn when no cn", map[string][]string{
			"givenName": {"Ada"}, "sn": {"Lovelace"},
		}, "Ada Lovelace"},
		{"only sn", map[string][]string{"sn": {"Lovelace"}}, "Lovelace"},
		{"fallback when nothing", map[string][]string{}, "ada@example.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entry := ldap.NewEntry("uid=ada,dc=example,dc=com", tc.attrs)
			if got := ldapDisplayName(entry, "ada@example.com"); got != tc.want {
				t.Errorf("ldapDisplayName = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLdapUserGroups_NoGroupBaseDN returns nil without touching the
// connection when group lookup is not configured.
func TestLdapUserGroups_NoGroupBaseDN(t *testing.T) {
	t.Parallel()

	got := ldapUserGroups(nil, ldapSettings{GroupBaseDN: ""}, "uid=ada,dc=example,dc=com")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
