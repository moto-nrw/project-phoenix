package architecture

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const sharedKernelRuleID = "shared-kernel.contract"

var everyScope = []string{"production", "internal_test", "external_test"}

func TestReviewedSharedKernelRuleShape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"every role in every scope", Rule{TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: everyScope}, true},
		{"scopes in another order", Rule{TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: []string{"external_test", "production", "internal_test"}}, true},
		{"production only", Rule{TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: []string{"production"}}, false},
		{"test scopes only", Rule{TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: []string{"internal_test", "external_test"}}, false},
		{"a source owner", Rule{SourceOwner: "enrollment", TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: everyScope}, false},
		{"a source owner kind", Rule{SourceOwnerKind: "domain", TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: everyScope}, false},
		{"a source role", Rule{SourceRole: "postgres", TargetOwner: "shared-kernel", TargetRole: "contract", Scopes: everyScope}, false},
		{"the retained date wrapper", Rule{TargetOwner: "legacy-shared", TargetRole: "domain", Scopes: everyScope}, false},
		{"the shared HTTP runtime", Rule{TargetOwner: "inbound-common", TargetRole: "http", Scopes: everyScope}, false},
		{"a target owner kind", Rule{TargetOwnerKind: "kernel", TargetRole: "contract", Scopes: everyScope}, false},
		{"another kernel role", Rule{TargetOwner: "shared-kernel", TargetRole: "domain", Scopes: everyScope}, false},
		{"same owner flag", Rule{TargetOwner: "shared-kernel", TargetRole: "contract", SameOwner: true, Scopes: everyScope}, false},
		{"external class", Rule{TargetClass: "utility", Scopes: everyScope}, false},
	} {
		if got := reviewedSharedKernelRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedSharedKernelRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func repositoryPolicy(t *testing.T) *Policy {
	t.Helper()
	policy, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// sharedKernelPolicies returns the repository policy as the candidate and a
// base one epoch older without the shared-kernel rule, so the comparison runs
// against the real owner map and package classification.
func sharedKernelPolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), func(rule Rule) bool {
		return rule.ID == sharedKernelRuleID
	})
	if len(base.Rules) == len(candidate.Rules) {
		t.Fatalf("the repository policy carries no %s rule", sharedKernelRuleID)
	}
	return &base, candidate
}

func TestSharedKernelRulePassesOnlyInAReviewedEpoch(t *testing.T) {
	t.Parallel()
	base, candidate := sharedKernelPolicies(t)
	if err := comparePolicyStrictness(base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err != nil {
		t.Fatalf("reviewed shared-kernel epoch was rejected: %v", err)
	}

	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	err := comparePolicyStrictness(base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
	if err == nil || !strings.Contains(err.Error(), "rule "+sharedKernelRuleID+" ") {
		t.Fatalf("the shared-kernel rule passed without an epoch increase: %v", err)
	}
}

func TestSharedKernelEpochGrantsNothingElse(t *testing.T) {
	t.Parallel()
	for _, extra := range []Rule{
		{ID: "every-role.legacy-date", Description: "test", Scopes: everyScope, TargetOwner: "legacy-shared", TargetRole: "domain"},
		{ID: "every-role.inbound-common", Description: "test", Scopes: everyScope, TargetOwner: "inbound-common", TargetRole: "http"},
		{ID: "every-role.domain-contracts", Description: "test", Scopes: everyScope, TargetOwnerKind: "domain", TargetRole: "contract"},
	} {
		base, candidate := sharedKernelPolicies(t)
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), extra)
		err := comparePolicyStrictness(base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
		if err == nil || !strings.Contains(err.Error(), "rule "+extra.ID+" ") {
			t.Fatalf("%s: the reviewed epoch admitted a rule outside the shared-kernel shape: %v", extra.ID, err)
		}
	}
}

// The kernel's own tests reach its contract through the shared-kernel rule
// alone: a same_owner rule never selects the kernel, so the two cannot overlap
// statically or on an edge, while every other owner keeps its same_owner grant.
func TestSameOwnerRulesDoNotSelectTheKernel(t *testing.T) {
	t.Parallel()
	policy := repositoryPolicy(t)
	sameOwner := Rule{ID: "same-owner.contract", Description: "test", Scopes: []string{"internal_test"}, SourceRole: "module-internal-test", SameOwner: true, TargetRole: "contract"}
	kernelTest := Package{Owner: "shared-kernel", Role: "module-internal-test"}
	kernel := Package{Owner: "shared-kernel", Role: "contract"}
	if policy.matchesTarget(sameOwner, kernelTest, kernel) {
		t.Error("a same_owner rule selected the shared kernel")
	}
	if !policy.matchesTarget(sameOwner, Package{Owner: "care-plan", Role: "module-internal-test"}, Package{Owner: "care-plan", Role: "contract"}) {
		t.Error("a same_owner rule no longer selects an ordinary owner")
	}

	decision := decideRules(policy.firstPartyRules(ScopeInternalTest, kernelTest, kernel))
	if decision.Allowed == nil || decision.Allowed.ID != sharedKernelRuleID || len(decision.Overlaps) != 0 {
		t.Fatalf("kernel test -> kernel contract decision = %+v, want only %s", decision, sharedKernelRuleID)
	}

	kernelRule := Rule{ID: sharedKernelRuleID, Description: "test", Scopes: everyScope, TargetOwner: "shared-kernel", TargetRole: "contract"}
	if scope, overlaps := overlappingScope(sameOwner, kernelRule, ownersByID(policy)); overlaps {
		t.Fatalf("the same_owner contract rule overlaps the shared-kernel rule in %s", scope)
	}
	domainRule := Rule{ID: "every-role.care-plan", Description: "test", Scopes: everyScope, TargetOwner: "care-plan", TargetRole: "contract"}
	if _, overlaps := overlappingScope(sameOwner, domainRule, ownersByID(policy)); !overlaps {
		t.Fatal("the kernel exception also hid a same_owner overlap on an ordinary owner")
	}
}
