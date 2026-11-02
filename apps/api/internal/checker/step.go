package checker

import (
	"fmt"
	"regexp"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// Step check type identifiers and shared constants.
const (
	StepGitLabBranch                     = "gitlab_branch"
	StepGitLabMROpen                     = "gitlab_mr_open"
	StepGitLabConventionalCommit         = "gitlab_conventional_commit"
	StepGitLabPipelinePassed             = "gitlab_pipeline_passed"
	StepGitLabCommitOnBranch             = "gitlab_commit_on_branch"
	StepGitLabFileOnBranch               = "gitlab_file_on_branch"
	StepGitLabConventionalCommitOnBranch = "gitlab_conventional_commit_on_branch"

	// mrStateOpened is the GitLab MR state filter for open merge requests.
	mrStateOpened = "opened"
	// defaultBaseBranch is used when no base branch is specified in checkParams.
	defaultBaseBranch = "main"
	// noOpenMRMessage is returned when a check requires an open MR but none exists.
	noOpenMRMessage = "Aucune MR ouverte. Ouvrez d'abord une Merge Request."
	// gitLabPageSize is the number of items fetched per page in paginated calls.
	gitLabPageSize = 100
)

// conventionalCommitRe matches the first line of a conventional commit message.
var conventionalCommitRe = regexp.MustCompile(
	`^(feat|fix|chore|docs|style|refactor|test|perf|ci|build|revert)(\([^)]+\))?: .+`,
)

// StepRequest is sent to POST /check-step.
type StepRequest struct {
	Username    string         `json:"username"`
	Project     string         `json:"project"`
	CheckType   string         `json:"checkType"`
	CheckParams map[string]any `json:"checkParams"`
}

// StepResponse is returned by POST /check-step.
type StepResponse = EvaluateResponse

// stepState holds pre-fetched GitLab data shared across all check functions
// for a single CheckStep call.
type stepState struct {
	branches       []*gitlab.Branch
	openMRs        []*gitlab.BasicMergeRequest
	pipelineStatus map[int64]string               // IID -> pipeline status
	mrFullData     map[int64]*gitlab.MergeRequest // IID -> full MR (for pipeline)
}

// CheckStep validates that the project belongs to the requesting user, fetches
// GitLab state once, then dispatches to the appropriate check.
func (f *GitLabFetcher) CheckStep(req StepRequest) (*StepResponse, error) { //nolint:cyclop // dispatch switch over 7 check types; each case is a single delegating call
	parts := strings.Split(req.Project, "/")
	if parts[len(parts)-1] != req.Username {
		return &StepResponse{
			Allow:      false,
			Violations: []string{"Le projet ne correspond pas à votre compte GitLab."},
		}, nil
	}

	state, err := f.fetchStepState(req.Project)
	if err != nil {
		return nil, err
	}

	state.openMRs = mrsByAuthor(state.openMRs, req.Username)

	switch req.CheckType {
	case StepGitLabBranch:
		return checkBranch(req, state)
	case StepGitLabMROpen:
		return checkMROpen(state)
	case StepGitLabConventionalCommit:
		return f.checkConventionalCommit(req, state)
	case StepGitLabPipelinePassed:
		return f.checkPipelinePassed(req, state)
	case StepGitLabCommitOnBranch:
		return checkCommitOnBranch(req, state)
	case StepGitLabFileOnBranch:
		return f.checkFileOnBranch(req, state)
	case StepGitLabConventionalCommitOnBranch:
		return f.checkConventionalCommitOnBranch(req, state)
	default:
		return &StepResponse{
			Allow:      false,
			Violations: []string{"Type de vérification inconnu : " + req.CheckType},
		}, nil
	}
}

// fetchStepState pre-fetches branches and open MRs for a project so that
// individual check functions do not each issue their own list calls.
func (f *GitLabFetcher) fetchStepState(project string) (*stepState, error) {
	branches, err := f.listAllBranches(project)
	if err != nil {
		return nil, err
	}

	openMRs, err := f.listAllOpenMRs(project)
	if err != nil {
		return nil, err
	}

	state := &stepState{
		branches:       branches,
		openMRs:        openMRs,
		pipelineStatus: make(map[int64]string),
		mrFullData:     make(map[int64]*gitlab.MergeRequest),
	}

	// Pre-fetch pipeline status for each MR to avoid duplicate API calls in checkPipelinePassed
	for _, mr := range openMRs {
		fullMR, _, err := f.client.MergeRequests.GetMergeRequest(project, mr.IID, nil)
		if err == nil {
			state.mrFullData[mr.IID] = fullMR
			if fullMR.HeadPipeline != nil {
				state.pipelineStatus[mr.IID] = fullMR.HeadPipeline.Status
			}
		}
	}

	return state, nil
}

// listAllBranches returns every branch for a project, iterating over all pages.
func (f *GitLabFetcher) listAllBranches(project string) ([]*gitlab.Branch, error) {
	var all []*gitlab.Branch

	opts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{PerPage: gitLabPageSize, Page: 1},
	}

	for {
		branches, resp, err := f.client.Branches.ListBranches(project, opts)
		if err != nil {
			return nil, fmt.Errorf("list branches: %w", err)
		}

		all = append(all, branches...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return all, nil
}

// listAllOpenMRs returns every open merge request for a project, iterating
// over all pages.
func (f *GitLabFetcher) listAllOpenMRs(project string) ([]*gitlab.BasicMergeRequest, error) {
	var all []*gitlab.BasicMergeRequest

	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:       mrOpened(),
		ListOptions: gitlab.ListOptions{PerPage: gitLabPageSize, Page: 1},
	}

	for {
		mrs, resp, err := f.client.MergeRequests.ListProjectMergeRequests(project, opts)
		if err != nil {
			return nil, fmt.Errorf("list open MRs: %w", err)
		}

		all = append(all, mrs...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return all, nil
}

// strParam reads a string value from params.
func strParam(params map[string]any, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// mrOpened returns a pointer to the "opened" state string for GitLab API calls.
func mrOpened() *string {
	s := mrStateOpened

	return &s
}

// mrsByAuthor returns only MRs whose author matches the given username.
func mrsByAuthor(mrs []*gitlab.BasicMergeRequest, username string) []*gitlab.BasicMergeRequest {
	filtered := make([]*gitlab.BasicMergeRequest, 0, len(mrs))

	for _, mr := range mrs {
		if mr.Author != nil && mr.Author.Username == username {
			filtered = append(filtered, mr)
		}
	}

	return filtered
}

// isConventionalMessage reports whether msg is a conventional commit.
func isConventionalMessage(msg string) bool {
	firstLine, _, _ := strings.Cut(msg, "\n")

	return conventionalCommitRe.MatchString(firstLine)
}

// branchByPattern returns the first branch whose name starts with pattern.
func branchByPattern(branches []*gitlab.Branch, pattern string) string {
	for _, b := range branches {
		if strings.HasPrefix(b.Name, pattern) {
			return b.Name
		}
	}

	return ""
}

// checkBranch verifies that at least one branch matching pattern exists.
func checkBranch(req StepRequest, state *stepState) (*StepResponse, error) {
	pattern := strParam(req.CheckParams, "pattern")

	if branchByPattern(state.branches, pattern) != "" {
		return &StepResponse{Allow: true}, nil
	}

	return &StepResponse{
		Allow: false,
		Violations: []string{fmt.Sprintf(
			"Aucune branche commençant par %q trouvée. Créez une branche de type feature/ et poussez-la.",
			pattern,
		)},
	}, nil
}

// checkMROpen verifies that at least one MR is open in the project.
func checkMROpen(state *stepState) (*StepResponse, error) {
	if len(state.openMRs) > 0 {
		return &StepResponse{Allow: true}, nil
	}

	return &StepResponse{
		Allow:      false,
		Violations: []string{"Aucune Merge Request ouverte trouvée. Créez une MR sans la fusionner."},
	}, nil
}

// checkConventionalCommit verifies that at least one commit on an open MR
// follows conventional format.
func (f *GitLabFetcher) checkConventionalCommit(req StepRequest, state *stepState) (*StepResponse, error) {
	if len(state.openMRs) == 0 {
		return &StepResponse{
			Allow:      false,
			Violations: []string{noOpenMRMessage},
		}, nil
	}

	for _, mr := range state.openMRs {
		commits, err := f.listAllMRCommits(req.Project, mr.IID)
		if err != nil {
			continue
		}

		for _, commit := range commits {
			if isConventionalMessage(commit.Message) {
				return &StepResponse{Allow: true}, nil
			}
		}
	}

	return &StepResponse{
		Allow:      false,
		Violations: []string{"Aucun commit conventionnel trouvé sur la MR. Format attendu : feat: description, fix: description, etc."},
	}, nil
}

// listAllMRCommits returns every commit for a merge request, iterating over
// all pages.
func (f *GitLabFetcher) listAllMRCommits(project string, mrIID int64) ([]*gitlab.Commit, error) {
	var all []*gitlab.Commit

	opts := &gitlab.GetMergeRequestCommitsOptions{
		ListOptions: gitlab.ListOptions{PerPage: gitLabPageSize, Page: 1},
	}

	for {
		commits, resp, err := f.client.MergeRequests.GetMergeRequestCommits(project, mrIID, opts)
		if err != nil {
			return nil, fmt.Errorf("list MR commits: %w", err)
		}

		all = append(all, commits...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return all, nil
}

// checkPipelinePassed verifies that the pipeline on the open MR passed.
func (f *GitLabFetcher) checkPipelinePassed(_ StepRequest, state *stepState) (*StepResponse, error) {
	if len(state.openMRs) == 0 {
		return &StepResponse{
			Allow:      false,
			Violations: []string{noOpenMRMessage},
		}, nil
	}

	for _, mr := range state.openMRs {
		fullMR, ok := state.mrFullData[mr.IID]
		if !ok {
			continue
		}

		if fullMR.HeadPipeline == nil {
			return &StepResponse{
				Allow:      false,
				Violations: []string{"Aucune pipeline trouvée sur la MR. Poussez un commit pour déclencher la CI."},
			}, nil
		}

		if fullMR.HeadPipeline.Status == "success" {
			return &StepResponse{Allow: true}, nil
		}

		return &StepResponse{
			Allow:      false,
			Violations: []string{fmt.Sprintf("Pipeline non validée (statut : %s). Vérifiez les logs CI.", fullMR.HeadPipeline.Status)},
		}, nil
	}

	return &StepResponse{
		Allow:      false,
		Violations: []string{"Impossible de lire le statut de la pipeline."},
	}, nil
}

// checkCommitOnBranch verifies that a branch matching pattern has at least
// one commit.
func checkCommitOnBranch(req StepRequest, state *stepState) (*StepResponse, error) {
	pattern := strParam(req.CheckParams, "pattern")
	if pattern == "" {
		pattern = "feature/"
	}

	for _, b := range state.branches {
		if strings.HasPrefix(b.Name, pattern) && b.Commit != nil {
			return &StepResponse{Allow: true}, nil
		}
	}

	return &StepResponse{
		Allow: false,
		Violations: []string{fmt.Sprintf(
			"Aucune branche %s* avec commit trouvée. Créez une branche, faites un commit et poussez.",
			pattern,
		)},
	}, nil
}

// checkFileOnBranch verifies that a specific file was modified on a branch
// vs the base branch.
// checkParams: pattern (branch prefix), file (path), base (default: "main").
func (f *GitLabFetcher) checkFileOnBranch(req StepRequest, state *stepState) (*StepResponse, error) {
	pattern := strParam(req.CheckParams, "pattern")
	file := strParam(req.CheckParams, "file")

	base := strParam(req.CheckParams, "base")
	if base == "" {
		base = defaultBaseBranch
	}

	branch := branchByPattern(state.branches, pattern)
	if branch == "" {
		return &StepResponse{
			Allow:      false,
			Violations: []string{fmt.Sprintf("Aucune branche commençant par %q trouvée. Créez et poussez votre branche d'abord.", pattern)},
		}, nil
	}

	cmp, _, err := f.client.Repositories.Compare(req.Project, &gitlab.CompareOptions{
		From: &base,
		To:   &branch,
	})
	if err != nil {
		return nil, fmt.Errorf("compare branches: %w", err)
	}

	for _, diff := range cmp.Diffs {
		if diff.NewPath == file || diff.OldPath == file {
			return &StepResponse{Allow: true}, nil
		}
	}

	return &StepResponse{
		Allow:      false,
		Violations: []string{fmt.Sprintf("Le fichier %q n'a pas été modifié sur la branche. Ouvrez lab.py, modifiez le message et commitez.", file)},
	}, nil
}

// checkConventionalCommitOnBranch verifies that all commits on a branch
// (vs base) follow conventional format.
// checkParams: pattern (branch prefix), base (default: "main").
func (f *GitLabFetcher) checkConventionalCommitOnBranch(req StepRequest, state *stepState) (*StepResponse, error) {
	pattern := strParam(req.CheckParams, "pattern")

	base := strParam(req.CheckParams, "base")
	if base == "" {
		base = defaultBaseBranch
	}

	branch := branchByPattern(state.branches, pattern)
	if branch == "" {
		return &StepResponse{
			Allow:      false,
			Violations: []string{fmt.Sprintf("Aucune branche commençant par %q trouvée. Créez et poussez votre branche d'abord.", pattern)},
		}, nil
	}

	cmp, _, err := f.client.Repositories.Compare(req.Project, &gitlab.CompareOptions{
		From: &base,
		To:   &branch,
	})
	if err != nil {
		return nil, fmt.Errorf("compare branches: %w", err)
	}

	if len(cmp.Commits) == 0 {
		return &StepResponse{
			Allow:      false,
			Violations: []string{"Aucun commit trouvé sur la branche. Faites un commit et poussez-le."},
		}, nil
	}

	for _, commit := range cmp.Commits {
		if !isConventionalMessage(commit.Message) {
			firstLine, _, _ := strings.Cut(commit.Message, "\n")

			return &StepResponse{
				Allow:      false,
				Violations: []string{fmt.Sprintf("Commit non conventionnel : %q. Format attendu : feat: description, fix: description, etc.", firstLine)},
			}, nil
		}
	}

	return &StepResponse{Allow: true}, nil
}
