package checker

import (
	"context"
	"strings"
	"testing"
)

// TestFetchCourseCheckPolicyContent_InvalidURL rejects sources that are not
// http(s) URLs.
func TestFetchCourseCheckPolicyContent_InvalidURL(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"://nope",
		"ftp://gitlab.local/group/repo.git",
		"not a url at all",
	}

	for _, src := range cases {
		_, err := FetchCourseCheckPolicyContent(context.Background(), src, "main", "check.rego", "")
		if err == nil {
			t.Errorf("src %q: expected error, got nil", src)
		}
	}
}

// TestFetchCourseCheckPolicyContent_Success fetches and base64-decodes a
// policy file from the mock GitLab.
func TestFetchCourseCheckPolicyContent_Success(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.files["check.rego"] = "package checker.lab\n\ndefault allow := true\n"
	_, srv := m.fetcher()

	defer srv.Close()

	src := srv.URL + "/group/alice.git"

	content, err := FetchCourseCheckPolicyContent(context.Background(), src, "main", "check.rego", "tok")
	if err != nil {
		t.Fatalf("FetchCourseCheckPolicyContent: %v", err)
	}

	if !strings.Contains(content, "package checker.lab") {
		t.Errorf("unexpected policy content: %q", content)
	}
}

// TestFetchCourseCheckPolicyContent_MissingFile surfaces an error when the
// policy file is absent.
func TestFetchCourseCheckPolicyContent_MissingFile(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	_, srv := m.fetcher()

	defer srv.Close()

	_, err := FetchCourseCheckPolicyContent(context.Background(), srv.URL+"/group/alice", "main", "absent.rego", "")
	if err == nil {
		t.Fatal("expected error for missing policy file")
	}
}

// TestNewFetcher_Error returns an error when the base URL is malformed.
func TestNewFetcher_Error(t *testing.T) {
	t.Parallel()

	// A control character in the base URL makes url.Parse fail inside the SDK.
	_, err := NewFetcher("tok", "http://exa\x7fmple.com")
	if err == nil {
		t.Fatal("expected NewFetcher error for malformed base URL")
	}
}

// TestFetch_Success assembles project, MR, pipeline, commit and file state
// from the mock.
func TestFetch_Success(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.project = map[string]any{
		"id":                  42,
		"name":                "alice-lab",
		"path_with_namespace": "group/alice",
		"default_branch":      "main",
	}
	m.mergedMRs = []map[string]any{{"iid": 1}, {"iid": 2}}
	m.openMRs = []map[string]any{
		{"iid": 5, "title": "WIP", "source_branch": "feature/x"},
	}
	m.mrByIID = map[string]any{"iid": 5, "head_pipeline": map[string]any{"status": "running"}}
	m.mrCommits = []map[string]any{{"message": "feat: x"}}
	m.files["lab.py"] = "print('hi')"

	f, srv := m.fetcher()

	defer srv.Close()

	state, err := f.Fetch("group/alice", []string{"lab.py", "missing.txt"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if state.Project == nil || state.Project.Name != "alice-lab" {
		t.Fatalf("unexpected project: %+v", state.Project)
	}

	if state.Project.ID != "42" {
		t.Errorf("project ID = %q, want \"42\"", state.Project.ID)
	}

	if state.MergedMRCount != 2 {
		t.Errorf("MergedMRCount = %d, want 2", state.MergedMRCount)
	}

	if len(state.OpenMRs) != 1 {
		t.Fatalf("OpenMRs = %d, want 1", len(state.OpenMRs))
	}

	if state.OpenMRs[0].PipelineStatus != "running" {
		t.Errorf("PipelineStatus = %q, want running", state.OpenMRs[0].PipelineStatus)
	}

	if len(state.OpenMRs[0].Commits) != 1 || state.OpenMRs[0].Commits[0].Message != "feat: x" {
		t.Errorf("unexpected commits: %+v", state.OpenMRs[0].Commits)
	}

	if state.Files["lab.py"] != "print('hi')" {
		t.Errorf("Files[lab.py] = %q", state.Files["lab.py"])
	}

	if _, ok := state.Files["missing.txt"]; ok {
		t.Error("missing.txt should have been skipped")
	}
}

// TestFetch_ProjectError wraps the GetProject failure.
func TestFetch_ProjectError(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.projectStatus = 500

	f, srv := m.fetcher()

	defer srv.Close()

	_, err := f.Fetch("group/alice", nil)
	if err == nil {
		t.Fatal("expected error when GetProject fails")
	}

	if !strings.Contains(err.Error(), "GetProject") {
		t.Errorf("error = %v, want GetProject wrap", err)
	}
}

// TestFetch_DefaultBranchWhenNoOpenMR reads files from the default branch
// when no MR is open.
func TestFetch_DefaultBranchWhenNoOpenMR(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.project = map[string]any{
		"id": 1, "name": "n", "path_with_namespace": "group/alice", "default_branch": "trunk",
	}
	m.openMRs = []map[string]any{}
	m.mergedMRs = []map[string]any{}
	m.files["a.txt"] = "content-a"

	f, srv := m.fetcher()

	defer srv.Close()

	state, err := f.Fetch("group/alice", []string{"a.txt"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if state.Files["a.txt"] != "content-a" {
		t.Errorf("expected file fetched from default branch, got %q", state.Files["a.txt"])
	}

	if len(state.OpenMRs) != 0 {
		t.Errorf("expected no open MRs, got %d", len(state.OpenMRs))
	}
}
