package architecture

import (
	"slices"
	"strings"
	"testing"
)

// reachOwners is a small owner map with one owner of every kind the reaches
// distinguish.
var reachOwners = map[string]Owner{
	"root-composition":   {ID: "root-composition", Kind: "composition"},
	"legacy-composition": {ID: "legacy-composition", Kind: "composition"},
	"migrations":         {ID: "migrations", Kind: "migration"},
	"workforce":          {ID: "workforce", Kind: "domain"},
	"tenant-runtime":     {ID: "tenant-runtime", Kind: "platform"},
	"session-end":        {ID: "session-end", Kind: "workflow"},
	"school-setup":       {ID: "school-setup", Kind: "projection"},
	"dev-tools":          {ID: "dev-tools", Kind: "tool"},
	"shared-kernel":      {ID: "shared-kernel", Kind: "kernel"},
	"test-support":       {ID: "test-support", Kind: "test-support"},
}

func TestReviewedRootCompositionRuleShape(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"domain", "platform", "workflow", "projection"} {
		for _, role := range []string{"compose", "public", "contract", "port", "http"} {
			for _, sourceRole := range []string{"", "compose", "cli", "module-internal-test", "module-behavior-test"} {
				rule := Rule{SourceOwner: "root-composition", SourceRole: sourceRole, TargetOwnerKind: kind, TargetRole: role, Scopes: everyScope}
				if !reviewedRootCompositionRule(rule, reachOwners) {
					t.Errorf("rejected the root reach into %s/%s from role %q", kind, role, sourceRole)
				}
			}
		}
	}
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"a named domain owner", Rule{SourceOwner: "root-composition", TargetOwner: "workforce", TargetRole: "http", Scopes: []string{"production"}}, true},
		{"the migrations", Rule{SourceOwner: "root-composition", TargetOwner: "migrations", TargetRole: "migration", Scopes: everyScope}, true},
		{"the root's own command package", Rule{SourceOwner: "root-composition", TargetOwner: "root-composition", TargetRole: "cli", Scopes: everyScope}, true},
		{"the root's own compose role from the commands root", Rule{SourceOwner: "root-composition", SourceRole: "cli", TargetOwner: "root-composition", TargetRole: "compose", Scopes: []string{"production"}}, true},
		{"the root's own packages through same_owner", Rule{SourceOwner: "root-composition", SameOwner: true, TargetRole: "compose", Scopes: everyScope}, true},
		{"one test scope", Rule{SourceOwner: "root-composition", SourceRole: "module-internal-test", TargetOwnerKind: "platform", TargetRole: "public", Scopes: []string{"internal_test"}}, true},
		{"another source owner", Rule{SourceOwner: "legacy-composition", TargetOwnerKind: "domain", TargetRole: "compose", Scopes: everyScope}, false},
		{"a domain owner as source", Rule{SourceOwner: "workforce", TargetOwnerKind: "domain", TargetRole: "compose", Scopes: everyScope}, false},
		{"the composition kind as source", Rule{SourceOwnerKind: "composition", TargetOwnerKind: "domain", TargetRole: "compose", Scopes: everyScope}, false},
		{"no source selector", Rule{TargetOwnerKind: "domain", TargetRole: "compose", Scopes: everyScope}, false},
		{"a tool owner kind", Rule{SourceOwner: "root-composition", TargetOwnerKind: "tool", TargetRole: "http", Scopes: everyScope}, false},
		{"a named tool owner", Rule{SourceOwner: "root-composition", TargetOwner: "dev-tools", TargetRole: "http", Scopes: []string{"production"}}, false},
		{"the composition kind", Rule{SourceOwner: "root-composition", TargetOwnerKind: "composition", TargetRole: "compose", Scopes: everyScope}, false},
		{"the legacy composition", Rule{SourceOwner: "root-composition", TargetOwner: "legacy-composition", TargetRole: "compose", Scopes: []string{"production"}}, false},
		{"the shared fixtures", Rule{SourceOwner: "root-composition", TargetOwner: "test-support", TargetRole: "test-support", Scopes: reachTestScopes}, false},
		{"the migration kind", Rule{SourceOwner: "root-composition", TargetOwnerKind: "migration", TargetRole: "migration", Scopes: everyScope}, false},
		{"another role of the migrations", Rule{SourceOwner: "root-composition", TargetOwner: "migrations", TargetRole: "test-support", Scopes: everyScope}, false},
		{"an application role", Rule{SourceOwner: "root-composition", TargetOwnerKind: "domain", TargetRole: "application", Scopes: everyScope}, false},
		{"a domain role", Rule{SourceOwner: "root-composition", TargetOwnerKind: "platform", TargetRole: "domain", Scopes: everyScope}, false},
		{"an adapter role", Rule{SourceOwner: "root-composition", TargetOwner: "workforce", TargetRole: "adapter", Scopes: []string{"production"}}, false},
		{"a postgres role", Rule{SourceOwner: "root-composition", TargetOwnerKind: "platform", TargetRole: "postgres", Scopes: []string{"production"}}, false},
		{"another owner's cli role", Rule{SourceOwner: "root-composition", TargetOwnerKind: "domain", TargetRole: "cli", Scopes: []string{"production"}}, false},
		{"same_owner with another target owner", Rule{SourceOwner: "root-composition", SameOwner: true, TargetOwner: "workforce", TargetRole: "compose", Scopes: everyScope}, false},
		{"same_owner with an owner kind", Rule{SourceOwner: "root-composition", SameOwner: true, TargetOwnerKind: "domain", TargetRole: "compose", Scopes: everyScope}, false},
		{"an external class", Rule{SourceOwner: "root-composition", TargetClass: "orm-sql", Scopes: everyScope}, false},
		{"no scope", Rule{SourceOwner: "root-composition", TargetOwnerKind: "domain", TargetRole: "compose"}, false},
	} {
		if got := reviewedRootCompositionRule(scenario.rule, reachOwners); got != scenario.want {
			t.Errorf("%s: reviewedRootCompositionRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func TestReviewedOwnerCompositionRuleShape(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"domain", "platform", "workflow", "projection", "tool"} {
		for _, role := range sortedAllowedRoles() {
			rule := Rule{SourceOwnerKind: kind, SourceRole: "compose", SameOwner: true, TargetRole: role, Scopes: []string{"production"}}
			if !reviewedOwnerCompositionRule(rule, reachOwners) {
				t.Errorf("rejected the composition of a %s owner reaching its own %s role", kind, role)
			}
		}
	}
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"a named owner", Rule{SourceOwner: "workforce", SourceRole: "compose", SameOwner: true, TargetRole: "adapter", Scopes: []string{"production"}}, true},
		{"its own owner named on both sides", Rule{SourceOwner: "workforce", SourceRole: "compose", TargetOwner: "workforce", TargetRole: "postgres", Scopes: []string{"production"}}, true},
		{"no source selector", Rule{SourceRole: "compose", SameOwner: true, TargetRole: "adapter", Scopes: []string{"production"}}, false},
		{"the composition kind", Rule{SourceOwnerKind: "composition", SourceRole: "compose", SameOwner: true, TargetRole: "domain", Scopes: []string{"production"}}, false},
		{"the legacy composition", Rule{SourceOwner: "legacy-composition", SourceRole: "compose", SameOwner: true, TargetRole: "domain", Scopes: []string{"production"}}, false},
		{"the root", Rule{SourceOwner: "root-composition", SourceRole: "compose", SameOwner: true, TargetRole: "compose", Scopes: []string{"production"}}, false},
		{"another owner", Rule{SourceOwner: "workforce", SourceRole: "compose", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"an owner kind as target", Rule{SourceOwnerKind: "domain", SourceRole: "compose", TargetOwnerKind: "domain", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"another source role", Rule{SourceOwnerKind: "domain", SourceRole: "application", SameOwner: true, TargetRole: "postgres", Scopes: []string{"production"}}, false},
		{"no source role", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "postgres", Scopes: []string{"production"}}, false},
		{"a test scope", Rule{SourceOwnerKind: "domain", SourceRole: "compose", SameOwner: true, TargetRole: "adapter", Scopes: []string{"production", "internal_test"}}, false},
		{"test scopes only", Rule{SourceOwnerKind: "domain", SourceRole: "compose", SameOwner: true, TargetRole: "adapter", Scopes: reachTestScopes}, false},
		{"an external class", Rule{SourceOwnerKind: "domain", SourceRole: "compose", TargetClass: "orm-sql", Scopes: []string{"production"}}, false},
		{"no scope", Rule{SourceOwnerKind: "domain", SourceRole: "compose", SameOwner: true, TargetRole: "adapter"}, false},
	} {
		if got := reviewedOwnerCompositionRule(scenario.rule, reachOwners); got != scenario.want {
			t.Errorf("%s: reviewedOwnerCompositionRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func TestReviewedOwnTestRuleShape(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"domain", "platform", "workflow", "projection", "tool", "migration", "test-support"} {
		for _, role := range []string{"public", "contract", "compose", "port", "adapter", "test-support"} {
			rule := Rule{SourceOwnerKind: kind, SameOwner: true, TargetRole: role, Scopes: reachTestScopes}
			if !reviewedOwnTestRule(rule) {
				t.Errorf("rejected the tests of a %s owner importing its own %s role", kind, role)
			}
		}
		for _, role := range []string{"public", "contract", "compose", "port", "adapter"} {
			rule := Rule{SourceOwnerKind: kind, SourceRole: "test-support", SameOwner: true, TargetRole: role, Scopes: []string{"production"}}
			if !reviewedOwnTestRule(rule) {
				t.Errorf("rejected the test support of a %s owner importing its own %s role", kind, role)
			}
		}
	}
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"a source role", Rule{SourceOwnerKind: "domain", SourceRole: "module-behavior-test", SameOwner: true, TargetRole: "public", Scopes: []string{"external_test"}}, true},
		{"a named owner", Rule{SourceOwner: "workforce", SourceRole: "e2e-test", SameOwner: true, TargetRole: "adapter", Scopes: reachTestScopes}, true},
		{"its own owner named on both sides", Rule{SourceOwner: "workforce", SourceRole: "e2e-test", TargetOwner: "workforce", TargetRole: "public", Scopes: reachTestScopes}, true},
		{"every owner", Rule{SameOwner: true, TargetRole: "public", Scopes: reachTestScopes}, true},
		{"the legacy composition", Rule{SourceOwner: "legacy-composition", SameOwner: true, TargetRole: "compose", Scopes: reachTestScopes}, true},
		{"another owner", Rule{SourceOwner: "workforce", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: reachTestScopes}, false},
		{"an owner kind as target", Rule{SourceOwnerKind: "domain", TargetOwnerKind: "domain", TargetRole: "public", Scopes: reachTestScopes}, false},
		{"an application role", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "application", Scopes: reachTestScopes}, false},
		{"a domain role", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "domain", Scopes: reachTestScopes}, false},
		{"a postgres role", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "postgres", Scopes: reachTestScopes}, false},
		{"an http role", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "http", Scopes: reachTestScopes}, false},
		{"a test role in production", Rule{SourceOwnerKind: "domain", SourceRole: "e2e-test", SameOwner: true, TargetRole: "public", Scopes: []string{"production"}}, false},
		{"no source role in production", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "public", Scopes: []string{"production"}}, false},
		{"every scope", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "public", Scopes: everyScope}, false},
		{"test support across scopes", Rule{SourceOwnerKind: "domain", SourceRole: "test-support", SameOwner: true, TargetRole: "public", Scopes: []string{"production", "internal_test"}}, false},
		{"test support into its own test support", Rule{SourceOwnerKind: "domain", SourceRole: "test-support", SameOwner: true, TargetRole: "test-support", Scopes: []string{"production"}}, false},
		{"test support into its application", Rule{SourceOwnerKind: "domain", SourceRole: "test-support", SameOwner: true, TargetRole: "application", Scopes: []string{"production"}}, false},
		{"an external class", Rule{SourceOwnerKind: "domain", TargetClass: "orm-sql", Scopes: reachTestScopes}, false},
		{"no scope", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetRole: "public"}, false},
	} {
		if got := reviewedOwnTestRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedOwnTestRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func TestReviewedRootTestRuleShape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"the tests of a domain owner", Rule{SourceOwnerKind: "domain", TargetOwner: "root-composition", TargetRole: "compose", Scopes: reachTestScopes}, true},
		{"the shared fixtures", Rule{SourceOwnerKind: "test-support", TargetOwner: "root-composition", TargetRole: "compose", Scopes: reachTestScopes}, true},
		{"one suite", Rule{SourceOwner: "workforce", SourceRole: "e2e-test", TargetOwner: "root-composition", TargetRole: "compose", Scopes: []string{"external_test"}}, true},
		{"every test package", Rule{TargetOwner: "root-composition", TargetRole: "compose", Scopes: reachTestScopes}, true},
		{"the legacy composition's external tests", Rule{SourceOwner: "legacy-composition", SourceRole: "module-behavior-test", TargetOwner: "root-composition", TargetRole: "compose", Scopes: []string{"external_test"}}, true},
		{"production", Rule{SourceOwnerKind: "domain", TargetOwner: "root-composition", TargetRole: "compose", Scopes: []string{"production"}}, false},
		{"every scope", Rule{SourceOwnerKind: "domain", TargetOwner: "root-composition", TargetRole: "compose", Scopes: everyScope}, false},
		{"the commands root", Rule{SourceOwnerKind: "domain", TargetOwner: "root-composition", TargetRole: "cli", Scopes: reachTestScopes}, false},
		{"the legacy composition as target", Rule{SourceOwnerKind: "domain", TargetOwner: "legacy-composition", TargetRole: "compose", Scopes: reachTestScopes}, false},
		{"the composition kind as target", Rule{SourceOwnerKind: "domain", TargetOwnerKind: "composition", TargetRole: "compose", Scopes: reachTestScopes}, false},
		{"same_owner", Rule{SourceOwnerKind: "domain", SameOwner: true, TargetOwner: "root-composition", TargetRole: "compose", Scopes: reachTestScopes}, false},
		{"an external class", Rule{SourceOwnerKind: "domain", TargetClass: "http-router", Scopes: reachTestScopes}, false},
		{"no scope", Rule{SourceOwnerKind: "domain", TargetOwner: "root-composition", TargetRole: "compose"}, false},
	} {
		if got := reviewedRootTestRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedRootTestRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

// compositionReachPolicies returns the repository policy as the candidate and
// a base one epoch older without the rules of the four reaches. Loading the
// candidate fails when one of its rules overlaps another.
func compositionReachPolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	candidate := repositoryPolicy(t)
	owners := ownersByID(candidate)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), func(rule Rule) bool {
		return reviewedCompositionReachRule(rule, owners)
	})
	if len(base.Rules) == len(candidate.Rules) {
		t.Fatal("the repository policy carries no rule of the ADR 0053 reaches")
	}
	return &base, candidate
}

func TestCompositionReachRulesPassOnlyInAReviewedEpoch(t *testing.T) {
	t.Parallel()
	base, candidate := compositionReachPolicies(t)
	if err := comparePolicyStrictness(base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed composition reaches were rejected: %v", err)
	}

	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	err := comparePolicyStrictness(base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil)
	if err == nil {
		t.Fatal("the composition reaches passed without an epoch increase")
	}
	for _, id := range []string{"root-composition.reach.domain.compose", "owner-compose.domain.adapter", "own-test.domain.compose", "own-test-support.domain.public", "root-test.domain.compose"} {
		if !strings.Contains(err.Error(), "rule "+id+" ") {
			t.Errorf("rule %s passed without an epoch increase: %v", id, err)
		}
	}
}

func TestCompositionReachRulesCarryNoIssue(t *testing.T) {
	t.Parallel()
	policy := repositoryPolicy(t)
	owners := ownersByID(policy)
	for _, rule := range policy.Rules {
		if reviewedCompositionReachRule(rule, owners) && rule.Issue != "" {
			t.Errorf("rule %s is a standing reach of ADR 0053 but carries the temporary issue %s", rule.ID, rule.Issue)
		}
	}
}

func TestCompositionReachEpochGrantsNothingElse(t *testing.T) {
	t.Parallel()
	base, candidate := compositionReachPolicies(t)
	for _, extra := range []Rule{
		{ID: "root.workforce-application", Description: "test", Scopes: []string{"production"}, SourceOwner: "root-composition", TargetOwner: "workforce", TargetRole: "application"},
		{ID: "root.platform-postgres", Description: "test", Scopes: []string{"production"}, SourceOwner: "root-composition", TargetOwnerKind: "platform", TargetRole: "postgres"},
		{ID: "root.legacy-composition", Description: "test", Scopes: []string{"production"}, SourceOwner: "root-composition", TargetOwner: "legacy-composition", TargetRole: "compose"},
		{ID: "root.tool-http", Description: "test", Scopes: []string{"production"}, SourceOwner: "root-composition", TargetOwnerKind: "tool", TargetRole: "http"},
		{ID: "compose.cross-owner", Description: "test", Scopes: []string{"production"}, SourceOwner: "workforce", SourceRole: "compose", TargetOwner: "care-plan", TargetRole: "public"},
		{ID: "compose.legacy-composition", Description: "test", Scopes: []string{"production"}, SourceOwner: "legacy-composition", SourceRole: "compose", SameOwner: true, TargetRole: "domain"},
		{ID: "compose.own-owner-in-tests", Description: "test", Scopes: []string{"internal_test"}, SourceOwner: "workforce", SourceRole: "compose", SameOwner: true, TargetRole: "postgres"},
		{ID: "own-test.application", Description: "test", Scopes: []string{"external_test"}, SourceOwnerKind: "domain", SourceRole: "e2e-test", SameOwner: true, TargetRole: "application"},
		{ID: "own-test.http", Description: "test", Scopes: []string{"internal_test"}, SourceOwnerKind: "domain", SourceRole: "module-internal-test", SameOwner: true, TargetRole: "http"},
		{ID: "own-test.foreign-adapter", Description: "test", Scopes: []string{"external_test"}, SourceOwner: "care-plan", SourceRole: "e2e-test", TargetOwner: "workforce", TargetRole: "adapter"},
		{ID: "root-test.production", Description: "test", Scopes: []string{"production"}, SourceOwnerKind: "domain", SourceRole: "test-support", TargetOwner: "root-composition", TargetRole: "compose"},
		{ID: "root-test.cli", Description: "test", Scopes: []string{"external_test"}, SourceOwnerKind: "domain", SourceRole: "module-behavior-test", TargetOwner: "root-composition", TargetRole: "cli"},
	} {
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), extra)
		problems := strings.Join(ruleLoosenings(base, &widened, noCreated()), "\n")
		if !strings.Contains(problems, "rule "+extra.ID+" ") {
			t.Errorf("%s escaped the guard: %s", extra.ID, problems)
		}
	}
}

func TestFirstPartyImportsAllowedByTheCompositionReaches(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	packages := candidate.packageMap()
	classified := func(path string) Package {
		t.Helper()
		pkg, ok := packages[candidate.absolutePackage(path)]
		if !ok {
			t.Fatalf("package %s is not classified", path)
		}
		return pkg
	}
	for _, scenario := range []struct {
		name           string
		scope          Scope
		source, target string
		want           bool
	}{
		{"the root composes an owner", ScopeProduction, "api", "modules/workforce/compose", true},
		{"an owner's suite uses its own contract", ScopeExternalTest, "modules/identityaccess/behavior", "modules/identityaccess", true},
		{"a root rule into a tool owner", ScopeProduction, "api", "demoprofile", false},
		{"an implementation role of another owner", ScopeProduction, "api", "modules/workforce/internal/adapters/substitutions", false},
		{"the legacy composition's own domain", ScopeProduction, "database/repositories/studentintegration", "models", false},
	} {
		got := firstPartyAllowedByReviewedCompositionReach(&base, candidate, scenario.scope, classified(scenario.source), classified(scenario.target))
		if got != scenario.want {
			t.Errorf("%s: firstPartyAllowedByReviewedCompositionReach = %v, want %v", scenario.name, got, scenario.want)
		}
	}
	if firstPartyAllowedByReviewedCompositionReach(candidate, candidate, ScopeProduction, classified("api"), classified("modules/workforce/compose")) {
		t.Error("a root import passed without an epoch increase")
	}
}
