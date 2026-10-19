package architecture

import (
	"slices"
	"strings"
	"testing"
)

// internalReachOwners is a small owner map with the kinds the reach of a
// domain owner's implementation distinguishes.
var internalReachOwners = map[string]Owner{
	"timetable-activities": {ID: "timetable-activities", Kind: "domain"},
	"student-presence":     {ID: "student-presence", Kind: "domain"},
	"workforce":            {ID: "workforce", Kind: "domain"},
	"tenant-runtime":       {ID: "tenant-runtime", Kind: "platform"},
	"transaction-runtime":  {ID: "transaction-runtime", Kind: "platform"},
	"delivery-platform":    {ID: "delivery-platform", Kind: "platform"},
	"school-setup":         {ID: "school-setup", Kind: "projection"},
	"session-end":          {ID: "session-end", Kind: "workflow"},
	"care-plan":            {ID: "care-plan", Kind: "domain"},
}

// internalReachBase holds the owner edges the base grants: Timetable's
// composition reaches Student Presence and School Setup, Workforce's
// application reaches Timetable.
var internalReachBase = &Policy{Rules: []Rule{
	{ID: "timetable-activities.compose.student-presence.public", SourceOwner: "timetable-activities", SourceRole: "compose", TargetOwner: "student-presence", TargetRole: "public", Scopes: []string{"production"}},
	{ID: "timetable-activities.compose.school-setup.public", SourceOwner: "timetable-activities", SourceRole: "compose", TargetOwner: "school-setup", TargetRole: "public", Scopes: []string{"production"}},
	{ID: "timetable-activities.compose.session-end.public", SourceOwner: "timetable-activities", SourceRole: "compose", TargetOwner: "session-end", TargetRole: "public", Scopes: []string{"production"}},
	{ID: "workforce.application.timetable-activities.public", SourceOwner: "workforce", SourceRole: "application", TargetOwner: "timetable-activities", TargetRole: "public", Scopes: []string{"production"}},
	{ID: "care-plan.contracttest.timetable-activities.compose", SourceOwner: "care-plan", SourceRole: "e2e-test", TargetOwner: "timetable-activities", TargetRole: "compose", Scopes: []string{"external_test"}},
	{ID: "timetable-activities.module-internal-test.workforce.public", SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "workforce", TargetRole: "public", Scopes: []string{"internal_test"}},
}}

func TestReviewedInternalReachRuleShape(t *testing.T) {
	t.Parallel()
	production := []string{"production"}
	internalTest := []string{"internal_test"}
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"generic own public from application", Rule{SourceOwnerKind: "domain", SourceRole: "application", TargetRole: "public", SameOwner: true, Scopes: production}, true},
		{"generic own public from postgres", Rule{SourceOwnerKind: "domain", SourceRole: "postgres", TargetRole: "public", SameOwner: true, Scopes: production}, true},
		{"platform public: tenant runtime", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, true},
		{"platform public: transaction runtime", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "transaction-runtime", TargetRole: "public", Scopes: production}, true},
		{"foreign domain with a base edge", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "student-presence", TargetRole: "public", Scopes: production}, true},
		{"foreign domain with a base edge from postgres", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "student-presence", TargetRole: "public", Scopes: production}, true},

		{"platform application role", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "transaction-runtime", TargetRole: "application", Scopes: production}, false},
		{"platform adapter role", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "tenant-runtime", TargetRole: "adapter", Scopes: production}, false},
		{"platform contract role", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "contract", Scopes: production}, false},
		{"platform http role", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "http", Scopes: production}, false},
		{"unlisted platform public", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "delivery-platform", TargetRole: "public", Scopes: production}, false},
		{"reverse DAG direction", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "workforce", TargetRole: "public", Scopes: production}, false},
		{"foreign domain without a base edge", Rule{SourceOwner: "student-presence", SourceRole: "application", TargetOwner: "timetable-activities", TargetRole: "public", Scopes: production}, false},
		{"projection target", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "school-setup", TargetRole: "public", Scopes: production}, false},
		{"workflow target", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "session-end", TargetRole: "public", Scopes: production}, false},
		{"projection source", Rule{SourceOwner: "school-setup", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"workflow source", Rule{SourceOwner: "session-end", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"platform source", Rule{SourceOwner: "delivery-platform", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"composition source role", Rule{SourceOwner: "timetable-activities", SourceRole: "compose", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"domain source role", Rule{SourceOwner: "timetable-activities", SourceRole: "domain", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"test scopes", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: everyScope}, false},
		{"own owner named", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "timetable-activities", TargetRole: "public", Scopes: production}, false},
		{"generic own contract", Rule{SourceOwnerKind: "domain", SourceRole: "application", TargetRole: "contract", SameOwner: true, Scopes: production}, false},
		{"generic own public of platforms", Rule{SourceOwnerKind: "platform", SourceRole: "application", TargetRole: "public", SameOwner: true, Scopes: production}, false},
		{"generic own public of projections", Rule{SourceOwnerKind: "projection", SourceRole: "postgres", TargetRole: "public", SameOwner: true, Scopes: production}, false},
		{"generic own public for every kind", Rule{SourceRole: "application", TargetRole: "public", SameOwner: true, Scopes: production}, false},
		{"generic own public in test scopes", Rule{SourceOwnerKind: "domain", SourceRole: "application", TargetRole: "public", SameOwner: true, Scopes: everyScope}, false},
		{"kind-wide foreign public", Rule{SourceOwnerKind: "domain", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"target owner kind", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetOwnerKind: "platform", TargetRole: "public", Scopes: production}, false},
		{"external class", Rule{SourceOwner: "timetable-activities", SourceRole: "application", TargetClass: "orm-sql", Scopes: production}, false},
		{"unknown source owner", Rule{SourceOwner: "nobody", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},

		{"internal test mirror: platform public", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: internalTest}, true},
		{"internal test mirror: foreign domain with a base edge", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "student-presence", TargetRole: "public", Scopes: internalTest}, true},
		{"internal test mirror in external_test", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"external_test"}}, false},
		{"internal test mirror in both test scopes", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"internal_test", "external_test"}}, false},
		{"internal test mirror in production", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, false},
		{"behaviour test role", Rule{SourceOwner: "timetable-activities", SourceRole: "module-behavior-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"external_test"}}, false},
		{"e2e test role", Rule{SourceOwner: "timetable-activities", SourceRole: "e2e-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: internalTest}, false},
		{"internal test mirror: target production would not get", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "delivery-platform", TargetRole: "public", Scopes: internalTest}, false},
		{"internal test mirror: reverse DAG direction", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "workforce", TargetRole: "public", Scopes: internalTest}, false},
		{"internal test mirror: platform application role", Rule{SourceOwner: "timetable-activities", SourceRole: "module-internal-test", TargetOwner: "transaction-runtime", TargetRole: "application", Scopes: internalTest}, false},
		{"internal test mirror: generic own public", Rule{SourceOwnerKind: "domain", SourceRole: "module-internal-test", TargetRole: "public", SameOwner: true, Scopes: internalTest}, false},
		{"internal test mirror: projection source", Rule{SourceOwner: "school-setup", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: internalTest}, false},

		{"port: generic own public", Rule{SourceOwnerKind: "domain", SourceRole: "port", TargetRole: "public", SameOwner: true, Scopes: production}, true},
		{"port: platform public", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}, true},
		{"port: foreign domain with a base edge", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "student-presence", TargetRole: "public", Scopes: production}, true},
		{"port: own legacy model (temporary)", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "timetable-activities", TargetRole: "legacy-model", Scopes: production}, true},
		{"port: reverse DAG direction", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "workforce", TargetRole: "public", Scopes: production}, false},
		{"port: platform non-public role", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "transaction-runtime", TargetRole: "application", Scopes: production}, false},
		{"port: projection target", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "school-setup", TargetRole: "public", Scopes: production}, false},
		{"port: workflow target", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "session-end", TargetRole: "public", Scopes: production}, false},
		{"port: another owner's legacy model", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "workforce", TargetRole: "legacy-model", Scopes: production}, false},
		{"port: legacy model in test scopes", Rule{SourceOwner: "timetable-activities", SourceRole: "port", TargetOwner: "timetable-activities", TargetRole: "legacy-model", Scopes: everyScope}, false},
		{"port: legacy model of a projection", Rule{SourceOwner: "school-setup", SourceRole: "port", TargetOwner: "school-setup", TargetRole: "legacy-model", Scopes: production}, false},
		{"postgres: own port", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "timetable-activities", TargetRole: "port", Scopes: production}, true},
		{"postgres: another owner's port", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "student-presence", TargetRole: "port", Scopes: production}, false},
		{"postgres: own port in test scopes", Rule{SourceOwner: "timetable-activities", SourceRole: "postgres", TargetOwner: "timetable-activities", TargetRole: "port", Scopes: everyScope}, false},
		{"postgres: own port of a projection", Rule{SourceOwner: "school-setup", SourceRole: "postgres", TargetOwner: "school-setup", TargetRole: "port", Scopes: production}, false},
		{"postgres: kind-wide own port", Rule{SourceOwnerKind: "domain", SourceRole: "postgres", TargetRole: "port", SameOwner: true, Scopes: production}, false},
		{"narrowing: compose reach to the port", Rule{SourceOwner: "care-plan", SourceRole: "e2e-test", TargetOwner: "timetable-activities", TargetRole: "port", Scopes: []string{"external_test"}}, true},
		{"narrowing: wider scope than the compose reach", Rule{SourceOwner: "care-plan", SourceRole: "e2e-test", TargetOwner: "timetable-activities", TargetRole: "port", Scopes: everyScope}, false},
		{"narrowing: no compose reach in the base", Rule{SourceOwner: "care-plan", SourceRole: "e2e-test", TargetOwner: "workforce", TargetRole: "port", Scopes: []string{"external_test"}}, false},
		{"narrowing: another source role", Rule{SourceOwner: "care-plan", SourceRole: "module-behavior-test", TargetOwner: "timetable-activities", TargetRole: "port", Scopes: []string{"external_test"}}, false},
	} {
		if got := reviewedInternalReachRule(scenario.rule, internalReachBase, internalReachOwners); got != scenario.want {
			t.Errorf("%s: reviewedInternalReachRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func TestInternalReachRulesRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	owners := ownersByID(candidate)
	base := *candidate
	base.PolicyEpoch--
	// The rules into a foreign domain need their edge in the base, so the
	// base keeps them; the generic rules and the platform rules go.
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), func(rule Rule) bool {
		return reviewedInternalReachRule(rule, candidate, owners) && owners[rule.TargetOwner].Kind != "domain"
	})
	if removed := len(candidate.Rules) - len(base.Rules); removed < 2 {
		t.Fatalf("expected at least the two generic own-public rules in the reach, found %d", removed)
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed internal reach rules: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("new internal reach permissions passed without an epoch")
	}
}

func TestInternalReachEpochGrantsOnlyTheReach(t *testing.T) {
	t.Parallel()
	production := []string{"production"}
	accepted := Rule{ID: "timetable-activities.application.tenant-runtime", Description: "test", SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production}
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	widened := *candidate
	widened.Rules = append(slices.Clone(candidate.Rules), accepted)
	if err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("%s was rejected: %v", accepted.ID, err)
	}
	for _, rule := range []Rule{
		{ID: "timetable-activities.application.transaction-runtime-application", Description: "test", SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "transaction-runtime", TargetRole: "application", Scopes: production},
		{ID: "student-presence.application.timetable-activities.public", Description: "test", SourceOwner: "student-presence", SourceRole: "application", TargetOwner: "timetable-activities", TargetRole: "public", Scopes: production},
		{ID: "people-directory.application.care-plan.public", Description: "test", SourceOwner: "people-directory", SourceRole: "application", TargetOwner: "care-plan", TargetRole: "public", Scopes: production},
		{ID: "timetable-activities.application.delivery-public", Description: "test", SourceOwner: "timetable-activities", SourceRole: "application", TargetOwner: "delivery-platform", TargetRole: "public", Scopes: production},
		{ID: "class-day-view.postgres.tenant-runtime", Description: "test", SourceOwner: "class-day-view", SourceRole: "postgres", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: production},
	} {
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), rule)
		err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil)
		if err == nil || !strings.Contains(err.Error(), rule.ID) {
			t.Fatalf("%s escaped the guard: %v", rule.ID, err)
		}
	}
}
