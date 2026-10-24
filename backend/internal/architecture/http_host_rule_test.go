package architecture

import (
	"slices"
	"strings"
	"testing"
)

func TestHTTPHostPermissionsRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedHTTPHostRule)
	if len(candidate.Rules)-len(base.Rules) != len(httpHostRules) {
		t.Fatalf("expected %d standing HTTP host rules, found %d", len(httpHostRules), len(candidate.Rules)-len(base.Rules))
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed HTTP host rules: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("new HTTP host permissions passed without an epoch")
	}
}

func TestHTTPHostEpochGrantsOnlyTheNamedPoints(t *testing.T) {
	t.Parallel()
	for _, rule := range []Rule{
		{ID: "root.scheduler-application", Description: "test", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "scheduler-runtime", TargetRole: "application", Scopes: []string{"production"}},
		{ID: "root.observability-adapter", Description: "test", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "observability", TargetRole: "adapter", Scopes: []string{"production"}},
		{ID: "root.orm", Description: "test", SourceOwner: "root-composition", SourceRole: "compose", TargetClass: "orm-sql", Scopes: []string{"production"}},
		{ID: "root.cli.router", Description: "test", SourceOwner: "root-composition", SourceRole: "cli", TargetClass: "http-router", Scopes: []string{"production"}},
		{ID: "root.compose.dev-tools", Description: "test", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "dev-tools", TargetRole: "cli", Scopes: []string{"production"}},
		{ID: "legacy.router", Description: "test", SourceOwner: "legacy-composition", SourceRole: "compose", TargetClass: "http-router", Scopes: []string{"production"}},
	} {
		candidate := repositoryPolicy(t)
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

func TestHTTPHostRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	production := []string{"production"}
	internalTest := []string{"internal_test"}
	standing := []Rule{
		{SourceOwner: "root-composition", SourceRole: "compose", TargetClass: "http-router", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "module-internal-test", TargetClass: "cli-config", Scopes: internalTest},
	}
	for _, original := range standing {
		if !reviewedHTTPHostRule(original) {
			t.Fatalf("standing HTTP host rule was rejected: %+v", original)
		}
		for _, mutate := range []func(*Rule){
			func(r *Rule) { r.SourceOwner = "" },
			func(r *Rule) { r.SourceOwner = "legacy-composition" },
			func(r *Rule) { r.SourceOwnerKind = "composition" },
			func(r *Rule) { r.SourceRole = "" },
			func(r *Rule) { r.TargetOwnerKind = "tool" },
			func(r *Rule) { r.TargetOwner = "scheduler-runtime" },
			func(r *Rule) { r.TargetRole = "application" },
			func(r *Rule) { r.TargetClass = "orm-sql" },
			func(r *Rule) { r.SameOwner = true },
			func(r *Rule) { r.Scopes = everyScope },
		} {
			broadened := original
			mutate(&broadened)
			if reviewedHTTPHostRule(broadened) {
				t.Fatalf("accepted broadened rule: %+v", broadened)
			}
		}
	}
	for _, broadened := range []Rule{
		{SourceOwner: "root-composition", SourceRole: "compose", TargetClass: "orm-sql", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "compose", TargetClass: "cli-config", Scopes: internalTest},
		{SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "identity-access", TargetRole: "application", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "tenant-runtime", TargetRole: "adapter", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "module-internal-test", TargetOwner: "tenant-runtime", TargetRole: "http", Scopes: internalTest},
		{SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "dev-tools", TargetRole: "cli", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "cli", TargetOwner: "identity-access", TargetRole: "adapter", Scopes: production},
		// ADR 0057: the session runtime is Identity's http role, which the
		// root reaches through ADR 0053 point 2; the dev tools' cli role is
		// another owner's private role.
		{SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "identity-access", TargetRole: "adapter", Scopes: production},
		{SourceOwner: "root-composition", SourceRole: "module-internal-test", TargetOwner: "identity-access", TargetRole: "adapter", Scopes: internalTest},
		{SourceOwner: "root-composition", SourceRole: "cli", TargetOwner: "dev-tools", TargetRole: "cli", Scopes: production},
	} {
		if reviewedHTTPHostRule(broadened) {
			t.Fatalf("accepted rule outside the HTTP host: %+v", broadened)
		}
	}
}
