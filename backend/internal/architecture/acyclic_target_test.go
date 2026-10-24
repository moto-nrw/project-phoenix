package architecture

import (
	"strings"
	"testing"
)

// acyclicFixture decodes the target-only fixture at epoch after mutate.
func acyclicFixture(t *testing.T, epoch int, mutate func(map[string]any)) error {
	t.Helper()
	document := mutatePolicy(t, readFile(t, fixturePath(t, "target-only-rules.json")), func(document map[string]any) {
		document["policy_epoch"] = epoch
		mutate(document)
	})
	_, err := DecodePolicy(strings.NewReader(document))
	return err
}

// closeAlphaBetaCycle adds the reverse of alpha.application.beta-public.
var closeAlphaBetaCycle = addFixtureRule(map[string]any{
	"id": "beta.application.alpha-public", "source_owner": "beta", "source_role": "application",
	"target_owner": "alpha", "target_role": "public",
})

func TestAcyclicTargetAcceptsADAG(t *testing.T) {
	t.Parallel()
	if err := acyclicFixture(t, acyclicTargetEpoch, func(map[string]any) {}); err != nil {
		t.Fatalf("acyclic target rejected: %v", err)
	}
}

func TestAcyclicTargetRejectsARuleThatClosesACycle(t *testing.T) {
	t.Parallel()
	err := acyclicFixture(t, acyclicTargetEpoch, closeAlphaBetaCycle)
	if err == nil {
		t.Fatal("a target graph with alpha <-> beta was accepted")
	}
	for _, want := range []string{"1 cyclic component", "{alpha, beta}", "alpha -> beta (alpha.application.beta-public, alpha.http.beta-session)", "beta -> alpha (beta.application.alpha-public)", "ADR 0058"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err, want)
		}
	}
}

func TestAcyclicTargetIgnoresTestScopes(t *testing.T) {
	t.Parallel()
	err := acyclicFixture(t, acyclicTargetEpoch, addFixtureRule(map[string]any{
		"id": "beta.module-internal-test.alpha-public", "scopes": []any{"internal_test"}, "source_owner": "beta",
		"source_role": "module-internal-test", "target_owner": "alpha", "target_role": "public",
	}))
	if err != nil {
		t.Fatalf("a test-scope reverse edge closed a target cycle: %v", err)
	}
}

// An epoch before the check stays loadable as an immutable PR base.
func TestAcyclicTargetIsEpochGated(t *testing.T) {
	t.Parallel()
	if err := acyclicFixture(t, acyclicTargetEpoch-1, closeAlphaBetaCycle); err != nil {
		t.Fatalf("an epoch-%d base with a cycle was rejected: %v", acyclicTargetEpoch-1, err)
	}
}
