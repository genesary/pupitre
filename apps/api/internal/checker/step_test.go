package checker

import (
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// TestStrParam reads string params and rejects non-string or missing keys.
func TestStrParam(t *testing.T) {
	t.Parallel()

	params := map[string]any{"pattern": "feature/", "num": 3}

	if got := strParam(params, "pattern"); got != "feature/" {
		t.Errorf("strParam pattern = %q", got)
	}

	if got := strParam(params, "num"); got != "" {
		t.Errorf("strParam num = %q, want empty (non-string)", got)
	}

	if got := strParam(params, "missing"); got != "" {
		t.Errorf("strParam missing = %q, want empty", got)
	}

	if got := strParam(nil, "x"); got != "" {
		t.Errorf("strParam nil map = %q", got)
	}
}

// TestMrOpened returns a pointer to the "opened" state literal.
func TestMrOpened(t *testing.T) {
	t.Parallel()

	if got := mrOpened(); got == nil || *got != "opened" {
		t.Errorf("mrOpened() = %v, want pointer to \"opened\"", got)
	}
}

// TestMrsByAuthor keeps only merge requests authored by the given user.
func TestMrsByAuthor(t *testing.T) {
	t.Parallel()

	mrs := []*gitlab.BasicMergeRequest{
		{IID: 1, Author: &gitlab.BasicUser{Username: "alice"}},
		{IID: 2, Author: &gitlab.BasicUser{Username: "bob"}},
		{IID: 3, Author: nil},
		{IID: 4, Author: &gitlab.BasicUser{Username: "alice"}},
	}

	got := mrsByAuthor(mrs, "alice")
	if len(got) != 2 {
		t.Fatalf("got %d MRs, want 2", len(got))
	}

	if got[0].IID != 1 || got[1].IID != 4 {
		t.Errorf("unexpected IIDs: %d, %d", got[0].IID, got[1].IID)
	}

	if len(mrsByAuthor(nil, "alice")) != 0 {
		t.Error("nil input should yield empty slice")
	}
}

// TestIsConventionalMessage classifies commit subject lines against the
// conventional-commit regex.
func TestIsConventionalMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		msg  string
		want bool
	}{
		{"feat: add thing", true},
		{"fix(scope): correct bug", true},
		{"chore: bump deps", true},
		{"refactor!: drop controller", false}, // "!" breaks the strict regex
		{"docs: update readme\n\nbody text", true},
		{"random commit message", false},
		{"feat add thing", false},
		{"WIP", false},
		{"feat:", false}, // no description after colon+space
	}

	for _, tc := range cases {
		if got := isConventionalMessage(tc.msg); got != tc.want {
			t.Errorf("isConventionalMessage(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// TestBranchByPattern returns the first branch whose name matches the
// prefix.
func TestBranchByPattern(t *testing.T) {
	t.Parallel()

	branches := []*gitlab.Branch{
		{Name: "main"},
		{Name: "feature/login"},
		{Name: "feature/logout"},
	}

	if got := branchByPattern(branches, "feature/"); got != "feature/login" {
		t.Errorf("branchByPattern = %q, want feature/login", got)
	}

	if got := branchByPattern(branches, "hotfix/"); got != "" {
		t.Errorf("branchByPattern no-match = %q, want empty", got)
	}

	if got := branchByPattern(nil, "x"); got != "" {
		t.Errorf("branchByPattern nil = %q", got)
	}
}

// TestCheckBranch allows when a branch matches the pattern and denies
// otherwise.
func TestCheckBranch(t *testing.T) {
	t.Parallel()

	state := &stepState{branches: []*gitlab.Branch{{Name: "feature/x"}}}

	req := StepRequest{CheckParams: map[string]any{"pattern": "feature/"}}
	resp, _ := checkBranch(req, state)

	if !resp.Allow {
		t.Errorf("expected allow when matching branch exists")
	}

	req.CheckParams = map[string]any{"pattern": "nope/"}
	resp, _ = checkBranch(req, state)

	if resp.Allow || len(resp.Violations) == 0 {
		t.Errorf("expected deny with a violation, got %+v", resp)
	}
}

// TestCheckMROpen allows when at least one MR is open.
func TestCheckMROpen(t *testing.T) {
	t.Parallel()

	resp, _ := checkMROpen(&stepState{openMRs: []*gitlab.BasicMergeRequest{{IID: 1}}})
	if !resp.Allow {
		t.Error("expected allow when an MR is open")
	}

	resp, _ = checkMROpen(&stepState{})
	if resp.Allow || len(resp.Violations) == 0 {
		t.Errorf("expected deny with violation, got %+v", resp)
	}
}

// TestCheckCommitOnBranch requires a matching branch that carries a commit.
func TestCheckCommitOnBranch(t *testing.T) {
	t.Parallel()

	state := &stepState{branches: []*gitlab.Branch{
		{Name: "feature/x", Commit: &gitlab.Commit{ID: "abc"}},
	}}

	// default pattern "feature/"
	resp, _ := checkCommitOnBranch(StepRequest{CheckParams: map[string]any{}}, state)
	if !resp.Allow {
		t.Error("expected allow: feature/ branch has a commit")
	}

	// branch present but no commit
	state.branches = []*gitlab.Branch{{Name: "feature/x"}}
	resp, _ = checkCommitOnBranch(StepRequest{CheckParams: map[string]any{}}, state)

	if resp.Allow {
		t.Error("expected deny: branch has no commit")
	}

	// explicit custom pattern that does not match
	resp, _ = checkCommitOnBranch(StepRequest{CheckParams: map[string]any{"pattern": "release/"}}, state)
	if resp.Allow {
		t.Error("expected deny: no release/ branch")
	}
}

// TestCheckPipelinePassed walks the open-MR pipeline states from missing
// data to success and failure.
func TestCheckPipelinePassed(t *testing.T) {
	t.Parallel()

	f := &GitLabFetcher{}

	// no open MRs
	resp, _ := f.checkPipelinePassed(StepRequest{}, &stepState{})
	if resp.Allow {
		t.Error("expected deny when no open MR")
	}

	mr := &gitlab.BasicMergeRequest{IID: 7}

	// MR present but no full data -> falls through to final violation
	resp, _ = f.checkPipelinePassed(StepRequest{}, &stepState{
		openMRs:    []*gitlab.BasicMergeRequest{mr},
		mrFullData: map[int64]*gitlab.MergeRequest{},
	})
	if resp.Allow {
		t.Error("expected deny when full MR data missing")
	}

	// full data, no pipeline
	resp, _ = f.checkPipelinePassed(StepRequest{}, &stepState{
		openMRs:    []*gitlab.BasicMergeRequest{mr},
		mrFullData: map[int64]*gitlab.MergeRequest{7: {}},
	})
	if resp.Allow {
		t.Error("expected deny when no pipeline")
	}

	// pipeline success
	full := &gitlab.MergeRequest{}
	full.HeadPipeline = &gitlab.Pipeline{Status: "success"}
	resp, _ = f.checkPipelinePassed(StepRequest{}, &stepState{
		openMRs:    []*gitlab.BasicMergeRequest{mr},
		mrFullData: map[int64]*gitlab.MergeRequest{7: full},
	})

	if !resp.Allow {
		t.Error("expected allow when pipeline succeeded")
	}

	// pipeline failed
	failed := &gitlab.MergeRequest{}
	failed.HeadPipeline = &gitlab.Pipeline{Status: "failed"}
	resp, _ = f.checkPipelinePassed(StepRequest{}, &stepState{
		openMRs:    []*gitlab.BasicMergeRequest{mr},
		mrFullData: map[int64]*gitlab.MergeRequest{7: failed},
	})

	if resp.Allow || len(resp.Violations) == 0 {
		t.Errorf("expected deny with violation, got %+v", resp)
	}
}

// TestCheckStep_UsernameMismatch denies when the project path does not end
// with the username.
func TestCheckStep_UsernameMismatch(t *testing.T) {
	t.Parallel()

	f := &GitLabFetcher{}

	resp, err := f.CheckStep(StepRequest{Project: "group/other", Username: "alice", CheckType: StepGitLabBranch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Allow || len(resp.Violations) == 0 {
		t.Errorf("expected deny for project/username mismatch, got %+v", resp)
	}
}

// TestCheckStep_UnknownType denies for an unrecognised check type.
func TestCheckStep_UnknownType(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{}
	m.openMRs = []map[string]any{}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{Project: "group/alice", Username: "alice", CheckType: "bogus"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Allow || len(resp.Violations) == 0 {
		t.Errorf("expected deny for unknown check type, got %+v", resp)
	}
}

// TestCheckStep_DispatchBranch routes a gitlab_branch check end to end
// against the mock.
func TestCheckStep_DispatchBranch(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{{"name": "feature/alice"}}
	m.openMRs = []map[string]any{}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabBranch,
		CheckParams: map[string]any{"pattern": "feature/"},
	})
	if err != nil {
		t.Fatalf("CheckStep: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow, got %+v", resp)
	}
}

// TestCheckStep_DispatchConventionalCommit routes a
// gitlab_conventional_commit check end to end.
func TestCheckStep_DispatchConventionalCommit(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{}
	m.openMRs = []map[string]any{
		{"iid": 1, "title": "t", "source_branch": "feature/alice", "author": map[string]any{"username": "alice"}},
	}
	m.mrByIID = map[string]any{"iid": 1, "head_pipeline": nil}
	m.mrCommits = []map[string]any{{"message": "feat: implement lab"}}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:   "group/alice",
		Username:  "alice",
		CheckType: StepGitLabConventionalCommit,
	})
	if err != nil {
		t.Fatalf("CheckStep: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow for conventional commit, got %+v", resp)
	}
}

// TestCheckStep_DispatchFileOnBranch routes a gitlab_file_on_branch check
// for present and absent diffs.
func TestCheckStep_DispatchFileOnBranch(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{{"name": "feature/alice"}}
	m.openMRs = []map[string]any{}
	m.compareDiffs = []map[string]any{{"new_path": "lab.py", "old_path": "lab.py"}}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabFileOnBranch,
		CheckParams: map[string]any{"pattern": "feature/", "file": "lab.py"},
	})
	if err != nil {
		t.Fatalf("CheckStep: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow: lab.py modified, got %+v", resp)
	}

	// file not in diff -> deny
	m.compareDiffs = []map[string]any{{"new_path": "other.py", "old_path": "other.py"}}
	resp, _ = f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabFileOnBranch,
		CheckParams: map[string]any{"pattern": "feature/", "file": "lab.py"},
	})

	if resp.Allow {
		t.Errorf("expected deny: lab.py not modified, got %+v", resp)
	}
}

// TestCheckStep_DispatchConventionalCommitOnBranch routes the branch-scoped
// conventional-commit check across its outcomes.
func TestCheckStep_DispatchConventionalCommitOnBranch(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{{"name": "feature/alice"}}
	m.openMRs = []map[string]any{}
	m.compareCmts = []map[string]any{
		{"message": "feat: one"},
		{"message": "fix: two"},
	}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabConventionalCommitOnBranch,
		CheckParams: map[string]any{"pattern": "feature/"},
	})
	if err != nil {
		t.Fatalf("CheckStep: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow: all commits conventional, got %+v", resp)
	}

	// one bad commit -> deny
	m.compareCmts = []map[string]any{{"message": "feat: one"}, {"message": "bad commit"}}
	resp, _ = f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabConventionalCommitOnBranch,
		CheckParams: map[string]any{"pattern": "feature/"},
	})

	if resp.Allow {
		t.Errorf("expected deny: non-conventional commit present, got %+v", resp)
	}

	// no commits -> deny
	m.compareCmts = []map[string]any{}
	resp, _ = f.CheckStep(StepRequest{
		Project:     "group/alice",
		Username:    "alice",
		CheckType:   StepGitLabConventionalCommitOnBranch,
		CheckParams: map[string]any{"pattern": "feature/"},
	})

	if resp.Allow {
		t.Errorf("expected deny: no commits, got %+v", resp)
	}
}

// TestCheckStep_MROpenAndPipeline routes gitlab_mr_open and
// gitlab_pipeline_passed checks against the mock.
func TestCheckStep_MROpenAndPipeline(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.branches = []map[string]any{}
	m.openMRs = []map[string]any{
		{"iid": 2, "source_branch": "feature/alice", "author": map[string]any{"username": "alice"}},
	}
	m.mrByIID = map[string]any{
		"iid":           2,
		"head_pipeline": map[string]any{"status": "success"},
	}
	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:   "group/alice",
		Username:  "alice",
		CheckType: StepGitLabMROpen,
	})
	if err != nil {
		t.Fatalf("CheckStep MROpen: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow: MR open, got %+v", resp)
	}

	resp, err = f.CheckStep(StepRequest{
		Project:   "group/alice",
		Username:  "alice",
		CheckType: StepGitLabPipelinePassed,
	})
	if err != nil {
		t.Fatalf("CheckStep PipelinePassed: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow: pipeline success, got %+v", resp)
	}
}
