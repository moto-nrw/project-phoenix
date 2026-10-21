package architecture

import "testing"

func TestReviewedSameOwnerContractRule(t *testing.T) {
	t.Parallel()
	valid := Rule{SourceOwner: "delivery-platform", SourceRole: "port", TargetOwner: "delivery-platform", TargetRole: "contract",
		TargetPackage: "modules/delivery/contract", Scopes: []string{"production"}}
	cases := []struct {
		name   string
		change func(*Rule)
		want   bool
	}{
		{"delivery port", func(*Rule) {}, true},
		{"delivery postgres", func(r *Rule) { r.SourceRole = "postgres" }, true},
		{"export transfer application", func(r *Rule) {
			r.SourceOwner, r.TargetOwner, r.SourceRole, r.TargetPackage = "export-transfer", "export-transfer", "application", "modules/exporttransfer"
		}, true},
		{"delivery application", func(r *Rule) { r.SourceRole = "application" }, false},
		{"delivery adapter", func(r *Rule) { r.SourceRole = "adapter" }, false},
		{"export transfer port", func(r *Rule) {
			r.SourceOwner, r.TargetOwner, r.TargetPackage = "export-transfer", "export-transfer", "modules/exporttransfer"
		}, false},
		{"other delivery contract package", func(r *Rule) { r.TargetPackage = "modules/delivery/mail" }, false},
		{"other owner's contract package", func(r *Rule) { r.TargetPackage = "modules/exporttransfer" }, false},
		{"unpinned target", func(r *Rule) { r.TargetPackage = "" }, false},
		{"foreign target owner", func(r *Rule) { r.TargetOwner = "export-transfer" }, false},
		{"foreign source owner", func(r *Rule) { r.SourceOwner = "communication" }, false},
		{"public role", func(r *Rule) { r.TargetRole = "public" }, false},
		{"domain role", func(r *Rule) { r.TargetRole = "domain" }, false},
		{"same owner selector", func(r *Rule) { r.SameOwner = true; r.TargetOwner = "" }, false},
		{"source kind", func(r *Rule) { r.SourceOwnerKind = "platform" }, false},
		{"target kind", func(r *Rule) { r.TargetOwnerKind = "platform" }, false},
		{"source package", func(r *Rule) { r.SourcePackage = "modules/delivery/internal/ports" }, false},
		{"external", func(r *Rule) { r.TargetClass = "crypto" }, false},
		{"test scope", func(r *Rule) { r.Scopes = []string{"internal_test"} }, false},
		{"wide scope", func(r *Rule) { r.Scopes = []string{"production", "internal_test"} }, false},
		{"empty scope", func(r *Rule) { r.Scopes = nil }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := valid
			tc.change(&rule)
			if got := reviewedSameOwnerContractRule(rule); got != tc.want {
				t.Fatalf("reviewedSameOwnerContractRule = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSameOwnerContractRulesRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = nil
	for _, rule := range candidate.Rules {
		if !reviewedSameOwnerContractRule(rule) {
			base.Rules = append(base.Rules, rule)
		}
	}
	if len(candidate.Rules)-len(base.Rules) != len(sameOwnerContractRules) {
		t.Fatalf("expected the %d same-owner contract rules, found %d", len(sameOwnerContractRules), len(candidate.Rules)-len(base.Rules))
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed same-owner contract reach: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("new same-owner contract permissions passed without an epoch")
	}
}
