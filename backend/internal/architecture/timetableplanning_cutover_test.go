package architecture

import (
	"path/filepath"
	"testing"
)

// timetablePlanningCutoverPolicies returns a base that classifies the nest
// exactly and a reviewed candidate one epoch later that still carries it:
// #3424 dissolves the package slice by slice. The historical permissions are
// explicit so the fixture keeps exercising the anchors after the repository's
// own policy drops them.
func timetablePlanningCutoverPolicies(t *testing.T, historical ...Rule) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 21
	base.Packages = []Package{
		{Path: timetablePlanningLegacyPath, Owner: "inbound-timetable", Role: "adapter", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "api/timetable", Owner: "inbound-timetable", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "modules/schoolcalendar", Owner: "school-calendar", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/schoolcalendar/compose", Owner: "school-calendar", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/timetable", Owner: "timetable-activities", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
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
	return base, &candidate
}

// timetablePlanningHistoricalConsumer is a permission to import the nest,
// the shape every consumer replacement must be anchored on.
func timetablePlanningHistoricalConsumer(scope Scope, owner, role string) Rule {
	return Rule{
		ID: "historical.timetable-planning." + owner + "." + role, Scopes: []string{string(scope)},
		SourceOwner: owner, SourceRole: role, TargetOwner: "inbound-timetable", TargetRole: "adapter",
	}
}

func timetablePlanningPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestTimetablePlanningCutoverReplacesConsumerPermission(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t, timetablePlanningHistoricalConsumer(ScopeProduction, "inbound-timetable", "http"))
	source := Package{Owner: "inbound-timetable", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	calendar := timetablePlanningPoint("school-calendar", "public")
	if !timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, calendar) {
		t.Fatal("former consumer cannot reach the School Calendar contract")
	}
	for _, role := range []string{"compose", "application", "port", "postgres", "domain"} {
		if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, timetablePlanningPoint("school-calendar", role)) {
			t.Fatalf("production consumer reached school-calendar/%s", role)
		}
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if timetablePlanningCutoverPermission(base, candidate, scope, source, calendar) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, timetablePlanningPoint("meal-plan", "public")) {
		t.Fatal("replacement granted an unrelated owner")
	}
	stranger := Package{Owner: "communication", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, stranger, calendar) {
		t.Fatal("replacement did not require the historical permission")
	}
}

func TestTimetablePlanningCutoverReplacesTestScopedConsumers(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t,
		timetablePlanningHistoricalConsumer(ScopeExternalTest, "enrollment", "module-behavior-test"),
		timetablePlanningHistoricalConsumer(ScopeExternalTest, "test-support", "e2e-test"),
	)
	calendar := timetablePlanningPoint("school-calendar", "public")
	suite := Package{Owner: "enrollment", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if !timetablePlanningCutoverPermission(base, candidate, ScopeExternalTest, suite, calendar) {
		t.Fatal("the behaviour suite that drove the nest cannot name the School Calendar")
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeInternalTest, suite, calendar) {
		t.Fatal("an external-test permission was lent to the internal-test scope")
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, suite, calendar) {
		t.Fatal("a test permission was lent to production")
	}
	e2e := Package{Owner: "test-support", Role: "test-support", InternalTestRole: "e2e-test", ExternalTestRole: "e2e-test"}
	if !timetablePlanningCutoverPermission(base, candidate, ScopeExternalTest, e2e, calendar) {
		t.Fatal("the end-to-end suite that drove the nest cannot name the School Calendar")
	}
}

func TestTimetablePlanningCutoverLetsTheNestConsumeTheReplacement(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t)
	nest := base.packageMap()[base.ModulePath+"/"+timetablePlanningLegacyPath]
	calendar := timetablePlanningPoint("school-calendar", "public")
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest, ScopeExternalTest} {
		if !timetablePlanningCutoverPermission(base, candidate, scope, nest, calendar) {
			t.Fatalf("the retained nest cannot reach the owner of the moved behaviour in %s", scope)
		}
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, nest, timetablePlanningPoint("school-calendar", "compose")) {
		t.Fatal("the nest reached the School Calendar composition")
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, nest, timetablePlanningPoint("meal-plan", "public")) {
		t.Fatal("the nest gained a target outside the replacement point")
	}
	other := Package{Owner: "inbound-timetable", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, other, calendar) {
		t.Fatal("another inbound-timetable role was treated as the nest without a historical permission")
	}
}

func TestTimetablePlanningCutoverRequiresAReviewedDissolution(t *testing.T) {
	t.Parallel()
	rule := timetablePlanningHistoricalConsumer(ScopeProduction, "inbound-timetable", "http")
	source := Package{Owner: "inbound-timetable", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	calendar := timetablePlanningPoint("school-calendar", "public")
	for name, change := range map[string]func(base, candidate *Policy){
		"same epoch":        func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch },
		"lower epoch":       func(base, candidate *Policy) { candidate.PolicyEpoch = base.PolicyEpoch - 1 },
		"reclassified nest": func(base, _ *Policy) { base.Packages[0].Role = "application" },
		"nest missing from the base": func(base, _ *Policy) {
			base.Packages = base.Packages[1:]
		},
		"foreign module": func(_, candidate *Policy) { candidate.ModulePath = "example.test/other" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := timetablePlanningCutoverPolicies(t, rule)
			change(base, candidate)
			if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, calendar) {
				t.Fatal("cutover accepted without a reviewed dissolution")
			}
		})
	}
}

// Slice S4 (#3550) moved conflict detection and staffing to the Timetable
// owner: its former consumers and the nest itself may name that owner's
// public contract, and nothing else of it.
func TestTimetablePlanningCutoverReplacesConsumersWithTheTimetableContract(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t,
		timetablePlanningHistoricalConsumer(ScopeProduction, "workforce", "application"),
		timetablePlanningHistoricalConsumer(ScopeProduction, "calendar-view", "adapter"),
		timetablePlanningHistoricalConsumer(ScopeInternalTest, "workforce", "module-internal-test"),
		timetablePlanningHistoricalConsumer(ScopeExternalTest, "workforce", "module-behavior-test"),
	)
	timetablePublic := timetablePlanningPoint("timetable-activities", "public")
	workforce := Package{Owner: "workforce", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	dashboard := Package{Owner: "calendar-view", Role: "adapter", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"}
	for _, source := range []Package{workforce, dashboard} {
		if !timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, timetablePublic) {
			t.Fatalf("%s/%s cannot reach the Timetable contract", source.Owner, source.Role)
		}
		for _, role := range []string{"compose", "application", "port", "postgres", "domain"} {
			if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, source, timetablePlanningPoint("timetable-activities", role)) {
				t.Fatalf("%s/%s reached timetable-activities/%s", source.Owner, source.Role, role)
			}
		}
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if !timetablePlanningCutoverPermission(base, candidate, scope, workforce, timetablePublic) {
			t.Fatalf("the Workforce suites cannot name the Timetable contract in %s", scope)
		}
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeInternalTest, dashboard, timetablePublic) {
		t.Fatal("a production permission was lent to the internal-test scope")
	}
	nest := base.packageMap()[base.ModulePath+"/"+timetablePlanningLegacyPath]
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest, ScopeExternalTest} {
		if !timetablePlanningCutoverPermission(base, candidate, scope, nest, timetablePublic) {
			t.Fatalf("the retained nest cannot reach the Timetable contract in %s", scope)
		}
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, nest, timetablePlanningPoint("timetable-activities", "compose")) {
		t.Fatal("the nest reached the Timetable composition")
	}
	stranger := Package{Owner: "communication", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, stranger, timetablePublic) {
		t.Fatal("the Timetable replacement did not require the historical permission")
	}
}

// Slice S5 (#3551) moved the timetable reads, the operational day, the
// retention cleanup and the ended-session completion to the Timetable
// owner: the scheduler and the timetable end-to-end flows, which named the
// nest for them, may name the owner's public contract in their own scope,
// and nothing else of it.
func TestTimetablePlanningCutoverReplacesTheReadAndCleanupConsumers(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t,
		timetablePlanningHistoricalConsumer(ScopeProduction, "scheduler-runtime", "application"),
		timetablePlanningHistoricalConsumer(ScopeInternalTest, "scheduler-runtime", "module-internal-test"),
		timetablePlanningHistoricalConsumer(ScopeInternalTest, "test-support", "e2e-test"),
	)
	timetablePublic := timetablePlanningPoint("timetable-activities", "public")
	scheduler := Package{Owner: "scheduler-runtime", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	e2e := Package{Owner: "test-support", Role: "e2e-support", InternalTestRole: "e2e-test", ExternalTestRole: "e2e-test"}
	if !timetablePlanningCutoverPermission(base, candidate, ScopeProduction, scheduler, timetablePublic) {
		t.Fatal("the scheduler cannot reach the Timetable contract")
	}
	for _, role := range []string{"compose", "application", "port", "postgres", "domain"} {
		if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, scheduler, timetablePlanningPoint("timetable-activities", role)) {
			t.Fatalf("the scheduler reached timetable-activities/%s", role)
		}
	}
	for _, source := range []Package{scheduler, e2e} {
		if !timetablePlanningCutoverPermission(base, candidate, ScopeInternalTest, source, timetablePublic) {
			t.Fatalf("the %s suites cannot name the Timetable contract", source.Owner)
		}
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeExternalTest, scheduler, timetablePublic) {
		t.Fatal("an internal-test permission was lent to the external-test scope")
	}
	if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, e2e, timetablePublic) {
		t.Fatal("a test permission was lent to the production scope")
	}
}

// Slice S2 (#3552) moved the template writes, the materialization and the
// roster maintenance to the Timetable owner: Enrollment, which named the
// nest for the recurrence gate, the roster maintenance and the offering
// resync contract, may name the owner's public contract in production and
// nothing else of it.
func TestTimetablePlanningCutoverReplacesTheEnrollmentRosterConsumer(t *testing.T) {
	t.Parallel()
	base, candidate := timetablePlanningCutoverPolicies(t,
		timetablePlanningHistoricalConsumer(ScopeProduction, "enrollment", "application"),
	)
	timetablePublic := timetablePlanningPoint("timetable-activities", "public")
	enrollment := Package{Owner: "enrollment", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}
	if !timetablePlanningCutoverPermission(base, candidate, ScopeProduction, enrollment, timetablePublic) {
		t.Fatal("Enrollment cannot reach the Timetable contract")
	}
	for _, role := range []string{"compose", "application", "port", "postgres", "domain"} {
		if timetablePlanningCutoverPermission(base, candidate, ScopeProduction, enrollment, timetablePlanningPoint("timetable-activities", role)) {
			t.Fatalf("Enrollment reached timetable-activities/%s", role)
		}
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if timetablePlanningCutoverPermission(base, candidate, scope, enrollment, timetablePublic) {
			t.Fatalf("the production permission was lent to %s", scope)
		}
	}
}
