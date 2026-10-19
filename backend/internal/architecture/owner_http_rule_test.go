package architecture

import "testing"

func TestReviewedOwnerHTTPRule(t *testing.T) {
	t.Parallel()
	base := &Policy{ModulePath: "example.com/project", Owners: []Owner{{ID: "care", Kind: "domain"}, {ID: "people", Kind: "domain"}, {ID: "higher", Kind: "domain"}, {ID: "projection", Kind: "projection"}, {ID: "workflow", Kind: "workflow"}, {ID: "tenant-runtime", Kind: "platform"}, {ID: "identity-access", Kind: "domain"}}, Rules: []Rule{{SourceOwner: "care", SourceRole: "application", TargetOwner: "people", TargetRole: "public", Scopes: []string{"production"}}}}
	candidate := *base
	candidate.Packages = []Package{{Path: "care/http", Owner: "care", Role: "http"}, {Path: "projection/http", Owner: "projection", Role: "http"}, {Path: "workflow/http", Owner: "workflow", Role: "http"}, {Path: "people/http", Owner: "people", Role: "http"}}
	valid := Rule{SourceOwner: "care", SourcePackage: "care/http", SourceRole: "http", TargetOwner: "people", TargetRole: "public", Scopes: []string{"production"}}
	cases := []struct {
		name   string
		change func(*Rule)
		want   bool
	}{
		{"existing direction", func(*Rule) {}, true},
		{"own contract", func(r *Rule) { r.TargetOwner = "care"; r.TargetRole = "contract" }, true},
		{"reverse", func(r *Rule) { r.SourceOwner = "people"; r.SourcePackage = "people/http"; r.TargetOwner = "care" }, false},
		{"new domain edge", func(r *Rule) { r.TargetOwner = "higher" }, false},
		{"not granted target role", func(r *Rule) { r.TargetRole = "contract" }, false},
		{"projection native contract", func(r *Rule) {
			r.SourceOwner = "projection"
			r.SourcePackage = "projection/http"
			r.TargetOwner = "higher"
			r.TargetRole = "contract"
		}, true},
		{"workflow platform public", func(r *Rule) {
			r.SourceOwner = "workflow"
			r.SourcePackage = "workflow/http"
			r.TargetOwner = "tenant-runtime"
		}, true},
		{"projection target", func(r *Rule) { r.TargetOwner = "projection" }, false},
		{"workflow target", func(r *Rule) { r.TargetOwner = "workflow" }, false},
		{"compose", func(r *Rule) { r.TargetRole = "compose" }, false},
		{"application", func(r *Rule) { r.TargetRole = "application" }, false},
		{"postgres", func(r *Rule) { r.TargetRole = "postgres" }, false},
		{"private domain", func(r *Rule) { r.TargetRole = "domain" }, false},
		{"unscoped package", func(r *Rule) { r.SourcePackage = "" }, false},
		{"wrong owner", func(r *Rule) { r.SourceOwner = "higher" }, false},
		{"source kind", func(r *Rule) { r.SourceOwnerKind = "domain" }, false},
		{"target kind", func(r *Rule) { r.TargetOwnerKind = "domain" }, false},
		{"external", func(r *Rule) { r.TargetClass = "http-router" }, false},
		{"own selector", func(r *Rule) { r.SameOwner = true }, false},
		{"test role production scope", func(r *Rule) { r.SourceRole = "adapter-test" }, false},
		{"production wide scope", func(r *Rule) { r.Scopes = []string{"production", "internal_test"} }, false},
		{"runtime common", func(r *Rule) { r.TargetOwner = "tenant-runtime"; r.TargetRole = "http"; r.TargetPackage = "api/common" }, true},
		{"runtime broad", func(r *Rule) { r.TargetOwner = "tenant-runtime"; r.TargetRole = "http" }, false},
		{"runtime arbitrary package", func(r *Rule) {
			r.TargetOwner = "identity-access"
			r.TargetRole = "http"
			r.TargetPackage = "modules/identityaccess/private"
		}, false},
		{"runtime test successor", func(r *Rule) {
			r.SourceRole = "adapter-test"
			r.Scopes = []string{"internal_test", "external_test"}
			r.TargetOwner = "identity-access"
			r.TargetRole = "http"
			r.TargetPackage = "modules/identityaccess/sessiontoken"
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := valid
			tc.change(&rule)
			if got := reviewedOwnerHTTPRule(rule, base, &candidate); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOwnerHTTPRulesRequireReviewedEpoch(t *testing.T) {
	t.Parallel()
	base := &Policy{ModulePath: "example.com/project", PolicyEpoch: 73, Owners: []Owner{{ID: "care", Kind: "domain"}}, Packages: []Package{{Path: "care/http", Owner: "care", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"}, {Path: "care/contract", Owner: "care", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"}}}
	candidate := *base
	candidate.PolicyEpoch++
	candidate.Rules = []Rule{{ID: "care.http.own-contract", SourceOwner: "care", SourcePackage: "care/http", SourceRole: "http", TargetOwner: "care", TargetRole: "contract", Scopes: []string{"production"}}}
	if err := comparePolicyStrictness(base, &candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("reviewed owner HTTP reach: %v", err)
	}
	candidate.PolicyEpoch = base.PolicyEpoch
	if err := comparePolicyStrictness(base, &candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err == nil {
		t.Fatal("owner HTTP reach passed without an epoch")
	}
}

func TestOwnerHTTPNativeSuccessorKeepsTheExistingTarget(t *testing.T) {
	t.Parallel()
	base := &Policy{ModulePath: "example.com/project", Owners: []Owner{{ID: "care-plan", Kind: "domain"}, {ID: "people-directory", Kind: "domain"}}, Rules: []Rule{{SourceOwner: "people-directory", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "public", Scopes: []string{"production"}}}}
	candidate := *base
	candidate.Packages = []Package{{Path: "modules/careplan/http", Owner: "care-plan", Role: "http"}}
	rule := Rule{SourceOwner: "care-plan", SourcePackage: "modules/careplan/http", SourceRole: "http", TargetOwner: "people-directory", TargetRole: "public", Scopes: []string{"production"}}
	if !reviewedOwnerHTTPRule(rule, base, &candidate) {
		t.Fatal("native successor refused")
	}
	missing := *base
	missing.Rules = nil
	if reviewedOwnerHTTPRule(rule, &missing, &candidate) {
		t.Fatal("successor without predecessor accepted")
	}
	rule.TargetRole = "compose"
	if reviewedOwnerHTTPRule(rule, base, &candidate) {
		t.Fatal("private successor accepted")
	}
}
