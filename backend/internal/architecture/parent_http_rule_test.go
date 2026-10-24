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
	if len(candidate.Rules)-len(base.Rules) != 3 {
		t.Fatal("expected three standing guardian portal HTTP rules")
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed guardian portal HTTP rules: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("new guardian portal HTTP permissions passed without an epoch")
	}
}

func TestParentHTTPEpochGrantsOnlyTheNamedRuntimes(t *testing.T) {
	t.Parallel()
	for _, rule := range []Rule{
		{ID: "parent.application", Description: "test", SourceOwner: "parent-portal", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "application", Scopes: []string{"production"}},
		{ID: "parent.orm", Description: "test", SourceOwner: "parent-portal", SourceRole: "http", TargetClass: "orm-sql", Scopes: []string{"production"}},
		{ID: "parent.models", Description: "test", SourceOwner: "parent-portal", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "domain", Scopes: []string{"production"}},
		{ID: "parent.composition", Description: "test", SourceOwner: "parent-portal", SourceRole: "adapter-test", TargetOwner: "legacy-composition", TargetRole: "compose", Scopes: []string{"external_test"}},
		{ID: "other.http", Description: "test", SourceOwner: "other-tool", SourceRole: "http", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}},
	} {
		candidate := repositoryPolicy(t)
		// Keep the unrelated owner independent of production owner retirements.
		candidate.Owners = append(candidate.Owners, Owner{ID: "other-tool", Kind: "tool"})
		base := *candidate
		base.PolicyEpoch--
		widened := *candidate
		widened.Rules = append(slices.Clone(candidate.Rules), rule)
		err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), nil)
		if err == nil || !strings.Contains(err.Error(), rule.ID) {
			t.Fatalf("%s escaped the guard: %v", rule.ID, err)
		}
	}
}

func TestParentHTTPRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	original := Rule{SourceOwner: "parent-portal", SourceRole: "http", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}
	for _, mutate := range []func(*Rule){
		func(r *Rule) { r.SourceOwner = "" },
		func(r *Rule) { r.SourceOwner = "tenant-runtime" },
		func(r *Rule) { r.SourceOwnerKind = "tool" },
		func(r *Rule) { r.SourceRole = "" },
		func(r *Rule) { r.TargetOwnerKind = "tool" },
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
	sessionTest := Rule{SourceOwner: "parent-portal", SourceRole: "adapter-test", TargetOwner: "identity-access", TargetRole: "http", Scopes: []string{"internal_test", "external_test"}}
	privateSession := sessionTest
	privateSession.TargetRole = "adapter"
	if reviewedParentHTTPRule(privateSession) {
		t.Fatal("accepted the session runtime in Identity's private adapter role")
	}
	if !reviewedParentHTTPRule(original) || !reviewedParentHTTPRule(sessionTest) {
		t.Fatal("standing guardian portal HTTP or session-test rule was rejected")
	}
	sessionTest.Scopes = everyScope
	if reviewedParentHTTPRule(sessionTest) {
		t.Fatal("session test rule gained production access")
	}
}
