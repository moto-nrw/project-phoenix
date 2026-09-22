package architecture

import (
	"path/filepath"
	"slices"
	"testing"
)

// shiftPlanningCutoverPolicies returns a base that still classifies the
// retired compatibility package and a reviewed candidate that retired it.
// The historical permissions are explicit so the fixture keeps exercising
// the retired package after the repository's own policy removed it.
func shiftPlanningCutoverPolicies(t *testing.T, historical ...Rule) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 21
	base.Packages = []Package{
		shiftPlanningRetiredPackage,
		{Path: "api", Owner: "root-composition", Role: "compose", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce", Owner: "workforce", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce/compose", Owner: "workforce", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce/internal/application", Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/timetable", Owner: "timetable-activities", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/timetable/compose", Owner: "timetable-activities", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "models/users", Owner: "people-directory", Role: "domain", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
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
	candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool { return p.Path == shiftPlanningLegacyPath })
	return base, &candidate
}

func shiftPlanningHistoricalConsumer(scope Scope, owner, role string) Rule {
	return Rule{ID: "historical.shift-planning", Scopes: []string{string(scope)}, SourceOwner: owner, SourceRole: role, TargetOwner: "inbound-staff-shifts", TargetRole: "adapter"}
}

func shiftPlanningPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestShiftPlanningCutoverReplacesConsumerPermission(t *testing.T) {
	t.Parallel()
	base, candidate := shiftPlanningCutoverPolicies(t, shiftPlanningHistoricalConsumer(ScopeProduction, "document-rendering", "adapter"))
	source := Package{Owner: "document-rendering", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	for _, owner := range []string{"workforce", "timetable-activities"} {
		if !shiftPlanningCutoverPermission(base, candidate, ScopeProduction, source, shiftPlanningPoint(owner, "public")) {
			t.Fatalf("former consumer cannot reach %s/public", owner)
		}
		for _, role := range []string{"compose", "application", "port", "postgres", "domain", "adapter", "http"} {
			if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, source, shiftPlanningPoint(owner, role)) {
				t.Fatalf("production consumer reached %s/%s", owner, role)
			}
		}
	}
	target := shiftPlanningPoint("workforce", "public")
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if shiftPlanningCutoverPermission(base, candidate, scope, source, target) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
	if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, source, shiftPlanningPoint("meal-plan", "public")) {
		t.Fatal("replacement granted an unrelated owner")
	}
	stranger := Package{Owner: "communication", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, stranger, target) {
		t.Fatal("replacement did not require the historical permission")
	}
}

func TestShiftPlanningCutoverConstructsTheReplacementInTestScopes(t *testing.T) {
	t.Parallel()
	base, candidate := shiftPlanningCutoverPolicies(t, shiftPlanningHistoricalConsumer(ScopeInternalTest, "document-rendering", "adapter-test"))
	source := Package{Owner: "document-rendering", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	compose := shiftPlanningPoint("workforce", "compose")
	if !shiftPlanningCutoverPermission(base, candidate, ScopeInternalTest, source, compose) {
		t.Fatal("suite that constructed the retired service cannot construct its replacement")
	}
	if shiftPlanningCutoverPermission(base, candidate, ScopeExternalTest, source, compose) {
		t.Fatal("test construction leaked into another test scope")
	}
	if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, source, compose) {
		t.Fatal("test construction leaked into production")
	}
	if shiftPlanningCutoverPermission(base, candidate, ScopeInternalTest, source, shiftPlanningPoint("workforce", "application")) {
		t.Fatal("test construction reached the application implementation")
	}
}

func TestShiftPlanningCutoverKeepsTheRetiredCodesReach(t *testing.T) {
	t.Parallel()
	historical := Rule{ID: "historical.shift-planning-reach", Scopes: []string{"production"}, SourceOwner: "inbound-staff-shifts", SourceRole: "adapter", TargetOwner: "people-directory", TargetRole: "domain"}
	base, candidate := shiftPlanningCutoverPolicies(t, historical)
	rows := shiftPlanningPoint("people-directory", "domain")
	for _, receiver := range []Package{
		{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Owner: "workforce", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Owner: "timetable-activities", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
	} {
		if !shiftPlanningCutoverPermission(base, candidate, ScopeProduction, receiver, rows) {
			t.Fatalf("%s/%s lost the reach the retired code had", receiver.Owner, receiver.Role)
		}
		for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
			if shiftPlanningCutoverPermission(base, candidate, scope, receiver, rows) {
				t.Fatalf("production reach of %s/%s lent to %s", receiver.Owner, receiver.Role, scope)
			}
		}
	}
	for _, other := range []Package{
		shiftPlanningPoint("workforce", "public"),
		shiftPlanningPoint("workforce", "http"),
		shiftPlanningPoint("workforce", "postgres"),
		shiftPlanningPoint("timetable-activities", "application"),
		shiftPlanningPoint("meal-plan", "application"),
	} {
		if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, other, rows) {
			t.Fatalf("%s/%s received reach it never held", other.Owner, other.Role)
		}
	}
	receiver := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, receiver, shiftPlanningPoint("meal-plan", "public")) {
		t.Fatal("receiving role gained a target the retired code never had")
	}
}

func TestShiftPlanningCutoverMovesTheSuites(t *testing.T) {
	t.Parallel()
	historical := Rule{ID: "historical.shift-planning-suite", Scopes: []string{"external_test"}, SourceOwner: "inbound-staff-shifts", SourceRole: "module-behavior-test", TargetOwner: "people-directory", TargetRole: "domain"}
	base, candidate := shiftPlanningCutoverPolicies(t, historical)
	rows := shiftPlanningPoint("people-directory", "domain")
	suite := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if !shiftPlanningCutoverPermission(base, candidate, ScopeExternalTest, suite, rows) {
		t.Fatal("moved suite lost the fixture reach it had")
	}
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest} {
		if shiftPlanningCutoverPermission(base, candidate, scope, suite, rows) {
			t.Fatalf("suite reach leaked into %s", scope)
		}
	}
	route := Package{Owner: "timetable-activities", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "workflow-integration-test"}
	if !shiftPlanningCutoverPermission(base, candidate, ScopeExternalTest, route, rows) {
		t.Fatal("moved route suite lost the fixture reach it had")
	}
	foreign := Package{Owner: "meal-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if shiftPlanningCutoverPermission(base, candidate, ScopeExternalTest, foreign, rows) {
		t.Fatal("reach granted to a foreign owner's suite")
	}
}

func TestShiftPlanningCutoverRequiresACompleteReviewedRetirement(t *testing.T) {
	t.Parallel()
	rule := shiftPlanningHistoricalConsumer(ScopeProduction, "document-rendering", "adapter")
	source := Package{Owner: "document-rendering", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	target := shiftPlanningPoint("workforce", "public")
	for name, change := range map[string]func(base, candidate *Policy){
		"same epoch":       func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch },
		"retained package": func(base, candidate *Policy) { candidate.Packages = base.Packages },
		"retained subpackage": func(_, candidate *Policy) {
			candidate.Packages = append(candidate.Packages, Package{Path: shiftPlanningLegacyPath + "/rest", Owner: "workforce", Role: "application"})
		},
		"reclassified base": func(base, _ *Policy) { base.Packages[0].Owner = "workforce" },
		"foreign module":    func(_, candidate *Policy) { candidate.ModulePath = "example.test/other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := shiftPlanningCutoverPolicies(t, rule)
			change(base, candidate)
			if shiftPlanningCutoverPermission(base, candidate, ScopeProduction, source, target) {
				t.Fatal("cutover accepted without a complete reviewed retirement")
			}
		})
	}
}
