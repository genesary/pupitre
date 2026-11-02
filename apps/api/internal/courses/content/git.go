package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// gitCacheFailureBackoff is how long a failed clone blocks retries for
// the same repository key.
const gitCacheFailureBackoff = 30 * time.Second

// gitCacheDirPerm is the permission used when creating the cache
// directory.
const gitCacheDirPerm = 0o750

// gitNoCloneOutput is returned by sanitizeGitOutput when git produced no
// output at all.
const gitNoCloneOutput = "(no output)"

// gitOAuth2Username is the conventional basic-auth username paired with an
// OAuth2 token when authenticating over HTTPS (GitHub/GitLab convention).
const gitOAuth2Username = "oauth2"

// cachedRepo tracks the on-disk location and health of a single cloned
// repository.
type cachedRepo struct {
	path        string
	clonedAt    time.Time
	failedUntil time.Time
}

// GitCache clones and caches git repositories on disk, deduplicating
// concurrent clones of the same repository/branch.
type GitCache struct {
	mu             sync.Mutex
	cacheDir       string
	ttl            time.Duration
	failureBackoff time.Duration
	repos          map[string]*cachedRepo
	cloning        map[string]chan struct{}
}

// NewGitCache creates a GitCache that stores clones under cacheDir and
// treats them as fresh for ttl.
func NewGitCache(cacheDir string, ttl time.Duration) *GitCache {
	_ = os.MkdirAll(cacheDir, gitCacheDirPerm)

	return &GitCache{
		cacheDir:       cacheDir,
		ttl:            ttl,
		failureBackoff: gitCacheFailureBackoff,
		repos:          make(map[string]*cachedRepo),
		cloning:        make(map[string]chan struct{}),
	}
}

// FetchModuleContent clones (or reuses a cached clone of) rawURL at
// branch and returns the contents of filePath within it.
func (gc *GitCache) FetchModuleContent(ctx context.Context, rawURL, branch, filePath, token string) ([]byte, error) {
	key := gc.cacheKey(rawURL, branch)

	repoDir, err := gc.getOrClone(ctx, key, rawURL, branch, token)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Clean(filepath.Join(repoDir, filePath))
	repoRoot := filepath.Clean(repoDir) + string(os.PathSeparator)

	if !strings.HasPrefix(fullPath, repoRoot) {
		return nil, fmt.Errorf("path traversal detected: %q escapes repository root", filePath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file %s from cached repo: %w", filePath, err)
	}

	return data, nil
}

// Clear removes every cached repository from disk and forgets them.
func (gc *GitCache) Clear() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	for _, cached := range gc.repos {
		if cached.path != "" {
			_ = os.RemoveAll(cached.path)
		}
	}

	gc.repos = make(map[string]*cachedRepo)
}

// ClearRepo removes a single cached repository identified by its URL and
// branch.
func (gc *GitCache) ClearRepo(rawURL, branch string) {
	key := gc.cacheKey(rawURL, branch)
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if cached, ok := gc.repos[key]; ok {
		if cached.path != "" {
			_ = os.RemoveAll(cached.path)
		}

		delete(gc.repos, key)
	}
}

// cacheKey derives the cache map key for a repository URL and branch.
func (gc *GitCache) cacheKey(rawURL, branch string) string {
	h := sha256.Sum256([]byte(rawURL + ":" + branch))

	return hex.EncodeToString(h[:16])
}

// getOrClone returns the local path for rawURL/branch, cloning it first
// if it is not already cached and fresh.
func (gc *GitCache) getOrClone(ctx context.Context, key, rawURL, branch, token string) (string, error) {
	gc.mu.Lock()

	if cached, ok := gc.repos[key]; ok {
		if path, fresh := freshCachedPath(cached, gc.ttl); fresh {
			gc.mu.Unlock()

			return path, nil
		}

		failErr := recentCloneFailure(cached)
		if failErr != nil {
			gc.mu.Unlock()

			return "", failErr
		}
	}

	if wait, waiting := gc.cloning[key]; waiting {
		return gc.awaitClone(ctx, key, rawURL, branch, token, wait)
	}

	done := make(chan struct{})
	gc.cloning[key] = done
	gc.mu.Unlock()

	return gc.cloneAndCache(ctx, key, rawURL, branch, token, done)
}

// awaitClone blocks until an in-flight clone of key finishes, then reuses
// its result or retries getOrClone. gc.mu must be held on entry.
func (gc *GitCache) awaitClone(ctx context.Context, key, rawURL, branch, token string, wait chan struct{}) (string, error) {
	gc.mu.Unlock()
	<-wait
	gc.mu.Lock()

	if cached, ok := gc.repos[key]; ok {
		if cached.path != "" {
			path := cached.path
			gc.mu.Unlock()

			return path, nil
		}

		failErr := recentCloneFailure(cached)
		if failErr != nil {
			gc.mu.Unlock()

			return "", failErr
		}
	}

	gc.mu.Unlock()

	return gc.getOrClone(ctx, key, rawURL, branch, token)
}

// cloneAndCache clones rawURL/branch into the cache directory using
// go-git (pure Go, no external git binary required), records the outcome
// under key, and wakes any goroutines waiting on done.
// gc.mu must not be held on entry.
func (gc *GitCache) cloneAndCache(ctx context.Context, key, rawURL, branch, token string, done chan struct{}) (string, error) {
	repoDir := filepath.Join(gc.cacheDir, key)

	zap.L().Debug("cloning repo into cache", zap.String("url", rawURL), zap.String("branch", branch), zap.String("dir", repoDir))

	_, statErr := os.Stat(repoDir)
	if statErr == nil {
		_ = os.RemoveAll(repoDir)
	}

	_, err := git.PlainCloneContext(ctx, repoDir, false, &git.CloneOptions{
		URL:           rawURL,
		Auth:          buildAuth(rawURL, token),
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
		Tags:          git.NoTags,
	})
	if err != nil {
		_ = os.RemoveAll(repoDir)

		gc.mu.Lock()
		gc.repos[key] = &cachedRepo{failedUntil: time.Now().Add(gc.failureBackoff)}
		delete(gc.cloning, key)
		gc.mu.Unlock()
		close(done)

		msg := sanitizeGitOutput(err.Error(), token)

		zap.L().Error("git clone failed", zap.String("error", msg))

		return "", fmt.Errorf("git clone failed: %s", msg)
	}

	gc.mu.Lock()
	gc.repos[key] = &cachedRepo{path: repoDir, clonedAt: time.Now()}
	delete(gc.cloning, key)
	gc.evictLocked()
	gc.mu.Unlock()
	close(done)

	return repoDir, nil
}

// maxCachedRepos bounds how many cloned repositories the cache keeps on
// disk. A course catalog references a handful of repositories, so this only
// bites on an instance that has served content from many distinct ones —
// where, without it, the cache directory would grow forever.
const maxCachedRepos = 64

// evictLocked drops cached repositories until at most maxCachedRepos
// remain, oldest clone first. gc.mu must be held.
//
// Eviction is safe at any moment: a dropped repository is re-cloned on the
// next request for it, and a reader that is mid-read holds an open file
// descriptor, which keeps the bytes alive until it is done.
func (gc *GitCache) evictLocked() {
	if len(gc.repos) <= maxCachedRepos {
		return
	}

	keys := make([]string, 0, len(gc.repos))
	for key := range gc.repos {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(a, b string) int {
		return gc.repos[a].clonedAt.Compare(gc.repos[b].clonedAt)
	})

	for _, key := range keys[:len(gc.repos)-maxCachedRepos] {
		if path := gc.repos[key].path; path != "" {
			_ = os.RemoveAll(path)
		}

		delete(gc.repos, key)

		zap.L().Debug("evicted cached repo", zap.String("key", key))
	}
}

// freshCachedPath returns cached's path when present and still within
// ttl, and whether it was returned.
func freshCachedPath(cached *cachedRepo, ttl time.Duration) (string, bool) {
	if cached.path != "" && time.Since(cached.clonedAt) < ttl {
		return cached.path, true
	}

	return "", false
}

// recentCloneFailure returns a non-nil error if cached recorded a clone
// failure that is still within its backoff window.
func recentCloneFailure(cached *cachedRepo) error {
	if !cached.failedUntil.IsZero() && time.Now().Before(cached.failedUntil) {
		return fmt.Errorf("git clone failed recently, retry in %v", time.Until(cached.failedUntil))
	}

	return nil
}

// buildAuth returns an OAuth2 basic-auth credential for rawURL when it is a
// plain HTTP(S) URL and a token is supplied; nil otherwise (e.g. SSH URLs,
// which are left to the transport's default auth resolution).
func buildAuth(rawURL, token string) transport.AuthMethod { //nolint:ireturn // go-git's CloneOptions.Auth field is interface-typed
	if token == "" || !strings.HasPrefix(rawURL, "http") {
		return nil
	}

	return &githttp.BasicAuth{
		Username: gitOAuth2Username,
		Password: token,
	}
}

// sanitizeGitOutput trims output and redacts token from it, if present.
func sanitizeGitOutput(output, token string) string {
	out := strings.TrimSpace(output)
	if token != "" && strings.Contains(out, token) {
		out = strings.ReplaceAll(out, token, "***")
	}

	if out == "" {
		return gitNoCloneOutput
	}

	return out
}
