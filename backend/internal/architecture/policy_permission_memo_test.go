package architecture

import (
	"slices"
	"testing"
)

func TestPolicyPermissionMemoMatchesUncachedDecisions(t *testing.T) {
	t.Parallel()
	policy := &Policy{
		Owners: []Owner{{ID: "alpha", Kind: "domain"}, {ID: "beta", Kind: "platform"}, {ID: "kernel", Kind: "kernel"}},
		Rules: []Rule{
			{ID: "own-port", Scopes: []string{"production"}, SourceRole: "application", TargetRole: "port", SameOwner: true},
			{ID: "own-contract", Scopes: []string{"production"}, SourceRole: "application", TargetRole: "contract", SameOwner: true},
			{ID: "domain-tests", Scopes: []string{"internal_test"}, SourceOwnerKind: "domain", SourceRole: "module-internal-test", TargetOwnerKind: "domain", TargetRole: "public"},
			{ID: "overlap-a", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetOwner: "beta", TargetRole: "public"},
			{ID: "overlap-b", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetOwner: "beta", TargetRole: "public"},
			{ID: "stdlib", Scopes: []string{"production"}, SourceRole: "application", TargetClass: "stdlib"},
			{ID: "platform-tests", Scopes: []string{"external_test"}, SourceOwnerKind: "platform", SourceRole: "adapter-test", TargetClass: "test"},
			{ID: "external-overlap-a", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetClass: "orm-sql"},
			{ID: "external-overlap-b", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetClass: "orm-sql"},
		},
	}
	sources := []Package{
		{Path: "first", Owner: "alpha", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "same-point", Owner: "alpha", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "different-test-roles", Owner: "alpha", Role: "application", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "platform", Owner: "beta", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "adapter-test"},
		{Path: "shared", Owner: "kernel", Role: "application", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
	}
	targets := []Package{
		{Path: "port", Owner: "alpha", Role: "port"},
		{Path: "other-port", Owner: "beta", Role: "port"},
		{Path: "public", Owner: "alpha", Role: "public"},
		{Path: "overlap", Owner: "beta", Role: "public"},
		{Path: "contract", Owner: "alpha", Role: "contract"},
		{Path: "shared-contract", Owner: "kernel", Role: "contract"},
	}
	external := []ExternalPackage{{Path: "fmt", Class: "stdlib"}, {Path: "strings", Class: "stdlib"}, {Path: "test", Class: "test"}, {Path: "db", Class: "orm-sql"}}
	memo := newPolicyPermissionMemo(policy)
	// Repeat with changed package paths and reversed order to exercise cache
	// hits, including denials, without borrowing a different scope/role's result.
	for pass := 0; pass < 2; pass++ {
		for _, scope := range allScopes() {
			for _, source := range sources {
				for _, target := range targets {
					want := policyAllowsFirstParty(policy, scope, source, target)
					if got := memo.allowsFirstParty(scope, source, target); got != want {
						t.Fatalf("%s %s -> %s: memo=%v, uncached=%v", scope, source.Path, target.Path, got, want)
					}
				}
				for _, target := range external {
					want := policyAllowsExternal(policy, scope, source, target)
					if got := memo.allowsExternal(scope, source, target); got != want {
						t.Fatalf("%s %s -> external %s: memo=%v, uncached=%v", scope, source.Path, target.Class, got, want)
					}
				}
			}
		}
		slices.Reverse(sources)
		slices.Reverse(targets)
		for i := range sources {
			sources[i].Path += "-alias"
		}
	}
}

func TestImportPermissionMemoKeepsPolicyAndScopeBoundaries(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"owner kind", "test role", "external class"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := &Policy{
				ModulePath: "example.test/project",
				Owners:     []Owner{{ID: "source", Kind: "platform"}, {ID: "target", Kind: "domain"}},
				Packages: []Package{
					{Path: "source-a", Owner: "source", Role: "application", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
					{Path: "source-b", Owner: "source", Role: "application", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
					{Path: "target", Owner: "target", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
				},
			}
			candidate := *base
			candidate.Owners = slices.Clone(base.Owners)
			candidate.Packages = slices.Clone(base.Packages)
			scope, targetPath := ScopeProduction, base.absolutePackage("target")
			sources := []string{"source-a", "source-b"}
			switch name {
			case "owner kind":
				base.Rules = []Rule{{ID: "domain", Scopes: []string{"production"}, SourceOwnerKind: "domain", SourceRole: "application", TargetOwner: "target", TargetRole: "public"}}
				candidate.Owners[0].Kind = "domain"
			case "test role":
				base.Rules = []Rule{{ID: "suite", Scopes: []string{"internal_test"}, SourceOwner: "source", SourceRole: "module-internal-test", TargetOwner: "target", TargetRole: "public"}}
				candidate.Packages[0].InternalTestRole = "module-internal-test"
				scope, sources = ScopeInternalTest, sources[:1]
			case "external class":
				base.Rules = []Rule{{ID: "db", Scopes: []string{"production"}, SourceOwner: "source", SourceRole: "application", TargetClass: "orm-sql"}}
				base.ExternalPackages = []ExternalPackage{{Path: "example.test/dependency", Class: "stdlib"}}
				candidate.ExternalPackages = []ExternalPackage{{Path: "example.test/dependency", Class: "orm-sql"}}
				targetPath = "example.test/dependency"
			}
			candidate.Rules = base.Rules
			var want []string
			for _, source := range sources {
				want = append(want, Violation{Scope: scope, Rule: "imports.forbidden", Source: base.absolutePackage(source), Target: targetPath}.Key())
			}
			got := slices.Clone(importLoosenings(base, &candidate))
			want = slices.Clone(uniqueStrings(want))
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("want exact import loosenings %v, got %v", want, got)
			}
			if got := importLoosenings(&candidate, &candidate); len(got) != 0 {
				t.Fatalf("unchanged policy gained imports: %v", got)
			}
		})
	}
}
