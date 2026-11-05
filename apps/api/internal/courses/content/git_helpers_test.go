package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// TestNewGitCache creates the cache directory and initialises the maps.
func TestNewGitCache(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "clones")

	gc := NewGitCache(dir, time.Minute)
	if gc == nil {
		t.Fatal("NewGitCache returned nil")
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("cache dir not created: %v", err)
	}

	if gc.repos == nil || gc.cloning == nil {
		t.Error("internal maps not initialised")
	}

	if gc.ttl != time.Minute {
		t.Errorf("ttl = %v, want 1m", gc.ttl)
	}
}

// TestGitCache_ClearAndClearRepo remove tracked repositories from disk and
// from the map.
func TestGitCache_ClearAndClearRepo(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	gc := NewGitCache(filepath.Join(base, "cache"), time.Minute)

	// Fake two cached repos backed by real directories.
	repoA := filepath.Join(base, "a")
	repoB := filepath.Join(base, "b")

	for _, d := range []string{repoA, repoB} {
		err := os.MkdirAll(d, 0o750)
		if err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	keyA := gc.cacheKey("https://git/a", "main")
	gc.repos[keyA] = &cachedRepo{path: repoA, clonedAt: time.Now()}
	gc.repos[gc.cacheKey("https://git/b", "main")] = &cachedRepo{path: repoB, clonedAt: time.Now()}

	gc.ClearRepo("https://git/a", "main")

	_, statErr := os.Stat(repoA)
	if !os.IsNotExist(statErr) {
		t.Error("ClearRepo did not remove repo A from disk")
	}

	if _, ok := gc.repos[keyA]; ok {
		t.Error("ClearRepo did not drop repo A from the map")
	}

	gc.Clear()

	_, statErr = os.Stat(repoB)
	if !os.IsNotExist(statErr) {
		t.Error("Clear did not remove repo B from disk")
	}

	if len(gc.repos) != 0 {
		t.Errorf("Clear left %d entries in the map", len(gc.repos))
	}
}

// TestGitCache_CacheKeyStable is deterministic and branch-sensitive.
func TestGitCache_CacheKeyStable(t *testing.T) {
	t.Parallel()

	gc := NewGitCache(t.TempDir(), time.Minute)

	k1 := gc.cacheKey("https://git/x", "main")
	k2 := gc.cacheKey("https://git/x", "main")
	k3 := gc.cacheKey("https://git/x", "dev")

	if k1 != k2 {
		t.Errorf("cacheKey not stable: %q vs %q", k1, k2)
	}

	if k1 == k3 {
		t.Error("cacheKey should differ by branch")
	}
}

// TestFreshCachedPath is true only for a non-empty path inside the TTL.
func TestFreshCachedPath(t *testing.T) {
	t.Parallel()

	fresh := &cachedRepo{path: "/tmp/x", clonedAt: time.Now()}
	if p, ok := freshCachedPath(fresh, time.Minute); !ok || p != "/tmp/x" {
		t.Errorf("fresh repo = %q, %v", p, ok)
	}

	stale := &cachedRepo{path: "/tmp/x", clonedAt: time.Now().Add(-2 * time.Minute)}
	if _, ok := freshCachedPath(stale, time.Minute); ok {
		t.Error("stale repo should not be fresh")
	}

	empty := &cachedRepo{clonedAt: time.Now()}
	if _, ok := freshCachedPath(empty, time.Minute); ok {
		t.Error("repo with empty path should not be fresh")
	}
}

// TestRecentCloneFailure returns an error only inside the backoff window.
func TestRecentCloneFailure(t *testing.T) {
	t.Parallel()

	zeroErr := recentCloneFailure(&cachedRepo{})
	if zeroErr != nil {
		t.Errorf("zero failedUntil should not error: %v", zeroErr)
	}

	activeErr := recentCloneFailure(&cachedRepo{failedUntil: time.Now().Add(10 * time.Second)})
	if activeErr == nil {
		t.Error("failure within backoff should return an error")
	}

	expiredErr := recentCloneFailure(&cachedRepo{failedUntil: time.Now().Add(-time.Second)})
	if expiredErr != nil {
		t.Errorf("expired backoff should not error: %v", expiredErr)
	}
}

// TestBuildAuth returns basic auth only for token-bearing HTTP(S) URLs.
func TestBuildAuth(t *testing.T) {
	t.Parallel()

	if got := buildAuth("https://git/x", ""); got != nil {
		t.Error("no token should yield nil auth")
	}

	if got := buildAuth("git@github.com:x/y.git", "tok"); got != nil {
		t.Error("SSH URL should yield nil auth")
	}

	got := buildAuth("https://git/x", "tok")

	basic, ok := got.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected *githttp.BasicAuth, got %T", got)
	}

	if basic.Username != gitOAuth2Username || basic.Password != "tok" {
		t.Errorf("unexpected credential: %+v", basic)
	}
}

// TestSanitizeGitOutput trims, redacts the token and handles empty output.
func TestSanitizeGitOutput(t *testing.T) {
	t.Parallel()

	if got := sanitizeGitOutput("   \n\t ", ""); got != gitNoCloneOutput {
		t.Errorf("blank output = %q, want %q", got, gitNoCloneOutput)
	}

	if got := sanitizeGitOutput("  hello  ", ""); got != "hello" {
		t.Errorf("trim = %q", got)
	}

	got := sanitizeGitOutput("failed for https://oauth2:s3cr3t@git/x", "s3cr3t")
	if strings.Contains(got, "s3cr3t") {
		t.Errorf("token not redacted: %q", got)
	}

	if !strings.Contains(got, "***") {
		t.Errorf("expected redaction marker: %q", got)
	}
}
