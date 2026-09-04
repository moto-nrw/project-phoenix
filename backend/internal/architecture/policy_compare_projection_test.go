package architecture

import (
	"strings"
	"testing"
)

func TestCandidateProjectionRequiresNewPackage(t *testing.T) {
	t.Parallel()

	base := &Policy{ModulePath: "example.test/project", Owners: []Owner{{ID: "module", Kind: "domain"}}}
	candidate := &Policy{
		ModulePath: "example.test/project",
		Owners:     []Owner{{ID: "module", Kind: "domain"}, {ID: "view", Kind: "projection"}},
		Packages: []Package{{
			Path: "projection", Owner: "view", Role: "postgres",
			InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
		}},
		ReadProjections: []ReadProjection{{
			ID: "view", Package: "projection", DataObjects: []string{"module.records"}, TenantSafe: true,
		}},
	}
	created := map[string]struct{}{"example.test/project/projection": {}}

	if err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, created, map[string]struct{}{}); err != nil {
		t.Fatalf("candidate projection was rejected: %v", err)
	}

	err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "owner view with kind projection was added") || !strings.Contains(err.Error(), "new tenant-safe read projection grant") {
		t.Fatalf("existing package bypassed the projection guard: %v", err)
	}
}
