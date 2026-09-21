package architecture

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// studentPresenceCutoverPolicies returns a base that still classifies the four
// retired nest packages and a reviewed candidate that retired them. The
// historical permissions are explicit so the fixture keeps exercising the nest
// after the repository's own policy removed it.
func studentPresenceCutoverPolicies(t *testing.T, historical ...Rule) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 20
	base.Packages = []Package{
		{Path: studentPresenceLegacyPath + "/services/active", Owner: "student-presence", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "e2e-test"},
		{Path: studentPresenceLegacyPath + "/models/active", Owner: "student-presence", Role: "domain", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"},
		{Path: studentPresenceLegacyPath + "/repositories/active", Owner: "student-presence", Role: "postgres", InternalTestRole: "module-internal-test", ExternalTestRole: "e2e-test"},
		{Path: studentPresenceLegacyPath + "/statistics", Owner: "student-presence", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"},
		{Path: "api/groups", Owner: "inbound-groups", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "modules/studentpresence", Owner: "student-presence", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/studentpresence/internal/adapters/postgres", Owner: "student-presence", Role: "postgres", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/carerequests", Owner: "care-plan", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/workforce", Owner: "workforce", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "internal/timezone", Owner: "legacy-shared", Role: "domain", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
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
	candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool {
		return strings.HasPrefix(p.Path, studentPresenceLegacyPath+"/")
	})
	// The retired repository suite moved into the native Postgres adapter and
	// brought its external test role with it, as the reviewed epoch records.
	for i := range candidate.Packages {
		if candidate.Packages[i].Path == "modules/studentpresence/internal/adapters/postgres" {
			candidate.Packages[i].ExternalTestRole = "e2e-test"
		}
	}
	return base, &candidate
}

// studentPresenceHistoricalConsumer is a permission to import one retired
// package, the shape every consumer replacement must be anchored on.
func studentPresenceHistoricalConsumer(scope Scope, owner, role, legacyRole string) Rule {
	return Rule{
		ID: "historical.student-presence." + legacyRole, Scopes: []string{string(scope)},
		SourceOwner: owner, SourceRole: role, TargetOwner: "student-presence", TargetRole: legacyRole,
	}
}

func studentPresencePoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestStudentPresenceCutoverReplacesConsumerPermission(t *testing.T) {
	t.Parallel()
	base, candidate := studentPresenceCutoverPolicies(t, studentPresenceHistoricalConsumer(ScopeProduction, "inbound-groups", "http", "adapter"))
	source := Package{Owner: "inbound-groups", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	target := studentPresencePoint("student-presence", "public")
	if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, target) {
		t.Fatal("former consumer cannot reach the public contract")
	}
	for _, role := range []string{"application", "port", "postgres", "domain", "adapter", "compose", "test-support"} {
		if studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, studentPresencePoint("student-presence", role)) {
			t.Fatalf("production consumer reached student-presence/%s", role)
		}
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if studentPresenceCutoverPermission(base, candidate, scope, source, target) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, studentPresencePoint("meal-plan", "public")) {
		t.Fatal("replacement granted an unrelated owner")
	}
	stranger := Package{Owner: "communication", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, stranger, target) {
		t.Fatal("replacement did not require the historical permission")
	}
}

func TestStudentPresenceCutoverHandsTheRowsToTheirOwners(t *testing.T) {
	t.Parallel()
	base, candidate := studentPresenceCutoverPolicies(t, studentPresenceHistoricalConsumer(ScopeProduction, "people-directory", "application", "domain"))
	source := studentPresencePoint("people-directory", "application")
	for _, target := range []Package{
		studentPresencePoint("care-plan", "contract"),
		studentPresencePoint("workforce", "public"),
		studentPresencePoint("workforce", "adapter"),
	} {
		if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, target) {
			t.Fatalf("consumer of the retired rows cannot reach %s/%s", target.Owner, target.Role)
		}
	}
	for _, target := range []Package{
		studentPresencePoint("care-plan", "application"),
		studentPresencePoint("care-plan", "compose"),
		studentPresencePoint("care-plan", "test-support"),
		studentPresencePoint("workforce", "postgres"),
		studentPresencePoint("workforce", "application"),
	} {
		if studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, target) {
			t.Fatalf("hand-back reached %s/%s", target.Owner, target.Role)
		}
	}
	fixtures := studentPresencePoint("test-support", "test-support")
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, fixtures, studentPresencePoint("care-plan", "test-support")) {
		t.Fatal("the fixture catalog reached the fixture rows without the historical permission")
	}
	base, candidate = studentPresenceCutoverPolicies(t, studentPresenceHistoricalConsumer(ScopeProduction, "test-support", "test-support", "domain"))
	if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, fixtures, studentPresencePoint("care-plan", "test-support")) {
		t.Fatal("the fixture catalog cannot insert the handed-back fixture rows")
	}
}

func TestStudentPresenceCutoverCarriesTheRowValue(t *testing.T) {
	t.Parallel()
	reach := Rule{
		ID: "historical.student-presence.calendar", Scopes: []string{"production"},
		SourceOwner: "student-presence", SourceRole: "domain", TargetOwner: "legacy-shared", TargetRole: "domain",
	}
	base, candidate := studentPresenceCutoverPolicies(t, reach)
	calendar := studentPresencePoint("legacy-shared", "domain")
	for _, source := range []Package{
		studentPresencePoint("care-plan", "contract"),
		studentPresencePoint("care-plan", "test-support"),
	} {
		if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, calendar) {
			t.Fatalf("%s/%s lost the calendar-date value of the handed-back rows", source.Owner, source.Role)
		}
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("care-plan", "application"), calendar) {
		t.Fatal("the row value reach leaked to another Care Plan role")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("care-plan", "contract"), studentPresencePoint("meal-plan", "public")) {
		t.Fatal("the row packages inherited more than the calendar-date value")
	}
}

func TestStudentPresenceCutoverKeepsTheRetiredCodesReach(t *testing.T) {
	t.Parallel()
	reach := Rule{
		ID: "historical.student-presence.reach", Scopes: []string{"production"},
		SourceOwner: "student-presence", SourceRole: "adapter", TargetOwner: "delivery-platform", TargetRole: "application",
	}
	base, candidate := studentPresenceCutoverPolicies(t, reach)
	publisher := studentPresencePoint("delivery-platform", "application")
	if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("student-presence", "application"), publisher) {
		t.Fatal("the application lost a dependency the retired services had")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("student-presence", "http"), publisher) {
		t.Fatal("a role that received no retired code inherited its reach")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("care-plan", "contract"), publisher) {
		t.Fatal("a foreign owner inherited the retired code's reach")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeExternalTest, studentPresencePoint("student-presence", "e2e-test"), publisher) {
		t.Fatal("a production dependency was lent to a test scope")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("student-presence", "application"), studentPresencePoint("meal-plan", "public")) {
		t.Fatal("the application reached a target the retired services never had")
	}
}

func TestStudentPresenceCutoverMovesTheNestsSuites(t *testing.T) {
	t.Parallel()
	inNest := Rule{
		ID: "historical.student-presence.suite", Scopes: []string{"external_test"},
		SourceOwner: "student-presence", SourceRole: "e2e-test", TargetOwner: "student-presence", TargetRole: "domain",
	}
	base, candidate := studentPresenceCutoverPolicies(t, inNest)
	adapter := base.packageMap()[base.ModulePath+"/modules/studentpresence/internal/adapters/postgres"]
	for _, target := range []Package{
		studentPresencePoint("student-presence", "application"),
		studentPresencePoint("student-presence", "port"),
		studentPresencePoint("care-plan", "contract"),
	} {
		if !studentPresenceCutoverPermission(base, candidate, ScopeExternalTest, adapter, target) {
			t.Fatalf("the moved repository suite cannot reach %s/%s", target.Owner, target.Role)
		}
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, adapter, studentPresencePoint("student-presence", "application")) {
		t.Fatal("the moved suite lent its reach to production")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeExternalTest, adapter, studentPresencePoint("meal-plan", "public")) {
		t.Fatal("the moved suite gained a target outside the replacement points")
	}
	foreign := Package{Path: "modules/careplan/carerequests", Owner: "care-plan", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if studentPresenceCutoverPermission(base, candidate, ScopeExternalTest, foreign, studentPresencePoint("student-presence", "port")) {
		t.Fatal("a foreign owner's suite reached the module's internals")
	}
}

func TestStudentPresenceCutoverConstructsTheReplacementInTestScopes(t *testing.T) {
	t.Parallel()
	base, candidate := studentPresenceCutoverPolicies(t, studentPresenceHistoricalConsumer(ScopeInternalTest, "inbound-students", "adapter-test", "adapter"))
	source := Package{Owner: "inbound-students", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	composition := studentPresencePoint("student-presence", "compose")
	if !studentPresenceCutoverPermission(base, candidate, ScopeInternalTest, source, composition) {
		t.Fatal("a suite that constructed the retired service cannot construct its replacement")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, composition) {
		t.Fatal("test construction leaked into production")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeExternalTest, source, composition) {
		t.Fatal("test construction leaked into another test scope")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeInternalTest, source, studentPresencePoint("student-presence", "application")) {
		t.Fatal("a foreign suite reached the application implementation")
	}
}

func TestStudentPresenceCutoverPinsTheFacilitiesRoomPort(t *testing.T) {
	t.Parallel()
	base, candidate := studentPresenceCutoverPolicies(t,
		studentPresenceHistoricalConsumer(ScopeProduction, "facilities", "compose", "adapter"),
		studentPresenceHistoricalConsumer(ScopeProduction, "open-room-move", "compose", "adapter"),
	)
	composition := studentPresencePoint("student-presence", "compose")
	if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("facilities", "compose"), composition) {
		t.Fatal("Facilities cannot implement the retired attendance-room port")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("open-room-move", "compose"), composition) {
		t.Fatal("the pinned production point admitted another composition")
	}
}

func TestStudentPresenceCutoverPinsTheHandBackBindings(t *testing.T) {
	t.Parallel()
	base, candidate := studentPresenceCutoverPolicies(t)
	rows := studentPresencePoint("care-plan", "contract")
	for _, source := range []Package{
		studentPresencePoint("student-presence", "public"),
		studentPresencePoint("care-plan", "contract"),
	} {
		if !studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, rows) {
			t.Fatalf("%s/%s cannot name the handed-back rows", source.Owner, source.Role)
		}
		for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
			if studentPresenceCutoverPermission(base, candidate, scope, source, rows) {
				t.Fatalf("the pinned binding leaked into %s", scope)
			}
		}
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("student-presence", "compose"), rows) {
		t.Fatal("the pinned binding admitted another Student Presence role")
	}
	if studentPresenceCutoverPermission(base, candidate, ScopeProduction, studentPresencePoint("student-presence", "public"), studentPresencePoint("care-plan", "public")) {
		t.Fatal("the pinned binding admitted another Care Plan role")
	}
}

func TestStudentPresenceCutoverRequiresACompleteReviewedRetirement(t *testing.T) {
	t.Parallel()
	rule := studentPresenceHistoricalConsumer(ScopeProduction, "inbound-groups", "http", "adapter")
	source := Package{Owner: "inbound-groups", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	target := studentPresencePoint("student-presence", "public")
	for name, change := range map[string]func(base, candidate *Policy){
		"same epoch":    func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch },
		"lower epoch":   func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch - 1 },
		"retained nest": func(base, candidate *Policy) { candidate.Packages = base.Packages },
		"retained subtree": func(_, candidate *Policy) {
			candidate.Packages = append(candidate.Packages, Package{Path: studentPresenceLegacyPath, Owner: "student-presence", Role: "application"})
		},
		"retained subpackage": func(_, candidate *Policy) {
			candidate.Packages = append(candidate.Packages, Package{Path: studentPresenceLegacyPath + "/services/active/rest", Owner: "student-presence", Role: "application"})
		},
		"reclassified base": func(base, _ *Policy) { base.Packages[1].Role = "contract" },
		"retired package missing from the base": func(base, _ *Policy) {
			base.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool {
				return p.Path == studentPresenceLegacyPath+"/statistics"
			})
		},
		"foreign module": func(_, candidate *Policy) { candidate.ModulePath = "example.test/other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := studentPresenceCutoverPolicies(t, rule)
			change(base, candidate)
			if studentPresenceCutoverPermission(base, candidate, ScopeProduction, source, target) {
				t.Fatal("cutover accepted without a complete reviewed retirement")
			}
		})
	}
}
