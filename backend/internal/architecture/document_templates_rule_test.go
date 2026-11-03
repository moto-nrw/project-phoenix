package architecture

import (
	"slices"
	"strings"
	"testing"
)

func TestDocumentTemplatesRulesRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedDocumentTemplatesRule)
	if len(candidate.Rules)-len(base.Rules) != 6 {
		t.Fatalf("expected six reviewed document templates rules, got %d", len(candidate.Rules)-len(base.Rules))
	}
	if err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed document templates rules: %v", err)
	}
	unreviewed := *candidate
	unreviewed.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(&base, &unreviewed, noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("the document templates rules passed without an epoch")
	}
}

func TestDocumentTemplatesRuleRejectsBroadenedSelectors(t *testing.T) {
	t.Parallel()
	original := Rule{SourceOwner: "parent-billing", SourceRole: "application", TargetPackage: documentTemplatesTargetPackage,
		TargetOwner: "document-rendering", TargetRole: "contract", Scopes: []string{"production"}}
	if !reviewedDocumentTemplatesRule(original) {
		t.Fatal("the reviewed rule itself must be accepted")
	}
	for name, mutate := range map[string]func(*Rule){
		"any source owner":    func(r *Rule) { r.SourceOwner = "" },
		"another source":      func(r *Rule) { r.SourceOwner = "enrollment" },
		"source kind":         func(r *Rule) { r.SourceOwnerKind = "domain" },
		"source package":      func(r *Rule) { r.SourcePackage = "modules/parentbilling/internal/application" },
		"another source role": func(r *Rule) { r.SourceRole = "postgres" },
		"every package":       func(r *Rule) { r.TargetPackage = "" },
		"another package":     func(r *Rule) { r.TargetPackage = "modules/documentrendering/lists" },
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
		if reviewedDocumentTemplatesRule(broadened) {
			t.Errorf("%s: broadened rule accepted", name)
		}
	}
}

func TestDocumentTemplatesEpochGrantsNoOtherDocumentRenderingPackage(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch--
	base.Rules = slices.DeleteFunc(slices.Clone(candidate.Rules), reviewedDocumentTemplatesRule)
	widened := *candidate
	rule := Rule{ID: "organization-tenancy.application.document-rendering-contract", Description: "test",
		SourceOwner: "organization-tenancy", SourceRole: "application", TargetOwner: "document-rendering", TargetRole: "contract",
		Scopes: []string{"production"}}
	widened.Rules = append(slices.Clone(candidate.Rules), rule)
	err := comparePolicyStrictness(&base, &widened, noCreated(), noCreated(), noCreated(), noCreated(), nil)
	if err == nil || !strings.Contains(err.Error(), rule.ID) {
		t.Fatalf("the whole Document Rendering contract role escaped the guard: %v", err)
	}
}
