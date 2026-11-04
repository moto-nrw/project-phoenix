package architecture

import (
	"slices"
	"strings"
	"testing"
)

func TestDemoUploadsRuleRequiresReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedDemoUploadsRule)
	if len(candidate.Rules)-len(base.Rules) != 1 {
		t.Fatal("expected one reviewed demo uploads rule")
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed demo uploads rule: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("the demo uploads rule passed without an epoch")
	}
}

func TestDemoUploadsRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	original := Rule{SourceOwner: "dev-tools", SourceRole: "compose", TargetPackage: demoUploadsTargetPackage,
		TargetOwner: "delivery-platform", TargetRole: "public", Scopes: []string{"production"}}
	if !reviewedDemoUploadsRule(original) {
		t.Fatal("the reviewed rule itself must be accepted")
	}
	for name, mutate := range map[string]func(*Rule){
		"any source owner":    func(r *Rule) { r.SourceOwner = "" },
		"another source":      func(r *Rule) { r.SourceOwner = "organization-tenancy" },
		"source kind":         func(r *Rule) { r.SourceOwnerKind = "tool" },
		"source package":      func(r *Rule) { r.SourcePackage = "cmd/devtools" },
		"another source role": func(r *Rule) { r.SourceRole = "cli" },
		"every package":       func(r *Rule) { r.TargetPackage = "" },
		"another package":     func(r *Rule) { r.TargetPackage = "modules/delivery/runtime" },
		"another owner":       func(r *Rule) { r.TargetOwner = "file-storage" },
		"owner kind":          func(r *Rule) { r.TargetOwnerKind = "platform" },
		"another role":        func(r *Rule) { r.TargetRole = "application" },
		"external class":      func(r *Rule) { r.TargetClass = "orm-sql" },
		"same owner":          func(r *Rule) { r.SameOwner = true },
		"test scope":          func(r *Rule) { r.Scopes = []string{"production", "internal_test"} },
		"only a test scope":   func(r *Rule) { r.Scopes = []string{"external_test"} },
	} {
		broadened := original
		broadened.Scopes = slices.Clone(original.Scopes)
		mutate(&broadened)
		if reviewedDemoUploadsRule(broadened) {
			t.Errorf("%s: broadened rule accepted", name)
		}
	}
}

func TestDemoUploadsEpochGrantsNoOtherDeliveryPackage(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedDemoUploadsRule)
	widened := *candidate
	rule := Rule{ID: "dev-tools.compose.delivery-public", Description: "test", SourceOwner: "dev-tools", SourceRole: "compose",
		TargetOwner: "delivery-platform", TargetRole: "public", Scopes: []string{"production"}}
	widened.Rules = append(slices.Clone(candidate.Rules), rule)
	err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), nil)
	if err == nil || !strings.Contains(err.Error(), rule.ID) {
		t.Fatalf("the whole Delivery public role escaped the guard: %v", err)
	}
}
