package architecture

import (
	"path/filepath"
	"slices"
	"testing"
)

// timeTrackingCutoverPolicies returns a base that still classifies the
// retired time-tracking package and a reviewed candidate that retired it.
// The historical permissions are explicit so the fixture keeps exercising
// the retired package after the repository's own policy removed it.
func timeTrackingCutoverPolicies(t *testing.T, historical ...Rule) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 38
	base.Packages = []Package{
		timeTrackingRetiredPackage,
		{Path: "modules/workforce", Owner: "workforce", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce/compose", Owner: "workforce", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce/internal/application", Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce/adapters/timerecords", Owner: "workforce", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "modules/studentpresence", Owner: "student-presence", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/mealplan", Owner: "meal-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
	}
	base.Rules = historical
	base.ReadProjections = []ReadProjection{}
	base.DataObjects = []DataObject{}
	base.LegacyComposition = []LegacyReference{}
	base.Relocations = nil
	base.ExternalPackages = []ExternalPackage{}
	candidate := *base
	candidate.PolicyEpoch++
	candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool { return p.Path == timeTrackingLegacyPath })
	return base, &candidate
}

func timeTrackingPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestTimeTrackingCutoverKeepsTheRetiredCodesReach(t *testing.T) {
	t.Parallel()
	historical := Rule{ID: "historical.time-tracking-reach", Scopes: []string{"production"}, SourceOwner: "workforce", SourceRole: "adapter", TargetOwner: "student-presence", TargetRole: "public"}
	base, candidate := timeTrackingCutoverPolicies(t, historical)
	presence := timeTrackingPoint("student-presence", "public")
	for _, receiver := range []Package{
		{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Owner: "workforce", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
	} {
		if !timeTrackingCutoverPermission(base, candidate, ScopeProduction, receiver, presence) {
			t.Fatalf("%s/%s lost the reach the retired code had", receiver.Owner, receiver.Role)
		}
		for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
			if timeTrackingCutoverPermission(base, candidate, scope, receiver, presence) {
				t.Fatalf("production reach of %s/%s lent to %s", receiver.Owner, receiver.Role, scope)
			}
		}
	}
	for _, other := range []Package{
		timeTrackingPoint("workforce", "public"),
		timeTrackingPoint("workforce", "http"),
		timeTrackingPoint("workforce", "postgres"),
		timeTrackingPoint("workforce", "adapter"),
		timeTrackingPoint("meal-plan", "application"),
	} {
		if timeTrackingCutoverPermission(base, candidate, ScopeProduction, other, presence) {
			t.Fatalf("%s/%s received reach it never held", other.Owner, other.Role)
		}
	}
	receiver := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if timeTrackingCutoverPermission(base, candidate, ScopeProduction, receiver, timeTrackingPoint("meal-plan", "public")) {
		t.Fatal("receiving role gained a target the retired code never had")
	}
}

func TestTimeTrackingCutoverMovesTheSuites(t *testing.T) {
	t.Parallel()
	historical := Rule{ID: "historical.time-tracking-suite", Scopes: []string{"external_test"}, SourceOwner: "workforce", SourceRole: "adapter-test", TargetOwner: "student-presence", TargetRole: "public"}
	base, candidate := timeTrackingCutoverPolicies(t, historical)
	presence := timeTrackingPoint("student-presence", "public")
	suite := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if !timeTrackingCutoverPermission(base, candidate, ScopeExternalTest, suite, presence) {
		t.Fatal("moved suite lost the fixture reach it had")
	}
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest} {
		if timeTrackingCutoverPermission(base, candidate, scope, suite, presence) {
			t.Fatalf("suite reach leaked into %s", scope)
		}
	}
	foreign := Package{Owner: "meal-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if timeTrackingCutoverPermission(base, candidate, ScopeExternalTest, foreign, presence) {
		t.Fatal("reach granted to a foreign owner's suite")
	}
	route := Package{Owner: "workforce", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	if !timeTrackingCutoverPermission(base, candidate, ScopeExternalTest, route, presence) {
		t.Fatal("a Workforce behaviour suite in the receiving test role lost the moved reach")
	}
}

func TestTimeTrackingCutoverRequiresACompleteReviewedRetirement(t *testing.T) {
	t.Parallel()
	rule := Rule{ID: "historical.time-tracking-reach", Scopes: []string{"production"}, SourceOwner: "workforce", SourceRole: "adapter", TargetOwner: "student-presence", TargetRole: "public"}
	source := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	target := timeTrackingPoint("student-presence", "public")
	for name, change := range map[string]func(base, candidate *Policy){
		"same epoch":       func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch },
		"retained package": func(base, candidate *Policy) { candidate.Packages = base.Packages },
		"retained subpackage": func(_, candidate *Policy) {
			candidate.Packages = append(candidate.Packages, Package{Path: timeTrackingLegacyPath + "/rest", Owner: "workforce", Role: "application"})
		},
		"reclassified base": func(base, _ *Policy) { base.Packages[0].Role = "application" },
		"foreign module":    func(_, candidate *Policy) { candidate.ModulePath = "example.test/other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := timeTrackingCutoverPolicies(t, rule)
			change(base, candidate)
			if timeTrackingCutoverPermission(base, candidate, ScopeProduction, source, target) {
				t.Fatal("cutover accepted without a complete reviewed retirement")
			}
		})
	}
}
