package architecture

import (
	"slices"
	"strings"
	"testing"
)

// transition for #4078; delete when no inbound owners remain.
func inboundTransitionPolicies() (*Policy, *Policy) {
	base := &Policy{
		ModulePath: "example.test/inbound", PolicyEpoch: targetOnlyRulesEpoch,
		Owners: []Owner{{ID: "inbound-meals", Kind: "inbound"}, {ID: "meals", Kind: "domain"}, {ID: rootCompositionOwner, Kind: "composition"}},
		Packages: []Package{
			{Path: "modules/meals/http", Owner: "inbound-meals", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
			{Path: "modules/meals", Owner: "meals", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
			{Path: "modules/meals/compose", Owner: "meals", Role: "compose", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
			{Path: "api", Owner: rootCompositionOwner, Role: "compose", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		},
		Rules: []Rule{
			{ID: "inbound.meals", Scopes: []string{"production"}, SourceOwner: "inbound-meals", SourceRole: "http", TargetOwner: "meals", TargetRole: "public"},
			{ID: "root.inbound", Scopes: []string{"production"}, SourceOwner: rootCompositionOwner, SourceRole: "compose", TargetOwner: "inbound-meals", TargetRole: "http"},
			{ID: "root.http", Scopes: []string{"production", "internal_test", "external_test"}, SourceOwner: rootCompositionOwner, TargetOwnerKind: "domain", TargetRole: "http"},
			{ID: "own.tests", Scopes: []string{"internal_test", "external_test"}, TargetRole: "compose", SameOwner: true},
		},
	}
	candidate := *base
	candidate.Owners = slices.Clone(base.Owners[1:])
	candidate.Packages = slices.Clone(base.Packages)
	candidate.Packages[0].Owner = "meals"
	candidate.Rules = slices.Clone(base.Rules)
	candidate.Rules[0].ID, candidate.Rules[0].SourceOwner = "meals.http.public", "meals"
	candidate.Rules = slices.Delete(candidate.Rules, 1, 2)
	return base, &candidate
}

func TestInboundOwnerTransitionClassification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*Policy, *Policy)
		allowed bool
	}{
		{name: "own HTTP", allowed: true},
		{name: "platform HTTP", allowed: true, mutate: func(base, candidate *Policy) { base.Owners[1].Kind, candidate.Owners[0].Kind = "platform", "platform" }},
		{name: "workflow HTTP", allowed: true, mutate: func(base, candidate *Policy) { base.Owners[1].Kind, candidate.Owners[0].Kind = "workflow", "workflow" }},
		{name: "root composition", allowed: true, mutate: func(_, candidate *Policy) {
			candidate.Packages[0].Owner, candidate.Packages[0].Role = rootCompositionOwner, "compose"
		}},
		{name: "real base owner", mutate: func(base, _ *Policy) { base.Owners[0].Kind = "domain" }},
		{name: "missing base owner", mutate: func(base, _ *Policy) { base.Owners = base.Owners[1:] }},
		{name: "inbound retained", mutate: func(base, candidate *Policy) { candidate.Owners = append(candidate.Owners, base.Owners[0]) }},
		{name: "another inbound", mutate: func(_, candidate *Policy) { candidate.Owners[0].Kind = "inbound" }},
		{name: "missing target owner", mutate: func(_, candidate *Policy) { candidate.Owners = candidate.Owners[1:] }},
		{name: "private target role", mutate: func(_, candidate *Policy) { candidate.Packages[0].Role = "application" }},
		{name: "domain composition", mutate: func(_, candidate *Policy) { candidate.Packages[0].Role = "compose" }},
		{name: "root HTTP", mutate: func(_, candidate *Policy) { candidate.Packages[0].Owner = rootCompositionOwner }},
		{name: "kernel HTTP", mutate: func(_, candidate *Policy) { candidate.Owners[0].Kind = "kernel" }},
		{name: "test role change", mutate: func(_, candidate *Policy) { candidate.Packages[0].InternalTestRole = "test-support" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := inboundTransitionPolicies()
			if tc.mutate != nil {
				tc.mutate(base, candidate)
			}
			problems := classificationLoosenings(base, candidate, nil, nil)
			if (len(problems) == 0) != tc.allowed {
				t.Fatalf("allowed=%v, problems=%v", tc.allowed, problems)
			}
		})
	}
}

func TestInboundOwnerTransitionStrictness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*Policy, *Policy)
		problem string
	}{
		{name: "same path successor and standing root and own test reach"},
		{name: "narrower successor scopes", mutate: func(base, _ *Policy) { base.Rules[0].Scopes = []string{"production", "internal_test"} }},
		{name: "deleted target rule successor", mutate: func(base, candidate *Policy) {
			base.Rules[0] = Rule{ID: "inbound.test.http", Scopes: []string{"external_test"}, SourceOwner: "meals", SourceRole: "module-behavior-test", TargetOwner: "inbound-meals", TargetRole: "http"}
			candidate.Rules[0] = base.Rules[0]
			candidate.Rules[0].ID, candidate.Rules[0].TargetOwner = "meals.test.http", "meals"
		}},
		{name: "retained predecessor", problem: "rule meals.http.public newly allows", mutate: func(base, candidate *Policy) { candidate.Rules = append(candidate.Rules, base.Rules[0]) }},
		{name: "wider scope", problem: "rule meals.http.public newly allows", mutate: func(_, candidate *Policy) { candidate.Rules[0].Scopes = []string{"production", "internal_test"} }},
		{name: "wider source selector", problem: "rule meals.http.public newly allows", mutate: func(_, candidate *Policy) { candidate.Rules[0].SourceRole = "" }},
		{name: "wider target selector", problem: "rule meals.http.public newly allows", mutate: func(_, candidate *Policy) {
			candidate.Rules[0].TargetOwner = ""
			candidate.Rules[0].TargetOwnerKind = "domain"
		}},
		{name: "foreign implementation", problem: "rule meals.http.public newly allows", mutate: func(_, candidate *Policy) {
			candidate.Rules[0].TargetOwner, candidate.Rules[0].TargetRole = rootCompositionOwner, "application"
		}},
		{name: "rule count growth", problem: "rule meals.http.public newly allows", mutate: func(base, candidate *Policy) { candidate.Rules = append(candidate.Rules, base.Rules[2], base.Rules[3]) }},
		{name: "unrelated existing HTTP gains reach", problem: "production|imports.forbidden|example.test/inbound/modules/meals/other|example.test/inbound/modules/meals", mutate: func(base, candidate *Policy) {
			pkg := candidate.Packages[0]
			pkg.Path = "modules/meals/other"
			base.Packages = append(base.Packages, pkg)
			candidate.Packages = append(candidate.Packages, pkg)
		}},
		{name: "unrelated owner change", problem: "changed owner", mutate: func(_, candidate *Policy) { candidate.Packages[1].Owner = rootCompositionOwner }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := inboundTransitionPolicies()
			if tc.mutate != nil {
				tc.mutate(base, candidate)
			}
			err := compareReclassification(base, candidate)
			if tc.problem == "" {
				if err != nil {
					t.Fatalf("transition rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.problem) {
				t.Fatalf("want %q, got %v", tc.problem, err)
			}
		})
	}
}
