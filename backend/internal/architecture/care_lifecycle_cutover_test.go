package architecture

import (
	"path/filepath"
	"slices"
	"testing"
)

// careLifecycleCutoverPolicies returns a base that still classifies the
// retired adapter and a reviewed candidate that retired it. The historical
// permissions are explicit so the fixture keeps exercising the retired
// adapter after the repository's own policy removed it.
func careLifecycleCutoverPolicies(t *testing.T, historical ...Rule) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 19
	base.Packages = []Package{
		{Path: careLifecycleLegacyPath, Owner: "inbound-students", Role: "adapter", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "api/operator", Owner: "inbound-operator", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "modules/careplan", Owner: "care-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/contracttest", Owner: "care-plan", Role: "test-support", InternalTestRole: "e2e-test", ExternalTestRole: "e2e-test"},
		{Path: "models/audit", Owner: "audit-platform", Role: "domain", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
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
	candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool { return p.Path == careLifecycleLegacyPath })
	return base, &candidate
}

func careLifecycleHistoricalConsumer(scope Scope, owner, role string) Rule {
	return Rule{ID: "historical.care-lifecycle", Scopes: []string{string(scope)}, SourceOwner: owner, SourceRole: role, TargetOwner: "inbound-students", TargetRole: "adapter"}
}

func TestCareLifecycleCutoverReplacesConsumerPermission(t *testing.T) {
	t.Parallel()
	base, candidate := careLifecycleCutoverPolicies(t, careLifecycleHistoricalConsumer(ScopeProduction, "inbound-operator", "http"))
	source := Package{Owner: "inbound-operator", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	for _, role := range []string{"public", "contract"} {
		if !careLifecycleCutoverPermission(base, candidate, ScopeProduction, source, Package{Owner: "care-plan", Role: role}) {
			t.Fatalf("former consumer cannot reach care-plan/%s", role)
		}
	}
	for _, role := range []string{"compose", "application", "port", "postgres", "domain", "test-support"} {
		if careLifecycleCutoverPermission(base, candidate, ScopeProduction, source, Package{Owner: "care-plan", Role: role}) {
			t.Fatalf("production consumer reached care-plan/%s", role)
		}
	}
	target := Package{Owner: "care-plan", Role: "public"}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if careLifecycleCutoverPermission(base, candidate, scope, source, target) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
	if careLifecycleCutoverPermission(base, candidate, ScopeProduction, source, Package{Owner: "meal-plan", Role: "public"}) {
		t.Fatal("replacement granted an unrelated owner")
	}
	stranger := Package{Owner: "communication", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if careLifecycleCutoverPermission(base, candidate, ScopeProduction, stranger, target) {
		t.Fatal("replacement did not require the historical permission")
	}
}

func TestCareLifecycleCutoverReplacesTestConstruction(t *testing.T) {
	t.Parallel()
	base, candidate := careLifecycleCutoverPolicies(t, careLifecycleHistoricalConsumer(ScopeExternalTest, "enrollment", "module-behavior-test"))
	source := Package{Owner: "enrollment", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	compose := Package{Owner: "care-plan", Role: "compose"}
	if !careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, source, compose) {
		t.Fatal("suite that constructed the retired service cannot construct its replacement")
	}
	if careLifecycleCutoverPermission(base, candidate, ScopeInternalTest, source, compose) {
		t.Fatal("test construction leaked into another test scope")
	}
	if careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, source, Package{Owner: "care-plan", Role: "application"}) {
		t.Fatal("test construction reached the application implementation")
	}
}

func TestCareLifecycleCutoverMovesTheRetiredSuites(t *testing.T) {
	t.Parallel()
	historical := Rule{ID: "historical.care-lifecycle-suite", Scopes: []string{"external_test"}, SourceOwner: "inbound-students", SourceRole: "module-behavior-test", TargetOwner: "audit-platform", TargetRole: "domain"}
	base, candidate := careLifecycleCutoverPolicies(t, historical)
	suite := Package{Owner: "care-plan", Role: "test-support", InternalTestRole: "e2e-test", ExternalTestRole: "e2e-test"}
	audit := Package{Owner: "audit-platform", Role: "domain"}
	if !careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, suite, audit) {
		t.Fatal("moved suite lost the fixture reach it had")
	}
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest} {
		if careLifecycleCutoverPermission(base, candidate, scope, suite, audit) {
			t.Fatalf("suite reach leaked into %s", scope)
		}
	}
	if careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, suite, Package{Owner: "meal-plan", Role: "public"}) {
		t.Fatal("moved suite gained a target the retired suites never had")
	}
	behavior := Package{Owner: "care-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, behavior, audit) {
		t.Fatal("reach granted to a Care Plan suite other than the contract suite")
	}
	selfReach := Rule{ID: "historical.care-lifecycle-self", Scopes: []string{"external_test"}, SourceOwner: "inbound-students", SourceRole: "module-behavior-test", TargetOwner: "inbound-students", TargetRole: "http"}
	base, candidate = careLifecycleCutoverPolicies(t, historical, selfReach)
	if careLifecycleCutoverPermission(base, candidate, ScopeExternalTest, suite, Package{Owner: "inbound-students", Role: "http"}) {
		t.Fatal("moved suite kept a dependency on the retired owner")
	}
}

func TestCareLifecycleCutoverRequiresACompleteReviewedRetirement(t *testing.T) {
	t.Parallel()
	rule := careLifecycleHistoricalConsumer(ScopeProduction, "inbound-operator", "http")
	source := Package{Owner: "inbound-operator", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	target := Package{Owner: "care-plan", Role: "public"}
	for name, change := range map[string]func(base, candidate *Policy){
		"same epoch":       func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch },
		"retained adapter": func(base, candidate *Policy) { candidate.Packages = base.Packages },
		"retained subpackage": func(_, candidate *Policy) {
			candidate.Packages = append(candidate.Packages, Package{Path: careLifecycleLegacyPath + "/rest", Owner: "care-plan", Role: "application"})
		},
		"reclassified base": func(base, _ *Policy) { base.Packages[0].Owner = "care-plan" },
		"foreign module":    func(_, candidate *Policy) { candidate.ModulePath = "example.test/other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := careLifecycleCutoverPolicies(t, rule)
			change(base, candidate)
			if careLifecycleCutoverPermission(base, candidate, ScopeProduction, source, target) {
				t.Fatal("cutover accepted without a complete reviewed retirement")
			}
		})
	}
}
