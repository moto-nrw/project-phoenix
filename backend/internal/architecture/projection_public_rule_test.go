package architecture

import "testing"

func TestReviewedProjectionPublicRule(t *testing.T) {
	t.Parallel()
	owners := map[string]Owner{
		"review":   {ID: "review", Kind: "projection"},
		"care":     {ID: "care", Kind: "domain"},
		"security": {ID: "security", Kind: "platform"},
		"workflow": {ID: "workflow", Kind: "workflow"},
	}
	valid := Rule{SourceOwner: "review", SourceRole: "public", TargetOwner: "care", TargetRole: "public", Scopes: []string{"production"}}
	cases := []struct {
		name   string
		change func(*Rule)
		want   bool
	}{
		{"domain public", func(*Rule) {}, true},
		{"domain contract", func(r *Rule) { r.TargetRole = "contract" }, true},
		{"platform public", func(r *Rule) { r.TargetOwner = "security" }, true},
		{"platform contract", func(r *Rule) { r.TargetOwner = "security"; r.TargetRole = "contract" }, true},
		{"reverse", func(r *Rule) { r.SourceOwner = "care"; r.TargetOwner = "review" }, false},
		{"workflow", func(r *Rule) { r.TargetOwner = "workflow" }, false},
		{"projection", func(r *Rule) { r.TargetOwner = "review" }, false},
		{"application", func(r *Rule) { r.TargetRole = "application" }, false},
		{"postgres", func(r *Rule) { r.TargetRole = "postgres" }, false},
		{"compose", func(r *Rule) { r.TargetRole = "compose" }, false},
		{"http", func(r *Rule) { r.TargetRole = "http" }, false},
		{"domain private", func(r *Rule) { r.TargetRole = "domain" }, false},
		{"adapter", func(r *Rule) { r.TargetRole = "adapter" }, false},
		{"unknown", func(r *Rule) { r.TargetOwner = "missing" }, false},
		{"source kind", func(r *Rule) { r.SourceOwnerKind = "projection" }, false},
		{"target kind", func(r *Rule) { r.TargetOwnerKind = "domain" }, false},
		{"other source role", func(r *Rule) { r.SourceRole = "compose" }, false},
		{"external", func(r *Rule) { r.TargetClass = "crypto" }, false},
		{"same owner", func(r *Rule) { r.SameOwner = true }, false},
		{"test scope", func(r *Rule) { r.Scopes = []string{"internal_test"} }, false},
		{"wide scope", func(r *Rule) { r.Scopes = []string{"production", "internal_test"} }, false},
		{"empty scope", func(r *Rule) { r.Scopes = nil }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := valid
			tc.change(&rule)
			if got := reviewedProjectionPublicRule(rule, owners); got != tc.want {
				t.Fatalf("reviewedProjectionPublicRule = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProjectionPublicRulesRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = nil
	for _, rule := range candidate.Rules {
		if !reviewedProjectionPublicRule(rule, ownersByID(candidate)) {
			base.Rules = append(base.Rules, rule)
		}
	}
	if len(base.Rules) == len(candidate.Rules) {
		t.Fatal("expected the response projection rules")
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed projection reach: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("new projection permissions passed without an epoch")
	}
}
