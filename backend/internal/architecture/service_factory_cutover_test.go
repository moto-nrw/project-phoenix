package architecture

import (
	"path/filepath"
	"testing"
)

// serviceFactoryCutoverPolicies returns a base that classifies the legacy
// composition exactly and a reviewed candidate one epoch later.
func serviceFactoryCutoverPolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 36
	retained := serviceFactoryPoint
	retained.Path = serviceFactoryRetainedPath
	base.Packages = []Package{retained}
	base.Rules = []Rule{}
	base.ReadProjections = []ReadProjection{}
	base.DataObjects = []DataObject{}
	base.LegacyComposition = []LegacyReference{}
	base.Relocations = nil
	base.ExternalPackages = []ExternalPackage{}
	candidate := *base
	candidate.PolicyEpoch++
	candidate.Packages = append([]Package{}, base.Packages...)
	return base, &candidate
}

func serviceFactoryCutoverPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestServiceFactoryCutoverGrantsExactlyTheListedPairs(t *testing.T) {
	t.Parallel()
	base, candidate := serviceFactoryCutoverPolicies(t)
	for grant := range serviceFactoryCutoverGrants {
		source := serviceFactoryCutoverPoint(grant[0].Owner, grant[0].Role)
		target := serviceFactoryCutoverPoint(grant[1].Owner, grant[1].Role)
		if !serviceFactoryCutoverPermission(base, candidate, ScopeProduction, source, target) {
			t.Fatalf("%s/%s cannot reach %s/%s", grant[0].Owner, grant[0].Role, grant[1].Owner, grant[1].Role)
		}
		for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
			if serviceFactoryCutoverPermission(base, candidate, scope, source, target) {
				t.Fatalf("production permission %s/%s -> %s/%s lent to %s", grant[0].Owner, grant[0].Role, grant[1].Owner, grant[1].Role, scope)
			}
		}
	}
}

func TestServiceFactoryCutoverRefusesOtherPairs(t *testing.T) {
	t.Parallel()
	base, candidate := serviceFactoryCutoverPolicies(t)
	for _, pair := range [][2]Package{
		{serviceFactoryCutoverPoint("root-composition", "compose"), serviceFactoryCutoverPoint("document-rendering", "contract")},
		{serviceFactoryCutoverPoint("legacy-composition", "compose"), serviceFactoryCutoverPoint("document-rendering", "application")},
		{serviceFactoryCutoverPoint("legacy-composition", "compose"), serviceFactoryCutoverPoint("audit-platform", "domain")},
		{serviceFactoryCutoverPoint("care-plan", "compose"), serviceFactoryCutoverPoint("audit-platform", "application")},
		{serviceFactoryCutoverPoint("audit-platform", "compose"), serviceFactoryCutoverPoint("audit-platform", "postgres")},
		{serviceFactoryCutoverPoint("legacy-composition", "compose"), serviceFactoryCutoverPoint("tenant-runtime", "public")},
	} {
		if serviceFactoryCutoverPermission(base, candidate, ScopeProduction, pair[0], pair[1]) {
			t.Fatalf("%s/%s reached %s/%s", pair[0].Owner, pair[0].Role, pair[1].Owner, pair[1].Role)
		}
	}
}

func TestServiceFactoryCutoverNeedsTheAnchor(t *testing.T) {
	t.Parallel()
	source := serviceFactoryCutoverPoint("legacy-composition", "compose")
	target := serviceFactoryCutoverPoint("document-rendering", "contract")

	base, candidate := serviceFactoryCutoverPolicies(t)
	candidate.PolicyEpoch = base.PolicyEpoch
	if serviceFactoryCutoverPermission(base, candidate, ScopeProduction, source, target) {
		t.Fatal("granted without a raised epoch")
	}

	base, candidate = serviceFactoryCutoverPolicies(t)
	base.Packages = nil
	if serviceFactoryCutoverPermission(base, candidate, ScopeProduction, source, target) {
		t.Fatal("granted without the legacy composition in the base")
	}

	base, candidate = serviceFactoryCutoverPolicies(t)
	base.Packages[0].Role = "application"
	if serviceFactoryCutoverPermission(base, candidate, ScopeProduction, source, target) {
		t.Fatal("granted with the legacy composition reclassified")
	}

	base, candidate = serviceFactoryCutoverPolicies(t)
	base.ModulePath = "example.test/other"
	candidate.ModulePath = base.ModulePath
	if serviceFactoryCutoverPermission(base, candidate, ScopeProduction, source, target) {
		t.Fatal("granted for a foreign module path")
	}
}
