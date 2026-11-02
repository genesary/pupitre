package checker

import "testing"

// TestFetch_PaginatesEveryList drives Fetch with a mock that splits every
// list response across two pages, so the NextPage loops in countMergedMRs,
// fetchOpenMRs and listAllMRCommits are exercised.
func TestFetch_PaginatesEveryList(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.paginate = true
	m.project = map[string]any{
		"id": 7, "name": "p", "path_with_namespace": "grp/alice", "default_branch": "main",
	}
	m.mergedMRs = []map[string]any{{"iid": 1}, {"iid": 2}, {"iid": 3}}
	m.openMRs = []map[string]any{
		{"iid": 10, "title": "a", "source_branch": "feature/a"},
		{"iid": 11, "title": "b", "source_branch": "feature/b"},
	}
	m.mrByIID = map[string]any{"iid": 10, "head_pipeline": map[string]any{"status": "running"}}
	m.mrCommits = []map[string]any{{"message": "one"}, {"message": "two"}, {"message": "three"}}

	f, srv := m.fetcher()

	defer srv.Close()

	state, err := f.Fetch("grp/alice", nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if state.MergedMRCount != 3 {
		t.Errorf("MergedMRCount = %d, want 3 (both pages summed)", state.MergedMRCount)
	}

	if len(state.OpenMRs) != 2 {
		t.Fatalf("OpenMRs = %d, want 2 (both pages of the open-MR list)", len(state.OpenMRs))
	}

	// Fetch's per-MR commit read (buildOpenMRInfo) is deliberately single-page.
	if len(state.OpenMRs[0].Commits) != 1 {
		t.Errorf("first MR commits = %d, want 1", len(state.OpenMRs[0].Commits))
	}
}

// TestCheckStep_PaginatesBranchesAndMRs runs a branch-scoped conventional
// commit check where branches, open MRs and per-branch commits all paginate.
func TestCheckStep_PaginatesBranchesAndMRs(t *testing.T) {
	t.Parallel()

	m := newMockGitLab(t)
	m.paginate = true
	m.branches = []map[string]any{
		{"name": "main"},
		{"name": "feature/alice"},
	}
	m.openMRs = []map[string]any{
		{"iid": 1, "source_branch": "feature/alice", "author": map[string]any{"username": "alice"}},
		{"iid": 2, "source_branch": "feature/alice-2", "author": map[string]any{"username": "alice"}},
	}
	m.mrByIID = map[string]any{"iid": 1, "head_pipeline": nil}
	m.mrCommits = []map[string]any{
		{"message": "feat: one"},
		{"message": "fix: two"},
	}

	f, srv := m.fetcher()

	defer srv.Close()

	resp, err := f.CheckStep(StepRequest{
		Project:   "grp/alice",
		Username:  "alice",
		CheckType: StepGitLabConventionalCommit,
	})
	if err != nil {
		t.Fatalf("CheckStep: %v", err)
	}

	if !resp.Allow {
		t.Errorf("expected allow: a conventional commit exists across the paginated MRs, got %+v", resp)
	}
}
