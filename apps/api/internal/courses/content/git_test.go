package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFetchModuleContent_PathTraversal verifies that filePath values
// containing ".." segments or pointing outside the cached repo directory
// are rejected before any filesystem read is attempted.
func TestFetchModuleContent_PathTraversal(t *testing.T) {
	t.Parallel()

	// Build a fake on-disk repo structure: <cacheDir>/<repoKey>/secret.txt
	// and a sentinel file outside the repo at <cacheDir>/outside.txt.
	cacheDir := t.TempDir()
	repoKey := "fakerepo"
	repoDir := filepath.Join(cacheDir, repoKey)

	err := os.MkdirAll(repoDir, 0o750)
	if err != nil {
		t.Fatalf("mkdir repoDir: %v", err)
	}

	err = os.WriteFile(filepath.Join(repoDir, "content.md"), []byte("ok"), 0o600)
	if err != nil {
		t.Fatalf("write content.md: %v", err)
	}

	outside := filepath.Join(cacheDir, "outside.txt")

	err = os.WriteFile(outside, []byte("secret"), 0o600)
	if err != nil {
		t.Fatalf("write outside.txt: %v", err)
	}

	gc := &GitCache{
		cacheDir:       cacheDir,
		ttl:            time.Hour,
		failureBackoff: gitCacheFailureBackoff,
		repos:          make(map[string]*cachedRepo),
		cloning:        make(map[string]chan struct{}),
	}

	// Inject a pre-cloned repo entry so FetchModuleContent skips the real clone.
	cacheKey := gc.cacheKey("https://fake.example.com/repo", "main")
	gc.repos[cacheKey] = &cachedRepo{path: repoDir, clonedAt: time.Now()}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{"normal file", "content.md", false},
		{"dot-dot escape", "../../outside.txt", true},
		{"deep dot-dot escape", "../" + repoKey + "/../outside.txt", true},
		{"absolute path to /etc/passwd", "/etc/passwd", true},
		{"null-byte embedded", "content\x00../../outside.txt", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := gc.FetchModuleContent(t.Context(), "https://fake.example.com/repo", "main", tc.filePath, "")
			if tc.wantErr && err == nil {
				t.Errorf("expected error for filePath %q, got nil", tc.filePath)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for filePath %q: %v", tc.filePath, err)
			}
		})
	}
}
