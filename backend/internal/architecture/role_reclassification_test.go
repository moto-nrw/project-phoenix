package architecture

import (
	"slices"
	"strings"
	"testing"
)

// reclassificationPolicies returns a base where beta's session runtime is an
// adapter, its rows are domain and alpha reaches both through rules, and a
// candidate one epoch later that classifies them as http and legacy-model.
func reclassificationPolicies() (*Policy, *Policy) {
	base := &Policy{
		PolicyEpoch: targetOnlyRulesEpoch - 1,
		ModulePath:  "example.test/reclassification",
		Owners:      []Owner{{ID: "alpha", Kind: "domain"}, {ID: "beta", Kind: "domain"}, {ID: "gamma", Kind: "domain"}},
		Packages: []Package{
			{Path: "alpha/http", Owner: "alpha", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
			{Path: "gamma/http", Owner: "gamma", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
			{Path: "beta/session", Owner: "beta", Role: "adapter", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
			{Path: "beta/inbound", Owner: "beta", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
			{Path: "beta/application", Owner: "beta", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
			{Path: "models/beta", Owner: "beta", Role: "domain", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		},
		Rules: []Rule{
			{ID: "alpha.http.beta-session", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "http", TargetOwner: "beta", TargetRole: "adapter"},
			{ID: "beta.application.domain", Scopes: []string{"production"}, SourceOwner: "beta", SourceRole: "application", TargetOwner: "beta", TargetRole: "domain"},
			{ID: "domain.models.public", Scopes: []string{"production"}, SourceOwner: "beta", SourceRole: "domain", TargetOwner: "alpha", TargetRole: "http"},
		},
	}
	candidate := *base
	candidate.PolicyEpoch = targetOnlyRulesEpoch
	candidate.Packages = slices.Clone(base.Packages)
	for i := range candidate.Packages {
		switch candidate.Packages[i].Path {
		case "beta/session":
			candidate.Packages[i].Role = "http"
		case "models/beta":
			candidate.Packages[i].Role = legacyModelRole
		}
	}
	candidate.Rules = []Rule{
		{ID: "alpha.http.beta-session", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "http", TargetOwner: "beta", TargetRole: "http"},
		{ID: "beta.application.legacy-model", Scopes: []string{"production"}, SourceOwner: "beta", SourceRole: "application", TargetOwner: "beta", TargetRole: legacyModelRole},
		{ID: "legacy-model.models.public", Scopes: []string{"production"}, SourceOwner: "beta", SourceRole: legacyModelRole, TargetOwner: "alpha", TargetRole: "http"},
	}
	return base, &candidate
}

func compareReclassification(base, candidate *Policy) error {
	return comparePolicyStrictness(base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated())
}

func TestReclassifiedRolesKeepTheirReach(t *testing.T) {
	t.Parallel()
	base, candidate := reclassificationPolicies()
	if err := compareReclassification(base, candidate); err != nil {
		t.Fatalf("reviewed reclassification rejected: %v", err)
	}
}

func TestReclassifiedRolesNeedAReviewedEpoch(t *testing.T) {
	t.Parallel()
	base, candidate := reclassificationPolicies()
	candidate.PolicyEpoch = base.PolicyEpoch
	err := compareReclassification(base, candidate)
	if err == nil || !strings.Contains(err.Error(), "rule alpha.http.beta-session newly allows") ||
		!strings.Contains(err.Error(), "rule beta.application.legacy-model newly allows") {
		t.Fatalf("reclassification passed without an epoch increase: %v", err)
	}
}

func TestReclassifiedRolesGrantNoOtherSource(t *testing.T) {
	t.Parallel()
	base, candidate := reclassificationPolicies()
	candidate.Rules = append(candidate.Rules, Rule{ID: "gamma.http.beta-session", Scopes: []string{"production"}, SourceOwner: "gamma", SourceRole: "http", TargetOwner: "beta", TargetRole: "http"})
	err := compareReclassification(base, candidate)
	if err == nil || !strings.Contains(err.Error(), "rule gamma.http.beta-session newly allows") {
		t.Fatalf("a source without reach into either role gained the reclassified runtime: %v", err)
	}
}

func TestOnlyNamedReclassificationsKeepTheirReach(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name   string
		mutate func(*Policy)
	}{
		{"adapter to public", func(candidate *Policy) {
			setPolicyPackageRole(candidate, "beta/session", "public")
			candidate.Rules[0].TargetRole = "public"
		}},
		{"changed test role", func(candidate *Policy) {
			for i := range candidate.Packages {
				if candidate.Packages[i].Path == "beta/session" {
					candidate.Packages[i].ExternalTestRole = "adapter-test"
				}
			}
		}},
		{"application to legacy-model", func(candidate *Policy) {
			setPolicyPackageRole(candidate, "beta/application", legacyModelRole)
			candidate.Rules = append(candidate.Rules, Rule{ID: "alpha.http.beta-models", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "http", TargetOwner: "beta", TargetRole: legacyModelRole})
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := reclassificationPolicies()
			scenario.mutate(candidate)
			if err := compareReclassification(base, candidate); err == nil || !strings.Contains(err.Error(), "newly allows") {
				t.Fatalf("unnamed reclassification kept its reach: %v", err)
			}
		})
	}
}

func setPolicyPackageRole(policy *Policy, path, role string) {
	for i := range policy.Packages {
		if policy.Packages[i].Path == path {
			policy.Packages[i].Role = role
		}
	}
}
