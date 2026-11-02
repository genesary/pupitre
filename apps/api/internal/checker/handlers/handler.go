// Package handlers wires up the checker-service HTTP API.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/checker"
	"github.com/jojo91550/pupitre2/apps/api/internal/checker/config"
)

// Handler serves the checker-service HTTP API.
type Handler struct {
	config *config.Config
}

// New creates a Handler using the given configuration.
func New(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// Failure is a check the checker refused or could not complete: the
// status and message checker-service answered with.
type Failure struct {
	Status  int
	Message string
}

// failure builds a Failure.
func failure(status int, message string) *Failure {
	return &Failure{Status: status, Message: message}
}

// RunEvaluate runs one lab policy evaluation. POST /evaluate decodes its
// body and calls it; the monolith calls it directly
// (docs/specs/107-backend-labs-and-checker.md).
func (h *Handler) RunEvaluate(ctx context.Context, req checker.EvaluateRequest) (*checker.EvaluateResponse, *Failure) {
	if fail := requireEvaluateFields(req); fail != nil {
		return nil, fail
	}

	if h.config.GitLabToken == "" {
		return nil, failure(http.StatusInternalServerError, "GITLAB_TOKEN not configured")
	}

	err := h.validateEvaluateRequest(req)
	if err != nil {
		return nil, failure(http.StatusBadRequest, err.Error())
	}

	policy, fail := h.fetchCourseCheckPolicy(ctx, req)
	if fail != nil {
		return nil, fail
	}

	state, fail := h.fetchStudentGitLabProjectState(req)
	if fail != nil {
		return nil, fail
	}

	result, err := checker.Evaluate(ctx, policy, state)
	if err != nil {
		zap.L().Error("rego eval", zap.Error(err))

		return nil, failure(http.StatusInternalServerError, "policy evaluation error")
	}

	return result, nil
}

// requireEvaluateFields refuses an evaluation that names no learner,
// project or policy, with the messages of POST /evaluate.
func requireEvaluateFields(req checker.EvaluateRequest) *Failure {
	if req.Username == "" || req.Project == "" {
		return failure(http.StatusBadRequest, "username and project are required")
	}

	if req.PolicySrc == "" || req.PolicyRef == "" || req.PolicyPath == "" {
		return failure(http.StatusBadRequest, "policySrc, policyRef and policyPath are required")
	}

	return nil
}

// RunCheckStep runs one named step check. POST /check-step decodes its
// body and calls it; the monolith calls it directly.
func (h *Handler) RunCheckStep(req checker.StepRequest) (*checker.StepResponse, *Failure) {
	if req.Username == "" || req.Project == "" || req.CheckType == "" {
		return nil, failure(http.StatusBadRequest, "username, project and checkType are required")
	}

	if h.config.GitLabToken == "" {
		return nil, failure(http.StatusInternalServerError, "GITLAB_TOKEN not configured")
	}

	fetcher, err := checker.NewFetcher(h.config.GitLabToken, h.config.GitLabBaseURL)
	if err != nil {
		zap.L().Error("gitlab client init", zap.Error(err))

		return nil, failure(http.StatusInternalServerError, "gitlab client error")
	}

	result, err := fetcher.CheckStep(req)
	if err != nil {
		zap.L().Error("step check", zap.String("checkType", req.CheckType), zap.String("project", req.Project), zap.Error(err))

		return nil, failure(http.StatusInternalServerError, "step check error")
	}

	return result, nil
}

// validateEvaluateRequest guards against SSRF by ensuring the policy source
// points to the configured GitLab instance and that ref/path are safe.
func (h *Handler) validateEvaluateRequest(req checker.EvaluateRequest) error {
	if !strings.HasPrefix(req.PolicySrc, h.config.GitLabBaseURL) {
		return errors.New("policySrc must point to the configured GitLab instance")
	}

	if strings.Contains(req.PolicyRef, "..") {
		return errors.New("policyRef must not contain path traversal sequences")
	}

	cleaned := path.Clean(req.PolicyPath)
	if strings.HasPrefix(cleaned, "..") {
		return errors.New("policyPath must not escape the repository root")
	}

	return nil
}

// fetchCourseCheckPolicy fetches the Rego policy from the course git repo.
func (h *Handler) fetchCourseCheckPolicy(ctx context.Context, req checker.EvaluateRequest) (string, *Failure) {
	token := ""
	if req.PolicyToken != nil {
		token = *req.PolicyToken
	}

	policy, err := checker.FetchCourseCheckPolicyContent(ctx, req.PolicySrc, req.PolicyRef, req.PolicyPath, token)
	if err != nil {
		zap.L().Error("policy fetch", zap.String("src", req.PolicySrc), zap.String("ref", req.PolicyRef), zap.String("path", req.PolicyPath), zap.Error(err))

		return "", failure(http.StatusInternalServerError, "failed to fetch policy")
	}

	return policy, nil
}

// fetchStudentGitLabProjectState fetches the student's GitLab project and MR
// state using the checker-service's GitLab token.
func (h *Handler) fetchStudentGitLabProjectState(req checker.EvaluateRequest) (*checker.GitLabState, *Failure) {
	fetcher, err := checker.NewFetcher(h.config.GitLabToken, h.config.GitLabBaseURL)
	if err != nil {
		zap.L().Error("gitlab client init", zap.Error(err))

		return nil, failure(http.StatusInternalServerError, "gitlab client error")
	}

	state, err := fetcher.Fetch(req.Project, req.Files)
	if err != nil {
		zap.L().Error("gitlab fetch", zap.String("project", req.Project), zap.Error(err))

		return nil, failure(http.StatusInternalServerError, "failed to fetch GitLab state")
	}

	return state, nil
}
