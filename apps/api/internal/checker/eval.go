// Package checker evaluates lab submissions against GitLab state via OPA.
package checker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

// opaEvalTimeout is the dedicated budget for a single OPA evaluation,
// independent of the HTTP request context so a slow policy cannot stall
// the handler goroutine after the response deadline has passed.
const opaEvalTimeout = 10 * time.Second

// maxInputBytes is the maximum size of the JSON-serialised OPA input
// (the GitLab state struct). Policies operating on larger inputs are
// rejected before evaluation to prevent CPU/memory exhaustion.
const maxInputBytes = 5 * 1024 * 1024 // 5 MB

// Evaluate runs the given Rego policy against the GitLab state and returns
// the resulting allow/violations verdict.
func Evaluate(ctx context.Context, policy string, state *GitLabState) (*EvaluateResponse, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}

	if len(stateJSON) > maxInputBytes {
		return nil, fmt.Errorf("OPA input too large: %d bytes exceeds limit of %d", len(stateJSON), maxInputBytes)
	}

	var input map[string]any

	err = json.Unmarshal(stateJSON, &input)
	if err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}

	query, err := rego.New(
		rego.Query("data.checker.lab"),
		rego.Module("check.rego", policy),
		rego.Input(input),
		rego.Capabilities(restrictedCapabilities()),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego prepare: %w", err)
	}

	evalCtx, cancel := context.WithTimeout(ctx, opaEvalTimeout)
	defer cancel()

	resultSet, err := query.Eval(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("rego eval: %w", err)
	}

	resp := &EvaluateResponse{}

	expr, found := firstExpression(resultSet)
	if !found {
		return resp, nil
	}

	res, ok := expr.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected rego output type")
	}

	if allow, ok := res["allow"].(bool); ok {
		resp.Allow = allow
	}

	resp.Violations = extractViolations(res["violations"])

	return resp, nil
}

// restrictedCapabilities returns an OPA Capabilities set that excludes
// network builtins (http.send, net.*) and opa.runtime, preventing SSRF
// and information disclosure from within evaluated policies.
func restrictedCapabilities() *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion()

	safe := make([]*ast.Builtin, 0, len(caps.Builtins))

	for _, builtin := range caps.Builtins {
		if strings.HasPrefix(builtin.Name, "http.") ||
			strings.HasPrefix(builtin.Name, "net.") ||
			builtin.Name == "opa.runtime" {
			continue
		}

		safe = append(safe, builtin)
	}

	caps.Builtins = safe

	return caps
}

// firstExpression returns the first expression value of the first rego
// result, if present.
func firstExpression(resultSet rego.ResultSet) (any, bool) {
	if len(resultSet) == 0 || len(resultSet[0].Expressions) == 0 {
		return nil, false
	}

	return resultSet[0].Expressions[0].Value, true
}

// extractViolations converts a raw violations value into a []string,
// skipping any non-string entries.
func extractViolations(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}

	violations := make([]string, 0, len(list))

	for _, msg := range list {
		if s, ok := msg.(string); ok {
			violations = append(violations, s)
		}
	}

	return violations
}
