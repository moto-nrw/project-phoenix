package architecture

import (
	"slices"
	"strings"
	"testing"
)

func TestParentHTTPPermissionsRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedParentHTTPRule)
	if len(candidate.Rules)-len(base.Rules) != 4 {
		t.Fatal("expected four standing guardian portal HTTP rules")
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err != nil {
		t.Fatalf("reviewed guardian portal HTTP rules: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err == nil {
		t.Fatal("new guardian portal HTTP permissions passed without an epoch")
	}
}

func TestParentHTTPEpochGrantsOnlyTheNamedRuntimes(t *testing.T) {
	t.Parallel()
	for _, rule := range []Rule{
		{ID: "parent.application", Description: "test", SourceOwner: "inbound-parent", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "application", Scopes: []string{"production"}},
		{ID: "parent.orm", Description: "test", SourceOwner: "inbound-parent", SourceRole: "http", TargetClass: "orm-sql", Scopes: []string{"production"}},
		{ID: "parent.models", Description: "test", SourceOwner: "inbound-parent", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "domain", Scopes: []string{"production"}},
		{ID: "parent.composition", Description: "test", SourceOwner: "inbound-parent", SourceRole: "adapter-test", TargetOwner: "legacy-composition", TargetRole: "compose", Scopes: []string{"external_test"}},
		{ID: "other.http", Description: "test", SourceOwner: "inbound-mealplan", SourceRole: "http", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}},
	} {
		candidate := repositoryPolicy(t)
		base := *candidate
		base.PolicyEpoch--
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), rule)
		err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
		if err == nil || !strings.Contains(err.Error(), rule.ID) {
			t.Fatalf("%s escaped the guard: %v", rule.ID, err)
		}
	}
}

func TestParentHTTPRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	original := Rule{SourceOwner: "inbound-parent", SourceRole: "http", TargetOwner: "inbound-common", TargetRole: "http", Scopes: []string{"production"}}
	for _, mutate := range []func(*Rule){
		func(r *Rule) { r.SourceOwner = "" },
		func(r *Rule) { r.SourceOwner = "inbound-common" },
		func(r *Rule) { r.SourceOwnerKind = "inbound" },
		func(r *Rule) { r.SourceRole = "" },
		func(r *Rule) { r.TargetOwnerKind = "inbound" },
		func(r *Rule) { r.TargetRole = "adapter" },
		func(r *Rule) { r.TargetClass = "orm-sql" },
		func(r *Rule) { r.SameOwner = true },
		func(r *Rule) { r.Scopes = everyScope },
		func(r *Rule) { r.SourceRole = "adapter-test"; r.Scopes = []string{"internal_test", "external_test"} },
	} {
		broadened := original
		mutate(&broadened)
		if reviewedParentHTTPRule(broadened) {
			t.Fatalf("accepted broadened rule: %+v", broadened)
		}
	}
	sessionTest := Rule{SourceOwner: "inbound-parent", SourceRole: "adapter-test", TargetOwner: "identity-access", TargetRole: "adapter", Scopes: []string{"internal_test", "external_test"}}
	if !reviewedParentHTTPRule(original) || !reviewedParentHTTPRule(sessionTest) {
		t.Fatal("standing guardian portal HTTP or session-test rule was rejected")
	}
	sessionTest.Scopes = everyScope
	if reviewedParentHTTPRule(sessionTest) {
		t.Fatal("session test rule gained production access")
	}
}
