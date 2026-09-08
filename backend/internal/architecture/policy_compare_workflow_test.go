package architecture

import (
	"strings"
	"testing"
)

// An application workflow owns no data, so a candidate that introduces one
// with only new packages and an increased epoch is not an ownership loosening (#2697). The same
// owner over an existing package, or a new owner of an owning kind, stays
// rejected.
func TestCandidateWorkflowRequiresNewPackages(t *testing.T) {
	t.Parallel()

	base := &Policy{ModulePath: "example.test/project", Owners: []Owner{{ID: "module", Kind: "domain"}}}
	candidate := &Policy{
		ModulePath: "example.test/project",
		PolicyEpoch: 1,
		Owners:     []Owner{{ID: "module", Kind: "domain"}, {ID: "flow", Kind: "workflow"}},
		Packages: []Package{
			{
				Path: "workflows/flow", Owner: "flow", Role: "public",
				InternalTestRole: "workflow-decision-test", ExternalTestRole: "workflow-integration-test",
			},
			{
				Path: "workflows/flow/compose", Owner: "flow", Role: "compose",
				InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test",
			},
		},
	}
	created := map[string]struct{}{
		"example.test/project/workflows/flow":         {},
		"example.test/project/workflows/flow/compose": {},
	}

	if err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, created, map[string]struct{}{}, map[string]struct{}{}); err != nil {
		t.Fatalf("candidate workflow was rejected: %v", err)
	}

	partiallyCreated := map[string]struct{}{"example.test/project/workflows/flow": {}}
	err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, partiallyCreated, map[string]struct{}{}, map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "owner flow with kind workflow was added") {
		t.Fatalf("a workflow over an existing package bypassed the owner guard: %v", err)
	}

	domain := &Policy{
		ModulePath: "example.test/project",
		Owners:     []Owner{{ID: "module", Kind: "domain"}, {ID: "flow", Kind: "domain"}},
		Packages:   candidate.Packages,
	}
	err = comparePolicyStrictness(base, domain, map[string]struct{}{}, created, map[string]struct{}{}, map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "owner flow with kind domain was added") {
		t.Fatalf("a new owning kind bypassed the owner guard: %v", err)
	}
}
