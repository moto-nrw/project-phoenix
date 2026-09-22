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
		{"calendar date for the e2e role", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "legacy-shared", TargetRole: "domain", Scopes: []string{"internal_test"}}, true},
		{"permission constants", Rule{SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "security-runtime", TargetRole: "contract", Scopes: []string{"internal_test", "external_test"}}, true},
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
		return strings.HasPrefix(rule.ID, "shared-fixtures.")
	})
	if len(base.Rules) == len(candidate.Rules) {
		t.Fatal("the repository policy carries no shared-fixtures rules")
	}
	return &base, candidate
}

func noCreated() map[string]struct{} { return map[string]struct{}{} }

func TestSharedFixtureRulesPassOnlyInAReviewedEpoch(t *testing.T) {
	t.Parallel()
	base, candidate := sharedFixturePolicies(t)
	if err := comparePolicyStrictness(base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err != nil {
		t.Fatalf("reviewed shared-fixture epoch was rejected: %v", err)
	}

	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	err := comparePolicyStrictness(base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
	if err == nil || !strings.Contains(err.Error(), "rule shared-fixtures.") {
		t.Fatalf("shared-fixture rules passed without an epoch increase: %v", err)
	}
}

func TestSharedFixtureEpochGrantsNothingElse(t *testing.T) {
	t.Parallel()
	for _, extra := range []Rule{
		{ID: "shared-fixtures.people-domain", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "people-directory", TargetRole: "domain"},
		{ID: "shared-fixtures.tenant-postgres", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "postgres"},
		{ID: "shared-fixtures.e2e-in-production", Description: "test", Scopes: []string{"production"}, SourceOwner: "test-support", SourceRole: "e2e-test", TargetOwner: "tenant-runtime", TargetRole: "public"},
		{ID: "shared-fixtures.foreign-owner", Description: "test", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "test-support", TargetOwner: "tenant-runtime", TargetRole: "public"},
	} {
		base, candidate := sharedFixturePolicies(t)
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), extra)
		err := comparePolicyStrictness(base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
		if err == nil || !strings.Contains(err.Error(), "rule "+extra.ID) {
			t.Fatalf("%s: the reviewed epoch admitted a rule outside the shared-fixture shape: %v", extra.ID, err)
		}
	}
}
