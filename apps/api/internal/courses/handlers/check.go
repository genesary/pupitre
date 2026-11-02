// Package handlers implements the course-service HTTP API endpoints.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"gopkg.in/yaml.v3"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// checkSpec is the structure of check.yaml in the course git repo.
// Placed in the same directory as the module content file.
type checkSpec struct {
	Provider string   `yaml:"provider"`
	Project  string   `yaml:"project"` // template, e.g. "e-learning/{{ .Username }}"
	Files    []string `yaml:"files"`
	Policy   string   `yaml:"policy"` // relative path to .rego file
}

// EvaluateRequest is the payload sent to checker-service's /evaluate route.
// Raw Rego text is no longer included: checker-service fetches the policy
// directly from the course git repository using the reference fields below.
type EvaluateRequest struct {
	Username    string   `json:"username"`
	Project     string   `json:"project"`
	Files       []string `json:"files"`
	PolicySrc   string   `json:"policySrc"`             // git repository URL
	PolicyRef   string   `json:"policyRef"`             // branch / tag / commit
	PolicyPath  string   `json:"policyPath"`            // relative path to the .rego file
	PolicyToken *string  `json:"policyToken,omitempty"` // optional git authentication token
}

// CheckResponse mirrors checker.EvaluateResponse.
type CheckResponse struct {
	Allow      bool     `json:"allow"`
	Violations []string `json:"violations"`
}

// CheckModule runs the OPA policy check for a lab module against the
// learner's GitLab project and stores the result.
// POST /api/courses/{slug}/modules/{index}/check.
func (s *State) CheckModule(writer http.ResponseWriter, req *http.Request) {
	courseSlug := param(req, "slug")
	indexStr := param(req, "index")
	claims := s.claims(req)

	course, found := s.course(writer, req, courseSlug)
	if !found {
		return
	}

	mod, idx, success := s.resolveLabModule(writer, req, course, indexStr)
	if !success {
		return
	}

	spec, success := s.fetchCheckSpec(req.Context(), writer, mod)
	if !success {
		return
	}

	username, identified := s.labUsername(writer, req)
	if !identified {
		return
	}

	// display name for traceability: prefer email over opaque UUID
	displayName := claims.Email
	if displayName == "" {
		displayName = username
	}

	token := s.tokenForRepo(mod.Src)

	// Resolve username template in project path
	project := strings.ReplaceAll(spec.Project, "{{ .Username }}", username)

	var policyToken *string
	if token != "" {
		policyToken = &token
	}

	result, success := s.callChecker(writer, req, EvaluateRequest{
		Username:    username,
		Project:     project,
		Files:       spec.Files,
		PolicySrc:   mod.Src,
		PolicyRef:   mod.Ref,
		PolicyPath:  path.Join(path.Dir(mod.Path), spec.Policy),
		PolicyToken: policyToken,
	})
	if !success {
		return
	}

	s.storeLabCheck(req.Context(), displayName, courseSlug, idx, mod.Name, result, true)

	s.JSON(writer, http.StatusOK, result)
}

// resolveLabModule looks up the module at indexStr among the modules
// visible to the requester and confirms it is a checkable lab.
func (s *State) resolveLabModule(
	writer http.ResponseWriter, req *http.Request, course *content.Course, indexStr string,
) (content.Module, int, bool) {
	modules := s.visibleModules(course, req)

	idx, err := strconv.Atoi(indexStr)
	if err != nil || idx < 0 || idx >= len(modules) {
		s.Error(writer, http.StatusNotFound, moduleNotFoundMessage)

		return content.Module{}, 0, false
	}

	mod := modules[idx]

	if mod.Type != moduleTypeLab {
		s.Error(writer, http.StatusBadRequest, "Module is not a lab")

		return content.Module{}, 0, false
	}

	if !mod.HasGitContent() {
		s.Error(writer, http.StatusBadRequest, "Lab module has no git content configured")

		return content.Module{}, 0, false
	}

	return mod, idx, true
}

// fetchCheckSpec fetches and parses check.yaml from the same git directory
// as the module content. The Rego policy itself is not fetched here:
// checker-service retrieves it from git via the reference in EvaluateRequest.
func (s *State) fetchCheckSpec(ctx context.Context, writer http.ResponseWriter, mod content.Module) (checkSpec, bool) {
	token := s.tokenForRepo(mod.Src)
	checkDir := path.Dir(mod.Path)

	specData, err := s.GitCache.FetchModuleContent(ctx, mod.Src, mod.Ref, path.Join(checkDir, "check.yaml"), token)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "No check.yaml found for this lab")

		return checkSpec{}, false
	}

	var spec checkSpec

	err = yaml.Unmarshal(specData, &spec)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Failed to parse check.yaml")

		return checkSpec{}, false
	}

	return spec, true
}

// labUsername is the learner's GitLab username: the login the identity
// provider gave them, not the profile username, which they can change
// (docs/specs/190-backend-defects.md B11). It writes the error response and
// returns ok=false when there is none.
func (s *State) labUsername(writer http.ResponseWriter, req *http.Request) (string, bool) {
	name, err := s.users().LabIdentity(req.Context(), s.claims(req).Subject)
	if err != nil {
		zap.L().Error("load lab identity", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return "", false
	}

	if name == "" {
		s.Error(writer, http.StatusForbidden, "GitLab labs need a sign-in through the identity provider")

		return "", false
	}

	return name, true
}

// callChecker sends req to checker-service's /evaluate route and returns
// the decoded response, writing an HTTP error and returning ok=false on
// any failure.
func (s *State) callChecker(writer http.ResponseWriter, req *http.Request, body EvaluateRequest) (CheckResponse, bool) {
	return s.callCheckerRoute(writer, req, "/evaluate", body)
}

// storeLabCheck persists a lab check result for later review, logging a
// warning on failure without interrupting the response to the client.
func (s *State) storeLabCheck(
	ctx context.Context, username, courseSlug string, moduleIndex int, moduleName string, result CheckResponse, verified bool,
) {
	if s.Repos.LabChecks == nil {
		return
	}

	violations := result.Violations
	if violations == nil {
		violations = []string{}
	}

	err := s.Repos.LabChecks.Create(ctx, &models.LabCheck{
		Username:    username,
		CourseSlug:  courseSlug,
		ModuleIndex: moduleIndex,
		ModuleName:  moduleName,
		Allow:       result.Allow,
		Violations:  violations,
		Verified:    verified,
	})
	if err != nil {
		zap.L().Warn("failed to store lab check", zap.Error(err))
	}
}

// StepCheckRequest is sent to checker-service's /check-step route.
type StepCheckRequest struct {
	Username    string         `json:"username"`
	Project     string         `json:"project"`
	CheckType   string         `json:"checkType"`
	CheckParams map[string]any `json:"checkParams"`
}

// CheckModuleStep runs a single named step check for a lab module.
// POST /api/courses/{slug}/modules/{index}/steps/{stepIndex}/check.
func (s *State) CheckModuleStep(writer http.ResponseWriter, req *http.Request) { //nolint:gocyclo,cyclop,funlen // single-responsibility handler with multiple early-exit guard clauses
	courseSlug := param(req, "slug")
	indexStr := param(req, "index")
	stepIndexStr := param(req, "stepIndex")

	course, found := s.course(writer, req, courseSlug)
	if !found {
		return
	}

	mod, _, success := s.resolveLabModule(writer, req, course, indexStr)
	if !success {
		return
	}

	if mod.CheckProvider != checkProviderGitLab || len(mod.Steps) == 0 {
		s.Error(writer, http.StatusBadRequest, "Module does not have GitLab step checks")

		return
	}

	stepIdx, err := strconv.Atoi(stepIndexStr)
	if err != nil || stepIdx < 0 || stepIdx >= len(mod.Steps) {
		s.Error(writer, http.StatusNotFound, "Step not found")

		return
	}

	username, identified := s.labUsername(writer, req)
	if !identified {
		return
	}

	step := mod.Steps[stepIdx]

	// Resolve {{ .Username }} template in every string value of checkParams.
	resolved := make(map[string]any, len(step.CheckParams))
	for k, v := range step.CheckParams {
		if sv, ok := v.(string); ok {
			resolved[k] = strings.ReplaceAll(sv, "{{ .Username }}", username)
		} else {
			resolved[k] = v
		}
	}

	// Derive project from checkParams if present; otherwise skip.
	project, ok := resolved["project"].(string)
	if !ok || project == "" {
		s.Error(writer, http.StatusBadRequest, "Step checkParams missing project")

		return
	}

	result, success := s.callCheckerStep(writer, req, StepCheckRequest{
		Username:    username,
		Project:     project,
		CheckType:   step.CheckType,
		CheckParams: resolved,
	})
	if !success {
		return
	}

	s.JSON(writer, http.StatusOK, result)
}

// callCheckerStep sends a step check request to checker-service's /check-step
// route and returns the decoded response.
func (s *State) callCheckerStep(writer http.ResponseWriter, req *http.Request, body StepCheckRequest) (CheckResponse, bool) {
	return s.callCheckerRoute(writer, req, "/check-step", body)
}

// callCheckerRoute runs one check through the checker and returns its
// result. It writes an HTTP error and returns ok=false on any failure: the
// status and message a CheckerError carries, or a 500 when the checker could
// not be reached at all.
func (s *State) callCheckerRoute(writer http.ResponseWriter, req *http.Request, route string, body any) (CheckResponse, bool) {
	var (
		result CheckResponse
		err    error
	)

	switch typed := body.(type) {
	case EvaluateRequest:
		result, err = s.labChecker().Evaluate(req.Context(), typed)
	case StepCheckRequest:
		result, err = s.labChecker().CheckStep(req.Context(), typed)
	default:
		err = &CheckerError{Status: http.StatusInternalServerError, Message: buildCheckerRequestFailed}
	}

	if err == nil {
		return result, true
	}

	if checkerErr, ok := errors.AsType[*CheckerError](err); ok {
		if checkerErr.RetryAfter > 0 {
			writer.Header().Set("Retry-After", strconv.Itoa(checkerErr.RetryAfter))
		}

		s.Error(writer, checkerErr.Status, checkerErr.Message)

		return CheckResponse{}, false
	}

	zap.L().Error("checker-service call failed", zap.String("route", route), zap.Error(err))
	s.Error(writer, http.StatusInternalServerError, "error when reaching for checker service")

	return CheckResponse{}, false
}

// LabChecker runs lab checks against a learner's GitLab project. In the
// monolith it calls the checker package directly; the HTTP implementation
// below serves the standalone course-service binary until it is removed
// (docs/specs/107-backend-labs-and-checker.md).
type LabChecker interface {
	// Evaluate runs a lab's OPA policy.
	Evaluate(ctx context.Context, req EvaluateRequest) (CheckResponse, error)
	// CheckStep runs one named step check.
	CheckStep(ctx context.Context, req StepCheckRequest) (CheckResponse, error)
}

// buildCheckerRequestFailed is the message of a check request that could
// not be built.
const buildCheckerRequestFailed = "Failed to build checker request"

// CheckerError is a check that the checker refused or could not complete,
// with the status and message the learner receives.
type CheckerError struct {
	Status  int
	Message string
	// RetryAfter, in seconds, is sent as Retry-After when the learner is
	// over the lab check limit.
	RetryAfter int
}

// Error implements error.
func (e *CheckerError) Error() string {
	return fmt.Sprintf("checker: %d %s", e.Status, e.Message)
}

// checkerRejected is the error of a check the checker answered with an
// error: relayed as a 502 carrying the checker's message.
func checkerRejected(message string) *CheckerError {
	if message == "" {
		message = "checker-service error"
	}

	return &CheckerError{Status: http.StatusBadGateway, Message: message}
}

// labChecker returns the checker wired in by the monolith; a State built
// without one answers every check as if the checker were unreachable.
func (s *State) labChecker() LabChecker { //nolint:ireturn // the wired checker, or none
	if s.Checker != nil {
		return s.Checker
	}

	return noChecker{}
}

// errNoChecker is returned by a State that was not wired to the checker.
var errNoChecker = errors.New("checker not wired")

// noChecker answers every check with errNoChecker.
type noChecker struct{}

// Evaluate implements LabChecker.
func (noChecker) Evaluate(context.Context, EvaluateRequest) (CheckResponse, error) {
	return CheckResponse{}, errNoChecker
}

// CheckStep implements LabChecker.
func (noChecker) CheckStep(context.Context, StepCheckRequest) (CheckResponse, error) {
	return CheckResponse{}, errNoChecker
}

// RecordLocalCheck persists the result of a Tauri-side local check
// (podman, etc.) without re-running any verification server-side.
// POST /api/courses/{slug}/modules/{index}/record.
func (s *State) RecordLocalCheck(writer http.ResponseWriter, req *http.Request) {
	courseSlug := param(req, "slug")
	indexStr := param(req, "index")
	claims := s.claims(req)

	course, found := s.course(writer, req, courseSlug)
	if !found {
		return
	}

	idx, err := strconv.Atoi(indexStr)
	if err != nil || idx < 0 || idx >= len(course.Modules) {
		s.Error(writer, http.StatusBadRequest, "Invalid module index")

		return
	}

	mod := course.Modules[idx]

	var result CheckResponse

	err = json.NewDecoder(req.Body).Decode(&result)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid body")

		return
	}

	displayName := claims.Email
	if displayName == "" {
		displayName = "unknown"
	}

	s.storeLabCheck(req.Context(), displayName, courseSlug, idx, mod.Name, result, false)
	s.JSON(writer, http.StatusOK, result)
}
