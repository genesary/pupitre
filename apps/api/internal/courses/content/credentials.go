package content

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Credential is a single git URL-to-token mapping loaded from the
// credentials file.
type Credential struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// GitCredentialStore holds the set of git credentials available for
// matching against repository URLs.
type GitCredentialStore struct {
	entries []Credential
}

// LoadCredentials reads and parses the YAML credentials file at credsPath.
func LoadCredentials(credsPath string) (*GitCredentialStore, error) {
	data, err := os.ReadFile(filepath.Clean(credsPath))
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	var doc struct {
		Credentials []Credential `yaml:"credentials"`
	}

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials file: %w", err)
	}

	return &GitCredentialStore{entries: doc.Credentials}, nil
}

// Match returns the token for the first credential whose URL pattern
// matches repoURL.
func (s *GitCredentialStore) Match(repoURL string) string {
	if s == nil {
		return ""
	}

	for _, c := range s.entries {
		if matchGitURL(c.URL, repoURL) {
			return c.Token
		}
	}

	return ""
}

// matchGitURL checks whether pattern matches repoURL.
// Pattern is a host/path glob like "github.com/myorg/*".
func matchGitURL(pattern, repoURL string) bool {
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimPrefix(repoURL, "git@")
	pattern = strings.TrimPrefix(pattern, "https://")
	pattern = strings.TrimPrefix(pattern, "http://")
	pattern = strings.TrimPrefix(pattern, "git@")

	matched, err := path.Match(pattern, repoURL)

	return err == nil && matched
}
