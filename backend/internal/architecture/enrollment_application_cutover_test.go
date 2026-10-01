package architecture

import (
	"path/filepath"
	"testing"
)

// enrollmentApplicationCutoverPolicies returns a base that classifies the
// retained package exactly and a reviewed candidate one epoch later that
// adds the module application package at the same point.
func enrollmentApplicationCutoverPolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 30
	retained := enrollmentApplicationPoint
	retained.Path = enrollmentRetainedApplicationPath
	base.Packages = []Package{
		retained,
		{Path: "modules/enrollment/compose", Owner: "enrollment", Role: "compose", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
	}
	base.Rules = []Rule{}
	base.ReadProjections = []ReadProjection{}
	base.DataObjects = []DataObject{}
	base.LegacyComposition = []LegacyReference{}
	base.Relocations = nil
	base.ExternalPackages = []ExternalPackage{}
	candidate := *base
	candidate.PolicyEpoch++
	moved := enrollmentApplicationPoint
	moved.Path = enrollmentModuleApplicationPath
	candidate.Packages = append(append([]Package{}, base.Packages...), moved)
	return base, &candidate
}

func enrollmentCutoverPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestEnrollmentApplicationCutoverLetsCompositionConstructApplication(t *testing.T) {
	t.Parallel()
	base, candidate := enrollmentApplicationCutoverPolicies(t)
	compose := Package{Owner: "enrollment", Role: "compose", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	application := enrollmentCutoverPoint("enrollment", "application")
	if !enrollmentApplicationCutoverPermission(base, candidate, ScopeProduction, compose, application) {
		t.Fatal("Enrollment composition cannot construct its application services")
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if enrollmentApplicationCutoverPermission(base, candidate, scope, compose, application) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
	for _, role := range []string{"domain", "port", "adapter", "test-support"} {
		if enrollmentApplicationCutoverPermission(base, candidate, ScopeProduction, compose, enrollmentCutoverPoint("enrollment", role)) {
			t.Fatalf("composition reached enrollment/%s", role)
		}
	}
	if enrollmentApplicationCutoverPermission(base, candidate, ScopeProduction, compose, enrollmentCutoverPoint("care-plan", "application")) {
		t.Fatal("composition reached a foreign application")
	}
}

func TestEnrollmentApplicationCutoverRefusesOtherSources(t *testing.T) {
	t.Parallel()
	base, candidate := enrollmentApplicationCutoverPolicies(t)
	application := enrollmentCutoverPoint("enrollment", "application")
	for _, source := range []Package{
		enrollmentCutoverPoint("enrollment", "public"),
		enrollmentCutoverPoint("enrollment", "postgres"),
		enrollmentCutoverPoint("care-plan", "compose"),
		enrollmentCutoverPoint("parent-portal", "compose"),
		enrollmentCutoverPoint("inbound-enrollment", "http"),
	} {
		if enrollmentApplicationCutoverPermission(base, candidate, ScopeProduction, source, application) {
			t.Fatalf("%s/%s reached enrollment/application", source.Owner, source.Role)
		}
	}
}

func TestEnrollmentApplicationCutoverRequiresAnchor(t *testing.T) {
	t.Parallel()
	compose := enrollmentCutoverPoint("enrollment", "compose")
	application := enrollmentCutoverPoint("enrollment", "application")
	cases := map[string]func(base, candidate *Policy){
		"same epoch": func(_, candidate *Policy) { candidate.PolicyEpoch-- },
		"retained package missing": func(base, _ *Policy) {
			base.Packages = base.Packages[1:]
		},
		"retained package reclassified": func(base, _ *Policy) {
			base.Packages = append([]Package{}, base.Packages...)
			base.Packages[0].Role = "adapter"
		},
		"module package missing": func(base, candidate *Policy) {
			candidate.Packages = base.Packages
		},
		"module package reclassified": func(_, candidate *Policy) {
			candidate.Packages[len(candidate.Packages)-1].ExternalTestRole = "adapter-test"
		},
		"foreign module": func(_, candidate *Policy) { candidate.ModulePath = "example.com/other" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := enrollmentApplicationCutoverPolicies(t)
			mutate(base, candidate)
			if enrollmentApplicationCutoverPermission(base, candidate, ScopeProduction, compose, application) {
				t.Fatal("exception granted without its anchor")
			}
		})
	}
}
