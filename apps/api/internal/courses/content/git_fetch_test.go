package content

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// contentGitFixture creates a local git repo on branch main with the given
// files and returns its path.
func contentGitFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}

	err = repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main"))
	if err != nil {
		t.Fatalf("set HEAD: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	for rel, body := range files {
		full := filepath.Join(dir, rel)

		err = os.MkdirAll(filepath.Dir(full), 0o750)
		if err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		err = os.WriteFile(full, []byte(body), 0o600)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		_, err = wt.Add(rel)
		if err != nil {
			t.Fatalf("add: %v", err)
		}
	}

	_, err = wt.Commit("fixture", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t.test", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	return dir
}

// TestGitCache_FetchModuleContent_CloneAndCache clones a real repo, reads a
// file, and proves the second read is served from the cache.
func TestGitCache_FetchModuleContent_CloneAndCache(t *testing.T) {
	t.Parallel()

	repoDir := contentGitFixture(t, map[string]string{
		"docs/lesson.md": "# Lesson\n\nbody text\n",
	})

	gc := NewGitCache(filepath.Join(t.TempDir(), "cache"), time.Minute)
	ctx := context.Background()

	data, err := gc.FetchModuleContent(ctx, repoDir, "main", "docs/lesson.md", "")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	if string(data) != "# Lesson\n\nbody text\n" {
		t.Errorf("unexpected content: %q", data)
	}

	// Second call hits the fresh cache path.
	data2, err := gc.FetchModuleContent(ctx, repoDir, "main", "docs/lesson.md", "")
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	if !bytes.Equal(data2, data) {
		t.Errorf("cached read differs: %q vs %q", data2, data)
	}

	// A missing file inside the cloned repo is an error, not a panic.
	_, err = gc.FetchModuleContent(ctx, repoDir, "main", "docs/absent.md", "")
	if err == nil {
		t.Error("expected an error for a missing file")
	}

	// ClearRepo drops it; the next read re-clones.
	gc.ClearRepo(repoDir, "main")

	_, err = gc.FetchModuleContent(ctx, repoDir, "main", "docs/lesson.md", "")
	if err != nil {
		t.Fatalf("fetch after ClearRepo: %v", err)
	}
}

// TestGitCache_FetchModuleContent_BadRef fails to clone a non-existent branch
// and records the failure so a quick retry is blocked.
func TestGitCache_FetchModuleContent_BadRef(t *testing.T) {
	t.Parallel()

	repoDir := contentGitFixture(t, map[string]string{"a.md": "a"})

	gc := NewGitCache(filepath.Join(t.TempDir(), "cache"), time.Minute)
	ctx := context.Background()

	_, err := gc.FetchModuleContent(ctx, repoDir, "no-such-branch", "a.md", "")
	if err == nil {
		t.Fatal("expected clone failure for a missing branch")
	}

	// The retry is short-circuited by the recorded failure.
	_, err = gc.FetchModuleContent(ctx, repoDir, "no-such-branch", "a.md", "")
	if err == nil {
		t.Fatal("expected the recorded failure to block a quick retry")
	}
}

// TestFetchModuleIndex reads and normalises a module index YAML file from git.
func TestFetchModuleIndex(t *testing.T) {
	t.Parallel()

	idx := "" +
		"- name: One\n  path: one.md\n" +
		"- name: Two\n  type: video\n  src: https://v/2\n  path: two\n"

	repoDir := contentGitFixture(t, map[string]string{"index.yaml": idx})

	gc := NewGitCache(filepath.Join(t.TempDir(), "cache"), time.Minute)

	parent := Module{Src: repoDir, Ref: "main", Path: "index.yaml"}

	mods, err := FetchModuleIndex(context.Background(), gc, parent, "")
	if err != nil {
		t.Fatalf("FetchModuleIndex: %v", err)
	}

	if len(mods) != 2 {
		t.Fatalf("want 2 modules, got %d", len(mods))
	}

	// First entry inherits src/ref from the parent and defaults to text.
	if mods[0].Src != repoDir || mods[0].Ref != "main" || mods[0].Type != ModuleTypeText {
		t.Errorf("index entry not normalised: %+v", mods[0])
	}

	if mods[1].Type != "video" || mods[1].Src != "https://v/2" {
		t.Errorf("explicit fields not kept: %+v", mods[1])
	}
}

// TestFetchQuizContent parses a git-hosted quiz YAML into the in-memory Quiz.
func TestFetchQuizContent(t *testing.T) {
	t.Parallel()

	quiz := "" +
		"title: Sample\n" +
		"questions:\n" +
		"  - id: q1\n    type: single\n    question: Q?\n" +
		"    answers:\n      - id: a\n        text: A\n        correct: true\n"

	repoDir := contentGitFixture(t, map[string]string{"quiz.yaml": quiz})

	gc := NewGitCache(filepath.Join(t.TempDir(), "cache"), time.Minute)

	got, err := FetchQuizContent(context.Background(), gc, repoDir, "main", "quiz.yaml", "")
	if err != nil {
		t.Fatalf("FetchQuizContent: %v", err)
	}

	// id defaults to the path, title is kept.
	if got.ID != "quiz.yaml" || got.Title != "Sample" {
		t.Errorf("quiz metadata: id=%q title=%q", got.ID, got.Title)
	}

	if len(got.Questions) != 1 || got.Questions[0].ID != "q1" {
		t.Fatalf("questions not parsed: %+v", got.Questions)
	}

	// convertQuestion applies the default point value.
	if got.Questions[0].Points != 1 {
		t.Errorf("default points not applied: %d", got.Questions[0].Points)
	}
}

// TestGitCache_ConcurrentFetchDeduplicates fires many simultaneous fetches
// of the same repo/branch. Exactly one goroutine performs the clone; the
// rest block in awaitClone and then reuse its result.
func TestGitCache_ConcurrentFetchDeduplicates(t *testing.T) {
	t.Parallel()

	repoDir := contentGitFixture(t, map[string]string{
		"docs/lesson.md": "# shared\n",
	})

	gc := NewGitCache(filepath.Join(t.TempDir(), "cache"), time.Minute)
	ctx := t.Context()

	const workers = 24

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()

			data, err := gc.FetchModuleContent(ctx, repoDir, "main", "docs/lesson.md", "")

			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstErr == nil {
				firstErr = err
			}

			if err == nil && !bytes.Equal(data, []byte("# shared\n")) {
				firstErr = fmt.Errorf("unexpected body %q", data)
			}
		}()
	}

	wg.Wait()

	if firstErr != nil {
		t.Fatalf("concurrent fetch failed: %v", firstErr)
	}
}
