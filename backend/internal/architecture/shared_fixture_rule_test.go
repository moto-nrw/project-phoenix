package architecture

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReviewedSharedFixtureRuleShape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"own tooling", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "test-support", TargetRole: "test-support", Scopes: []string{"production"}}, true},
		{"tenant runtime", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, true},
		{"calendar-date wrapper for the e2e role", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "legacy-shared", TargetRole: "domain", Scopes: []string{"internal_test"}}, false},
		{"permission constants", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "security-runtime", TargetRole: "contract", Scopes: []string{"internal_test", "external_test"}}, true},
		{"security contract", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"production"}}, true},
		{"settings contract", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "settings-platform", TargetRole: "public", Scopes: []string{"production"}}, true},
		{"settings contract for the e2e role", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "settings-platform", TargetRole: "public", Scopes: []string{"internal_test"}}, true},
		{"transaction runtime bootstrap", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "transaction-runtime", TargetRole: "compose", Scopes: []string{"production"}}, true},
		{"session token runtime", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "identity-access", TargetRole: "http", Scopes: []string{"production"}}, true},
		{"session token runtime for the e2e role", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "identity-access", TargetRole: "http", Scopes: []string{"internal_test", "external_test"}}, true},
		{"identity adapter", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "identity-access", TargetRole: "adapter", Scopes: []string{"production"}}, false},
		{"identity application", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "identity-access", TargetRole: "application", Scopes: []string{"production"}}, false},
		{"identity contract", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "identity-access", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"another owner's adapter", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "workforce", TargetRole: "adapter", Scopes: []string{"production"}}, false},
		{"transaction runtime bootstrap for the e2e role", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "transaction-runtime", TargetRole: "compose", Scopes: []string{"internal_test"}}, false},
		{"transaction runtime bootstrap in a test scope", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "transaction-runtime", TargetRole: "compose", Scopes: []string{"production", "internal_test"}}, false},
		{"transaction adapter", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "transaction-runtime", TargetRole: "postgres", Scopes: []string{"production"}}, false},
		{"transaction application", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "transaction-runtime", TargetRole: "application", Scopes: []string{"production"}}, false},
		{"audit contract", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "audit-platform", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"another owner's composition", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "audit-platform", TargetRole: "compose", Scopes: []string{"production"}}, false},
		{"composition by owner kind", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwnerKind: "platform", TargetRole: "compose", Scopes: []string{"production"}}, false},
		{"settings application", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "settings-platform", TargetRole: "application", Scopes: []string{"production"}}, false},
		{"settings repository", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "settings-platform", TargetRole: "postgres", Scopes: []string{"internal_test"}}, false},
		{"no role", Rule{SourceOwner: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"test-support role in a test scope", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"internal_test"}}, false},
		{"e2e role in production", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"another owner", Rule{SourceOwner: "care-plan", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"owner-agnostic", Rule{SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"owner kind", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwnerKind: "platform", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"foreign role of the fixture owner", Rule{SourceOwner: "test-support", SourceRole: "adapter-test", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"internal_test"}}, false},
		{"a dissolving domain", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "people-directory", TargetRole: "domain", Scopes: []string{"production"}}, false},
		{"implementation role of a granted owner", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "postgres", Scopes: []string{"production"}}, false},
		{"authorization application", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "security-runtime", TargetRole: "application", Scopes: []string{"internal_test"}}, false},
		{"external class", Rule{SourceOwner: "test-support", SourceRole: "test-support", TargetClass: "crypto", Scopes: []string{"production"}}, false},
		{"same owner flag", Rule{SourceOwner: "test-support", SourceRole: "test-support", SameOwner: true, TargetRole: "test-support", Scopes: []string{"production"}}, false},
	} {
		if got := reviewedSharedFixtureRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedSharedFixtureRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

// sharedFixturePolicies returns the repository policy as the candidate and a
// base one epoch older without the shared-fixture rules, so the comparison
// runs against the real owner map and package classification.
func sharedFixturePolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	candidate, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), func(rule Rule) bool {
		return strings.HasPrefix(rule.ID, "shared-fixtures.") || ownerSuiteRuleIDs[rule.ID]
	})
	if len(base.Rules) == len(candidate.Rules) {
		t.Fatal("the repository policy carries no shared-fixtures rules")
	}
	return &base, candidate
}

// ownerSuiteRuleIDs are the repository rules registered under the pinned
// owner-suite shapes of ADR 0047.
var ownerSuiteRuleIDs = map[string]bool{
	"settings-platform.internal-test.security-runtime-public": true,
}

func TestReviewedOwnerSuiteRuleShape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"settings suite decides through the security contract", Rule{SourceOwner: "settings-platform", SourceRole: "module-internal-test", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"internal_test"}}, true},
		{"audit suite composes Enrollment (platform into a domain, ADR 0057)", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "enrollment", TargetRole: "test-support", Scopes: []string{"external_test"}}, false},
		{"audit suite composes Care Plan (platform into a domain, ADR 0057)", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "test-support", Scopes: []string{"external_test"}}, false},
		{"wider scope", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "enrollment", TargetRole: "test-support", Scopes: []string{"internal_test", "external_test"}}, false},
		{"production scope", Rule{SourceOwner: "settings-platform", SourceRole: "module-internal-test", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"production"}}, false},
		{"another source role", Rule{SourceOwner: "settings-platform", SourceRole: "application", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"internal_test"}}, false},
		{"another source owner", Rule{SourceOwner: "workforce", SourceRole: "adapter-test", TargetOwner: "enrollment", TargetRole: "test-support", Scopes: []string{"external_test"}}, false},
		{"another target role", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "enrollment", TargetRole: "domain", Scopes: []string{"external_test"}}, false},
		{"security application", Rule{SourceOwner: "settings-platform", SourceRole: "module-internal-test", TargetOwner: "security-runtime", TargetRole: "application", Scopes: []string{"internal_test"}}, false},
		{"owner-agnostic source", Rule{SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "test-support", Scopes: []string{"external_test"}}, false},
		{"owner kind target", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwnerKind: "domain", TargetRole: "test-support", Scopes: []string{"external_test"}}, false},
		{"no scope", Rule{SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "test-support"}, false},
	} {
		if got := reviewedOwnerSuiteRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedOwnerSuiteRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func noCreated() map[string]struct{} { return map[string]struct{}{} }

func TestSharedFixtureRulesPassOnlyInAReviewedEpoch(t *testing.T) {
	t.Parallel()
	base, candidate := sharedFixturePolicies(t)
	if err := comparePolicyStrictness(base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed shared-fixture epoch was rejected: %v", err)
	}

	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	err := comparePolicyStrictness(base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil)
	if err == nil || !strings.Contains(err.Error(), "rule shared-fixtures.") {
		t.Fatalf("shared-fixture rules passed without an epoch increase: %v", err)
	}
	if !strings.Contains(err.Error(), "rule shared-fixtures.transaction-runtime-compose ") {
		t.Fatalf("the transaction runtime bootstrap rule passed without an epoch increase: %v", err)
	}
	for id := range ownerSuiteRuleIDs {
		if !strings.Contains(err.Error(), "rule "+id) {
			t.Fatalf("owner-suite rule %s passed without an epoch increase: %v", id, err)
		}
	}
}

func TestSharedFixtureEpochGrantsNothingElse(t *testing.T) {
	t.Parallel()
	for _, extra := range []Rule{
		{ID: "shared-fixtures.people-domain", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "people-directory", TargetRole: "domain"},
		{ID: "shared-fixtures.tenant-postgres", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "postgres"},
		{ID: "shared-fixtures.e2e-in-production", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "tenant-runtime", TargetRole: "public"},
		{ID: "shared-fixtures.settings-application", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "settings-platform", TargetRole: "application"},
		{ID: "shared-fixtures.transaction-adapter", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "transaction-runtime", TargetRole: "postgres"},
		{ID: "shared-fixtures.e2e.transaction-bootstrap", Description: "test", Scopes: []string{"internal_test"}, SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "transaction-runtime", TargetRole: "compose"},
		{ID: "shared-fixtures.audit-contract", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "audit-platform", TargetRole: "public"},
		{ID: "owner-suite.audit-enrollment-domain", Description: "test", Scopes: []string{"external_test"}, SourceOwner: "audit-platform", SourceRole: "adapter-test", TargetOwner: "enrollment", TargetRole: "domain"},
		{ID: "owner-suite.settings-security-production", Description: "test", Scopes: []string{"production"}, SourceOwner: "settings-platform", SourceRole: "application", TargetOwner: "security-runtime", TargetRole: "public"},
		{ID: "shared-fixtures.foreign-owner", Description: "test", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public"},
	} {
		base, candidate := sharedFixturePolicies(t)
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), extra)
		err := comparePolicyStrictness(base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), nil)
		if err == nil || !strings.Contains(err.Error(), "rule "+extra.ID) {
			t.Fatalf("%s: the reviewed epoch admitted a rule outside the shared-fixture shape: %v", extra.ID, err)
		}
	}
}

// Even an old policy permission cannot authorize shared owner construction.
func TestSharedFixtureCompositionGuard(t *testing.T) {
	t.Parallel()
	packages := map[string]Package{
		"test":           {Owner: "test-support", Role: "test-support"},
		"enrollmenttest": {Owner: "enrollment", Role: "test-support"},
		"presence":       {Owner: "student-presence", Role: "compose"},
		"enrollment":     {Owner: "enrollment", Role: "compose"},
		"bootstrap":      {Owner: "transaction-runtime", Role: "compose"},
		"root":           {Owner: "root-composition", Role: "compose"},
	}
	cases := []struct {
		name  string
		edges []Edge
		count int
	}{
		{"formerly permitted presence", []Edge{{ScopeProduction, "test", "presence"}}, 1},
		{"bootstrap", []Edge{{ScopeProduction, "test", "bootstrap"}}, 0},
		{"owner-local support not shared", []Edge{{ScopeProduction, "enrollmenttest", "enrollment"}}, 0},
		{"suite builds root", []Edge{{ScopeInternalTest, "test", "root"}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := sharedFixtureCompositionViolations(packages, &Graph{Edges: tc.edges})
			if len(violations) != tc.count {
				t.Fatalf("got %v, want %d violations", violations, tc.count)
			}
		})
	}
}

func TestSharedFixtureCompositionGuardOverridesPolicyPermission(t *testing.T) {
	t.Parallel()
	policy := &Policy{
		ModulePath: "fixture",
		Packages: []Package{
			{Path: "test", Owner: "test-support", Role: "test-support"},
			{Path: "presence", Owner: "student-presence", Role: "compose"},
		},
		Rules: []Rule{{ID: "test-support.to.student-presence-compose", Scopes: []string{"production"},
			SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "student-presence", TargetRole: "compose"}},
	}
	graph := &Graph{
		Packages: []string{"fixture/test", "fixture/presence"},
		Edges:    []Edge{{ScopeProduction, "fixture/test", "fixture/presence"}},
	}
	violations := Check(policy, graph)
	if len(violations) != 1 || violations[0].Rule != "imports.forbidden" {
		t.Fatalf("shared owner composition passed an explicit policy permission: %v", violations)
	}
}
