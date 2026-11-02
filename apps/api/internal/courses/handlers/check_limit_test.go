package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// limitedChecker refuses every check as over the lab check limit.
type limitedChecker struct{}

// Evaluate implements LabChecker.
func (limitedChecker) Evaluate(context.Context, EvaluateRequest) (CheckResponse, error) {
	return CheckResponse{}, &CheckerError{Status: http.StatusTooManyRequests, Message: "Too many lab checks, retry in 60 s", RetryAfter: 60}
}

// CheckStep implements LabChecker.
func (limitedChecker) CheckStep(context.Context, StepCheckRequest) (CheckResponse, error) {
	return CheckResponse{}, &CheckerError{Status: http.StatusTooManyRequests, Message: "Too many lab checks, retry in 60 s", RetryAfter: 60}
}

// TestCallCheckerRoute_RateLimited checks that a learner over the lab check
// limit gets 429 with Retry-After (docs/specs/190-backend-defects.md B9).
func TestCallCheckerRoute_RateLimited(t *testing.T) {
	t.Parallel()

	s := &State{Checker: limitedChecker{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)

	_, ok := s.callCheckerRoute(rec, req, "/evaluate", EvaluateRequest{Username: "alice"})
	if ok {
		t.Fatal("a refused check was reported as done")
	}

	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" {
		t.Errorf("got %d with Retry-After %q, want 429 with Retry-After 60", rec.Code, rec.Header().Get("Retry-After"))
	}
}
