package checker

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	"go.uber.org/zap"
)

// FetchCourseCheckPolicyContent fetches a Rego policy file from a GitLab repo
// using the GitLab SDK. src is the git clone URL, ref is the branch/tag,
// filePath is the relative path to the .rego file, and token is the OAuth2
// access token. The context is accepted for API compatibility but the SDK
// handles its own request lifecycle.
func FetchCourseCheckPolicyContent(_ context.Context, src, ref, filePath, token string) (string, error) {
	parsed, err := url.Parse(src)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid policy source URL: %q", src)
	}

	baseURL := parsed.Scheme + "://" + parsed.Host

	fetcher, err := NewFetcher(token, baseURL)
	if err != nil {
		return "", fmt.Errorf("create gitlab client: %w", err)
	}

	// Strip scheme+host prefix and optional .git suffix to obtain the project path.
	projectPath := strings.TrimSuffix(strings.TrimPrefix(src, baseURL+"/"), ".git")

	return fetcher.FetchPolicy(projectPath, ref, filePath)
}

// GitLabFetcher fetches project, merge request, and file state from GitLab
// for policy evaluation.
type GitLabFetcher struct {
	client *gitlab.Client
}

// NewFetcher creates a GitLabFetcher authenticated against the given GitLab
// instance.
func NewFetcher(token, baseURL string) (*GitLabFetcher, error) {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("gitlab new client: %w", err)
	}

	return &GitLabFetcher{client: client}, nil
}

// Fetch gathers project info, merge request state, and file contents for the
// given project from GitLab.
func (f *GitLabFetcher) Fetch(project string, files []string) (*GitLabState, error) {
	state := &GitLabState{Files: make(map[string]string)}

	proj, _, err := f.client.Projects.GetProject(project, &gitlab.GetProjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("GetProject %q: %w", project, err)
	}

	state.Project = &projectInfo{
		ID:            strconv.FormatInt(proj.ID, 10),
		Name:          proj.Name,
		Path:          proj.PathWithNamespace,
		DefaultBranch: proj.DefaultBranch,
	}

	state.MergedMRCount = f.countMergedMRs(project)
	state.OpenMRs = f.fetchOpenMRs(project)

	// File contents — check on the source branch of the first open MR (if any), else default branch
	ref := proj.DefaultBranch
	if len(state.OpenMRs) > 0 {
		ref = state.OpenMRs[0].SourceBranch
	}

	state.Files = f.fetchFiles(project, ref, files)

	return state, nil
}

// FetchPolicy retrieves a Rego policy file from the given GitLab project at
// the given ref using the SDK, avoiding any direct HTTP calls to user-supplied
// URLs.
func (f *GitLabFetcher) FetchPolicy(project, ref, filePath string) (string, error) {
	file, _, err := f.client.RepositoryFiles.GetFile(project, filePath, &gitlab.GetFileOptions{
		Ref: &ref,
	})
	if err != nil {
		return "", fmt.Errorf("fetch policy %q at ref %q: %w", filePath, ref, err)
	}

	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return "", fmt.Errorf("decode policy content: %w", err)
	}

	return string(decoded), nil
}

// countMergedMRs returns the total number of merged merge requests for the
// project, paginating to ensure an accurate count.
func (f *GitLabFetcher) countMergedMRs(project string) int {
	merged := "merged"
	total := 0
	page := int64(1)

	for {
		mrs, resp, err := f.client.MergeRequests.ListProjectMergeRequests(project, &gitlab.ListProjectMergeRequestsOptions{
			State:       &merged,
			ListOptions: gitlab.ListOptions{PerPage: gitLabPageSize, Page: page},
		})
		if err != nil {
			zap.L().Warn("list merged MRs failed", zap.String("project", project), zap.Error(err))

			break
		}

		total += len(mrs)

		if resp.NextPage == 0 {
			break
		}

		page = resp.NextPage
	}

	return total
}

// fetchOpenMRs returns all open merge requests for the project, enriched with
// pipeline status and commits, paginating to retrieve every result.
func (f *GitLabFetcher) fetchOpenMRs(project string) []openMRInfo {
	openMRs, err := f.listAllOpenMRs(project)
	if err != nil {
		zap.L().Warn("list open MRs failed", zap.String("project", project), zap.Error(err))

		return nil
	}

	infos := make([]openMRInfo, 0, len(openMRs))
	for _, mergeRequest := range openMRs {
		infos = append(infos, f.buildOpenMRInfo(project, mergeRequest))
	}

	return infos
}

// buildOpenMRInfo enriches a merge request with pipeline status and commits.
func (f *GitLabFetcher) buildOpenMRInfo(project string, mergeRequest *gitlab.BasicMergeRequest) openMRInfo {
	info := openMRInfo{
		IID:          mergeRequest.IID,
		Title:        mergeRequest.Title,
		SourceBranch: mergeRequest.SourceBranch,
	}

	// ListProjectMergeRequests does not populate HeadPipeline; fetch individually
	fullMR, _, err := f.client.MergeRequests.GetMergeRequest(project, mergeRequest.IID, &gitlab.GetMergeRequestsOptions{})
	if err == nil && fullMR.HeadPipeline != nil {
		info.PipelineStatus = fullMR.HeadPipeline.Status
	}

	commits, _, err := f.client.MergeRequests.GetMergeRequestCommits(project, mergeRequest.IID, nil)
	if err == nil {
		for _, commit := range commits {
			info.Commits = append(info.Commits, commitInfo{Message: commit.Message})
		}
	}

	return info
}

// fetchFiles reads the given file paths from the given ref, skipping any
// that fail to fetch or decode.
func (f *GitLabFetcher) fetchFiles(project, ref string, files []string) map[string]string {
	contents := make(map[string]string, len(files))

	for _, filePath := range files {
		file, _, err := f.client.RepositoryFiles.GetFile(project, filePath, &gitlab.GetFileOptions{
			Ref: &ref,
		})
		if err != nil {
			zap.L().Warn("fetch file failed", zap.String("project", project), zap.String("path", filePath), zap.Error(err))

			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			decoded = []byte(file.Content)
		}

		contents[filePath] = string(decoded)
	}

	return contents
}
