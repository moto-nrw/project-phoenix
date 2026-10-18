package architecture

import (
	"slices"
	"strings"
	"testing"
)

func TestCommonHTTPPermissionsRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := historicalCommonHTTPPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedCommonHTTPRule)
	if len(candidate.Rules)-len(base.Rules) != 3 {
		t.Fatal("expected three standing HTTP rules")
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err != nil {
		t.Fatalf("reviewed HTTP contracts: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), noCreated()); err == nil {
		t.Fatal("new HTTP permissions passed without an epoch")
	}
}

func TestCommonHTTPEpochGrantsOnlyTheNamedContracts(t *testing.T) {
	t.Parallel()
	for _, rule := range []Rule{
		{ID: "common.application", Description: "test", SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "security-runtime", TargetRole: "application", Scopes: []string{"production"}},
		{ID: "common.orm", Description: "test", SourceOwner: "inbound-common", SourceRole: "http", TargetClass: "orm-sql", Scopes: []string{"production"}},
		{ID: "other.http", Description: "test", SourceOwner: "other-inbound", SourceRole: "http", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"production"}},
		{ID: "common.models", Description: "test", SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "domain", Scopes: []string{"production"}},
	} {
		candidate := historicalCommonHTTPPolicy(t)
		// Keep the unrelated owner independent of production inbound retirements.
		candidate.Owners = append(candidate.Owners, Owner{ID: "other-inbound", Kind: "inbound"})
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

func TestCommonHTTPRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	original := Rule{SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}}
	for _, mutate := range []func(*Rule){
		func(r *Rule) { r.SourceOwner = "" },
		func(r *Rule) { r.SourceOwnerKind = "inbound" },
		func(r *Rule) { r.SourceRole = "" },
		func(r *Rule) { r.TargetOwnerKind = "platform" },
		func(r *Rule) { r.TargetRole = "postgres" },
		func(r *Rule) { r.TargetClass = "orm-sql" },
		func(r *Rule) { r.SameOwner = true },
		func(r *Rule) { r.Scopes = everyScope },
		func(r *Rule) { r.SourceRole = "adapter-test"; r.Scopes = []string{"internal_test", "external_test"} },
	} {
		broadened := original
		mutate(&broadened)
		if reviewedCommonHTTPRule(broadened) {
			t.Fatalf("accepted broadened rule: %+v", broadened)
		}
	}
	sessionTest := Rule{SourceOwner: "inbound-common", SourceRole: "adapter-test", TargetOwner: "identity-access", TargetRole: "http", Scopes: []string{"internal_test", "external_test"}}
	privateSession := sessionTest
	privateSession.TargetRole = "adapter"
	if reviewedCommonHTTPRule(privateSession) {
		t.Fatal("accepted the session runtime in Identity's private adapter role")
	}
	if !reviewedCommonHTTPRule(original) || !reviewedCommonHTTPRule(sessionTest) {
		t.Fatal("standing HTTP or session-test contract was rejected")
	}
	sessionTest.Scopes = everyScope
	if reviewedCommonHTTPRule(sessionTest) {
		t.Fatal("session test rule gained production access")
	}
}

// historicalCommonHTTPPolicy keeps the original epoch contracts independent
// of the production owner retirement. The final tool cutover removes this
// temporary historical guard together with its production matcher.
func historicalCommonHTTPPolicy(t *testing.T) *Policy {
	t.Helper()
	policy := repositoryPolicy(t)
	policy.Owners = append(policy.Owners, Owner{ID: "inbound-common", Kind: "inbound"})
	policy.Rules = append(policy.Rules,
		Rule{ID: "inbound-common.http.tenant-runtime-public", Description: "The standing shared HTTP runtime consumes tenant scope, transactions and rollback markers (ADR 0046, #2738, #3898).", SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "tenant-runtime", TargetRole: "public", Scopes: []string{"production"}},
		Rule{ID: "inbound-common.http.security-runtime-contract", Description: "The standing shared HTTP runtime consumes validated principals and the permission registry (ADR 0046, #2738, #3898).", SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "security-runtime", TargetRole: "contract", Scopes: []string{"production"}},
		Rule{ID: "inbound-common.http.security-runtime-public", Description: "The standing shared HTTP runtime consumes authorization decisions, local-seeder gates and upload filename entropy (ADR 0046, #2738, #3898).", SourceOwner: "inbound-common", SourceRole: "http", TargetOwner: "security-runtime", TargetRole: "public", Scopes: []string{"production"}},
	)
	return policy
}
