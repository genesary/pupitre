package checker

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// TestEvaluate_AllowNoViolations runs a trivially-passing policy.
func TestEvaluate_AllowNoViolations(t *testing.T) {
	t.Parallel()

	policy := `package checker.lab

default allow := false

allow if {
	input.project.name == "demo"
}
`
	state := &GitLabState{Project: &projectInfo{Name: "demo"}}

	resp, err := Evaluate(context.Background(), policy, state)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !resp.Allow {
		t.Errorf("Allow = false, want true")
	}

	if len(resp.Violations) != 0 {
		t.Errorf("Violations = %v, want none", resp.Violations)
	}
}

// TestEvaluate_DenyWithViolations checks that string violations flow through.
func TestEvaluate_DenyWithViolations(t *testing.T) {
	t.Parallel()

	policy := `package checker.lab

default allow := false

violations contains msg if {
	input.mergedMrCount == 0
	msg := "no merged MR"
}

violations contains msg if {
	not input.project.name
	msg := "no open MR"
}
`
	state := &GitLabState{}

	resp, err := Evaluate(context.Background(), policy, state)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if resp.Allow {
		t.Errorf("Allow = true, want false")
	}

	if len(resp.Violations) != 2 {
		t.Fatalf("Violations = %v, want 2", resp.Violations)
	}

	joined := strings.Join(resp.Violations, "|")
	if !strings.Contains(joined, "no merged MR") || !strings.Contains(joined, "no open MR") {
		t.Errorf("unexpected violations: %v", resp.Violations)
	}
}

// TestEvaluate_NoResult returns an empty response when the rule set does not
// produce the queried document.
func TestEvaluate_NoResult(t *testing.T) {
	t.Parallel()

	// Policy defines a different package: data.checker.lab is undefined.
	policy := `package checker.other

allow := true
`

	resp, err := Evaluate(context.Background(), policy, &GitLabState{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if resp.Allow {
		t.Errorf("Allow = true, want false for undefined document")
	}

	if resp.Violations != nil {
		t.Errorf("Violations = %v, want nil", resp.Violations)
	}
}

// TestEvaluate_InvalidPolicy surfaces a compile error.
func TestEvaluate_InvalidPolicy(t *testing.T) {
	t.Parallel()

	_, err := Evaluate(context.Background(), "this is not rego", &GitLabState{})
	if err == nil {
		t.Fatal("expected error for invalid policy")
	}

	if !strings.Contains(err.Error(), "rego prepare") {
		t.Errorf("error = %v, want rego prepare failure", err)
	}
}

// TestEvaluate_NetworkBuiltinRejected confirms http.send is stripped from the
// capability set, so a policy that references it fails to compile.
func TestEvaluate_NetworkBuiltinRejected(t *testing.T) {
	t.Parallel()

	policy := `package checker.lab

allow if {
	http.send({"method": "get", "url": "http://169.254.169.254/"})
}
`

	_, err := Evaluate(context.Background(), policy, &GitLabState{})
	if err == nil {
		t.Fatal("expected compile error: http.send must be unavailable")
	}
}

// TestRestrictedCapabilities verifies the dangerous builtins are removed and
// ordinary ones are kept.
func TestRestrictedCapabilities(t *testing.T) {
	t.Parallel()

	caps := restrictedCapabilities()

	for _, b := range caps.Builtins {
		if strings.HasPrefix(b.Name, "http.") ||
			strings.HasPrefix(b.Name, "net.") ||
			b.Name == "opa.runtime" {
			t.Errorf("dangerous builtin %q not stripped", b.Name)
		}
	}

	var hasConcat bool

	for _, b := range caps.Builtins {
		if b.Name == "concat" {
			hasConcat = true

			break
		}
	}

	if !hasConcat {
		t.Error("expected safe builtin \"concat\" to remain available")
	}
}

// TestFirstExpression covers the empty and populated result sets.
func TestFirstExpression(t *testing.T) {
	t.Parallel()

	if _, ok := firstExpression(rego.ResultSet{}); ok {
		t.Error("empty result set should report not found")
	}

	if _, ok := firstExpression(rego.ResultSet{{}}); ok {
		t.Error("result with no expressions should report not found")
	}
}

// TestExtractViolations covers the type-filtering logic.
func TestExtractViolations(t *testing.T) {
	t.Parallel()

	if got := extractViolations(nil); got != nil {
		t.Errorf("nil input = %v, want nil", got)
	}

	if got := extractViolations("not a slice"); got != nil {
		t.Errorf("non-slice input = %v, want nil", got)
	}

	got := extractViolations([]any{"a", 42, "b", nil, "c"})
	want := []string{"a", "b", "c"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestEvaluate_NonMapOutput errors when data.checker.lab is not an object.
func TestEvaluate_NonMapOutput(t *testing.T) {
	t.Parallel()

	policy := "package checker\n\nlab := 42\n"

	_, err := Evaluate(context.Background(), policy, &GitLabState{})
	if err == nil || !strings.Contains(err.Error(), "unexpected rego output type") {
		t.Fatalf("want unexpected-output error, got %v", err)
	}
}

// TestEvaluate_InputTooLarge rejects an OPA input over the 5 MB cap before
// evaluation.
func TestEvaluate_InputTooLarge(t *testing.T) {
	t.Parallel()

	big := make([]byte, 6*1024*1024)
	for i := range big {
		big[i] = 'x'
	}

	state := &GitLabState{Files: map[string]string{"huge.txt": string(big)}}

	_, err := Evaluate(context.Background(), "package checker.lab\n", state)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too-large error, got %v", err)
	}
}
