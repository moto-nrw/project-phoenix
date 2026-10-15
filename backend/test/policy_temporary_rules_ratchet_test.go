package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestPolicyTemporaryRulesRatchet keeps temporary permissions out of
// backend/architecture/policy.json (ADR 0057, #4070).
//
// A rule states the target. Every allowance that exists only until a carrier
// lands is an exact backend/architecture/legacy.jsonl key with an open carrier
// issue, where `scripts/backend-architecture.sh check` counts it and pull
// requests can only remove it. A rule that calls itself temporary, or that
// carries an `issue` field, is debt the ratchet does not see.
//
// Until #4070 this test counted rules by phrase and let the count fall. The
// phrases missed most of them: on 4072c83a48 it saw 2 and 49 of the 180 rules
// whose description declared them temporary. #4070 converted every such rule
// into exact keys, so the test is structural now: any rule with a transition
// marker or an `issue` field fails, and there is no seed to raise. The
// former registers policyTempRulesTotal, policyTempRulesCompatTotal and
// policyTempRulesFamilies are retired under ADR 0057; the ratchet budget
// comparison of `scripts/backend-architecture.sh check` reads this sentence
// as the record of their retirement.
//
// The evaluator rejects the same rules when it loads the policy
// (internal/architecture/target_rule.go). This test reads the policy file
// without the evaluator, so it also names an `issue` field the evaluator would
// only reject from the target-only epoch on. The markers below must stay
// identical to the evaluator's TransitionMarkers;
// TestStructuralRatchetUsesTheEvaluatorMarkers in internal/architecture checks
// that.
//
// Failure mode: remove the rule and record each import it allowed as an exact
// legacy.jsonl key with its open carrier issue (backend/architecture/README.md,
// "Converting temporary permissions to exact debt"). If the rule is target,
// describe it without a transition marker.
func TestPolicyTemporaryRulesRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	violations, err := policyTemporaryRules(filepath.Join(backendRoot, "architecture", "policy.json"))
	if err != nil {
		t.Fatalf("policy scan failed: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("policy.json carries %d temporary rule(s); a temporary allowance is an exact legacy.jsonl key with an open carrier, never a rule (ADR 0057):\n\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// policyTransitionMarkers mirror TransitionMarkers in
// internal/architecture/target_rule.go: the phrases by which a description
// declares its rule temporary.
var policyTransitionMarkers = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{name: "convert to exact debt", pattern: regexp.MustCompile(`(?i)\bconvert\b[^.]*\bto exact debt\b`)},
	{name: "compatibility permission", pattern: regexp.MustCompile(`(?i)\bcompatibility permissions?\b`)},
	{name: "compatibility binding", pattern: regexp.MustCompile(`(?i)\bcompatibility bindings?\b`)},
	{name: "compatibility adapter", pattern: regexp.MustCompile(`(?i)\bcompatibility adapters?\b`)},
	{name: "temporary", pattern: regexp.MustCompile(`(?i)\btemporar(?:y|ily)\b`)},
	{name: "until #", pattern: regexp.MustCompile(`(?i)\buntil #\d+`)},
	{name: "tracked by #", pattern: regexp.MustCompile(`(?i)\btracked by #\d+`)},
}

// policyTemporaryRule is the shape this test needs. policy.json is large, so
// it is decoded into these fields; encoding/json drops every other one.
type policyTemporaryRule struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Issue       json.RawMessage `json:"issue"`
}

// policyTemporaryRules returns one line per rule that carries an issue field
// or a transition marker. It only reads the policy.
func policyTemporaryRules(path string) ([]string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- test reads a repo-local policy file
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var policy struct {
		Rules []policyTemporaryRule `json:"rules"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&policy); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(policy.Rules) == 0 {
		return nil, fmt.Errorf("no rules decoded from %s", path)
	}
	var violations []string
	for _, rule := range policy.Rules {
		var reasons []string
		if rule.Issue != nil {
			reasons = append(reasons, "issue field")
		}
		for _, marker := range policyTransitionMarkers {
			if marker.pattern.MatchString(rule.Description) {
				reasons = append(reasons, "marker \""+marker.name+"\"")
			}
		}
		if len(reasons) > 0 {
			violations = append(violations, fmt.Sprintf("  %s: %s", rule.ID, strings.Join(reasons, ", ")))
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func TestPolicyTemporaryRulesDetectsMarkersAndIssues(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	policy := `{"rules":[
		{"id":"target","description":"Alpha decides through Beta's public capability."},
		{"id":"contemporary","description":"The contemporary roster is compatible with the adapter."},
		{"id":"issue","description":"Owner reach.","issue":"https://github.com/moto-nrw/project-phoenix/issues/2729"},
		{"id":"convert","description":"Compatibility permission: convert it to exact debt once the package exists at a base SHA."},
		{"id":"binding","description":"Compatibility binding of the retained coordinator."},
		{"id":"adapter","description":"The live-group compatibility adapters return the public capability."},
		{"id":"temporary","description":"Temporarily reached by the retained composition."},
		{"id":"until","description":"Joins the tenant transaction until #2675."},
		{"id":"tracked","description":"Owner HTTP adapter, tracked by #2706."}
	]}`
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := policyTemporaryRules(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`  adapter: marker "compatibility adapter"`,
		`  binding: marker "compatibility binding"`,
		`  convert: marker "convert to exact debt", marker "compatibility permission"`,
		`  issue: issue field`,
		`  temporary: marker "temporary"`,
		`  tracked: marker "tracked by #"`,
		`  until: marker "until #"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("temporary rules = %q, want %q", got, want)
	}
}
