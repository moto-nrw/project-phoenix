package architecture

import (
	"slices"
	"testing"
)

// These controls use only the pre-existing memo API and direct evaluators so
// the exact same file can qualify the original implementation first.
func TestPolicyPermissionMemoFixedPermissionVectors(t *testing.T) {
	t.Parallel()
	p := permissionVectorPolicy()
	vectors := []struct {
		name, source, target, class string
		scope                       Scope
		want                        bool
	}{
		{"empty source shortlist", "empty", "target", "", ScopeProduction, false},
		{"empty source external", "empty", "", "stdlib", ScopeProduction, false},
		{"unselected before selected", "source-other", "target", "", ScopeProduction, false},
		{"exact selected source and target", "source", "target", "", ScopeProduction, true},
		{"source selector is not prefix", "source/nested", "target", "", ScopeProduction, false},
		{"target selector is not prefix", "source", "target/nested", "", ScopeProduction, false},
		{"unselected target", "source", "target-other", "", ScopeProduction, false},
		{"same role internal scope denied", "source", "target", "", ScopeInternalTest, false},
		{"same role external scope denied", "source", "target", "", ScopeExternalTest, false},
		{"external source selected independently", "source-other", "", "stdlib", ScopeProduction, true},
		{"external unselected source", "source", "", "stdlib", ScopeProduction, false},
		{"external selector is not prefix", "source-other/nested", "", "stdlib", ScopeProduction, false},
		{"external class stays distinct", "source-other", "", "unknown", ScopeProduction, false},
		{"external scope stays distinct", "source-other", "", "stdlib", ScopeInternalTest, false},
		{"same role external only", "source", "", "codec", ScopeExternalTest, true},
		{"same role codec production denied", "source", "", "codec", ScopeProduction, false},
		{"same role codec internal denied", "source", "", "codec", ScopeInternalTest, false},
		{"scoped internal role", "scoped", "target", "", ScopeInternalTest, true},
		{"internal role not production", "scoped", "target", "", ScopeProduction, false},
		{"scoped external role", "scoped", "", "test", ScopeExternalTest, true},
		{"external role not internal", "scoped", "", "test", ScopeInternalTest, false},
		{"same owner domain", "source", "own-contract", "", ScopeProduction, true},
		{"same owner rejects foreign", "platform", "own-contract", "", ScopeProduction, false},
		{"same owner excludes kernel", "kernel-source", "kernel-contract", "", ScopeProduction, false},
		{"source owner kind platform", "platform", "own-port", "", ScopeProduction, true},
		{"source owner kind domain rejected", "source", "own-port", "", ScopeProduction, false},
		{"unknown owner kind rejected", "unknown-owner", "own-port", "", ScopeProduction, false},
		{"target owner kind rejected", "platform", "platform-port", "", ScopeProduction, false},
		{"first party overlap denies", "source", "overlap", "", ScopeProduction, false},
		{"external overlap denies", "source", "", "orm-sql", ScopeProduction, false},
		{"external class cannot allow first party", "source-other", "target", "", ScopeProduction, false},
	}
	// Each memo sees both visitation orders, repeat denials, and different
	// targets under an already visited source. Reversing policy order must not
	// turn either overlapping match set into a first-match permission.
	for ruleOrder := 0; ruleOrder < 2; ruleOrder++ {
		memo := newPolicyPermissionMemo(p)
		for pass := 0; pass < 3; pass++ {
			for _, v := range vectors {
				source := permissionControlPackage(t, p, v.source)
				var direct, got bool
				if v.class != "" {
					target := ExternalPackage{Path: "example.test/external/" + v.class, Class: v.class}
					direct = policyAllowsExternal(p, v.scope, source, target)
					got = memo.allowsExternal(v.scope, source, target)
				} else {
					target := permissionControlPackage(t, p, v.target)
					direct = policyAllowsFirstParty(p, v.scope, source, target)
					got = memo.allowsFirstParty(v.scope, source, target)
				}
				if direct != v.want || got != v.want {
					t.Fatalf("%s rule-order=%d pass=%d: direct=%v memo=%v, want %v", v.name, ruleOrder, pass, direct, got, v.want)
				}
			}
			slices.Reverse(vectors)
		}
		slices.Reverse(p.Rules)
	}
}

func TestPolicyPermissionMemoCompleteMatrixParity(t *testing.T) {
	t.Parallel()
	p := permissionVectorPolicy()
	sources := slices.Clone(p.Packages)
	targets := slices.Clone(p.Packages)
	externals := []ExternalPackage{
		{Path: "fmt", Class: "stdlib"}, {Path: "strings", Class: "stdlib"},
		{Path: "test", Class: "test"}, {Path: "db", Class: "orm-sql"},
		{Path: "codec", Class: "codec"}, {Path: "unknown", Class: "unknown"},
	}
	for ruleOrder := 0; ruleOrder < 2; ruleOrder++ {
		memo := newPolicyPermissionMemo(p)
		for pass := 0; pass < 2; pass++ {
			for _, scope := range allScopes() {
				for _, source := range sources {
					for _, target := range targets {
						want := policyAllowsFirstParty(p, scope, source, target)
						if got := memo.allowsFirstParty(scope, source, target); got != want {
							t.Fatalf("rule-order=%d pass=%d %s %s -> %s: memo=%v direct=%v", ruleOrder, pass, scope, source.Path, target.Path, got, want)
						}
					}
					for _, target := range externals {
						want := policyAllowsExternal(p, scope, source, target)
						if got := memo.allowsExternal(scope, source, target); got != want {
							t.Fatalf("rule-order=%d pass=%d %s %s -> external %s: memo=%v direct=%v", ruleOrder, pass, scope, source.Path, target.Path, got, want)
						}
					}
				}
			}
			slices.Reverse(sources)
			slices.Reverse(targets)
			slices.Reverse(externals)
		}
		slices.Reverse(p.Rules)
	}
}

func TestPolicyPermissionMemoFreshPolicyControls(t *testing.T) {
	t.Parallel()
	p := permissionComparisonPolicy()
	source := permissionControlPackage(t, p, "source")
	target := permissionControlPackage(t, p, "target")
	external := ExternalPackage{Path: "fmt", Class: "stdlib"}
	rules := slices.Clone(p.Rules)
	p.Rules = nil
	for phase, want := range []bool{false, true, false} {
		// Do not use an old memo after mutating its policy. Each comparison
		// builds a new memo, including after an earlier empty rule set.
		memo := newPolicyPermissionMemo(p)
		if got := memo.allowsFirstParty(ScopeProduction, source, target); got != want {
			t.Fatalf("phase %d first party=%v, want %v", phase, got, want)
		}
		if got := memo.allowsExternal(ScopeProduction, source, external); got != want {
			t.Fatalf("phase %d external=%v, want %v", phase, got, want)
		}
		switch phase {
		case 0:
			p.Rules = rules
		case 1:
			p.Rules[0].SourcePackage = "source-other"
			p.Rules[1].Scopes = []string{"external_test"}
		}
	}
}

func TestImportPermissionMemoFixedDiagnosticVectors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		prepare func(*Policy)
		change  func(*Policy)
		forward []string
		reverse []string
	}{
		{
			name:   "source selector removal",
			change: func(p *Policy) { p.Rules[0].SourcePackage = "" },
			forward: []string{
				"production|imports.forbidden|example.test/permissions/source-other|example.test/permissions/target",
				"production|imports.forbidden|example.test/permissions/source/nested|example.test/permissions/target",
			},
		},
		{
			name:   "target selector removal",
			change: func(p *Policy) { p.Rules[0].TargetPackage = "" },
			forward: []string{
				"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target-other",
				"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target/nested",
			},
		},
		{
			name:    "source selector moves",
			change:  func(p *Policy) { p.Rules[0].SourcePackage = "source-other" },
			forward: []string{"production|imports.forbidden|example.test/permissions/source-other|example.test/permissions/target"},
			reverse: []string{"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target"},
		},
		{
			name:    "target selector moves",
			change:  func(p *Policy) { p.Rules[0].TargetPackage = "target-other" },
			forward: []string{"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target-other"},
			reverse: []string{"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target"},
		},
		{
			name:   "external source selector removal",
			change: func(p *Policy) { p.Rules[1].SourcePackage = "" },
			forward: []string{
				"production|imports.forbidden|example.test/permissions/source-other|fmt",
				"production|imports.forbidden|example.test/permissions/source-other|strings",
				"production|imports.forbidden|example.test/permissions/source/nested|fmt",
				"production|imports.forbidden|example.test/permissions/source/nested|strings",
			},
		},
		{
			name:   "same role new scope",
			change: func(p *Policy) { p.Rules[1].Scopes = []string{"production", "external_test"} },
			forward: []string{
				"external_test|imports.forbidden|example.test/permissions/source|fmt",
				"external_test|imports.forbidden|example.test/permissions/source|strings",
			},
		},
		{
			name: "source owner kind differs by policy",
			prepare: func(p *Policy) {
				p.Owners[0].Kind = "platform"
				for i := range p.Rules {
					p.Rules[i].SourceOwner, p.Rules[i].SourceOwnerKind = "", "domain"
				}
			},
			change: func(p *Policy) { p.Owners[0].Kind = "domain" },
			forward: []string{
				"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target",
				"production|imports.forbidden|example.test/permissions/source|fmt",
				"production|imports.forbidden|example.test/permissions/source|strings",
			},
		},
		{
			name:    "external class differs by policy",
			prepare: func(p *Policy) { p.Rules[1].TargetClass = "orm-sql" },
			change:  func(p *Policy) { p.ExternalPackages[0].Class = "orm-sql" },
			forward: []string{"production|imports.forbidden|example.test/permissions/source|fmt"},
		},
		{
			name: "overlap removal exposes both permissions",
			prepare: func(p *Policy) {
				first, external := p.Rules[0], p.Rules[1]
				first.ID, external.ID = "overlap-first", "overlap-external"
				p.Rules = append(p.Rules, first, external)
			},
			change: func(p *Policy) { p.Rules = p.Rules[:2] },
			forward: []string{
				"production|imports.forbidden|example.test/permissions/source|example.test/permissions/target",
				"production|imports.forbidden|example.test/permissions/source|fmt",
				"production|imports.forbidden|example.test/permissions/source|strings",
			},
		},
		{
			name: "scoped source role differs by policy",
			prepare: func(p *Policy) {
				p.Packages[0].InternalTestRole = "adapter-test"
				for i := range p.Rules {
					p.Rules[i].Scopes = []string{"internal_test"}
				}
			},
			change: func(p *Policy) { p.Packages[0].InternalTestRole = "application" },
			forward: []string{
				"internal_test|imports.forbidden|example.test/permissions/source|example.test/permissions/target",
				"internal_test|imports.forbidden|example.test/permissions/source|fmt",
				"internal_test|imports.forbidden|example.test/permissions/source|strings",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := permissionComparisonPolicy(), permissionComparisonPolicy()
			if tc.prepare != nil {
				tc.prepare(base)
				tc.prepare(candidate)
			}
			tc.change(candidate)
			assertPermissionControlDiagnostics(t, base, candidate, tc.forward)
			assertPermissionControlDiagnostics(t, candidate, base, tc.reverse)
			assertPermissionControlDiagnostics(t, base, base, nil)
			assertPermissionControlDiagnostics(t, candidate, candidate, nil)
		})
	}
}

func permissionComparisonPolicy() *Policy {
	p := &Policy{
		ModulePath: "example.test/permissions", PolicyEpoch: 1,
		Owners:           []Owner{{ID: "alpha", Kind: "domain"}, {ID: "beta", Kind: "platform"}},
		ExternalPackages: []ExternalPackage{{Path: "fmt", Class: "stdlib"}, {Path: "strings", Class: "stdlib"}, {Path: "db", Class: "orm-sql"}},
		Rules: []Rule{
			{ID: "selected-first", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", SourcePackage: "source", TargetOwner: "beta", TargetRole: "public", TargetPackage: "target"},
			{ID: "selected-external", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", SourcePackage: "source", TargetClass: "stdlib"},
		},
	}
	for _, path := range []string{"source", "source-other", "source/nested"} {
		p.Packages = append(p.Packages, Package{Path: path, Owner: "alpha", Role: "application", InternalTestRole: "application", ExternalTestRole: "application"})
	}
	for _, path := range []string{"target", "target-other", "target/nested"} {
		p.Packages = append(p.Packages, Package{Path: path, Owner: "beta", Role: "public", InternalTestRole: "public", ExternalTestRole: "public"})
	}
	return p
}

func permissionVectorPolicy() *Policy {
	p := permissionComparisonPolicy()
	p.Owners = append(p.Owners, Owner{ID: "gamma", Kind: "domain"}, Owner{ID: "kernel", Kind: "kernel"})
	p.Rules[1].SourcePackage = "source-other"
	p.Rules = append(p.Rules,
		Rule{ID: "internal-role", Scopes: []string{"internal_test"}, SourceOwnerKind: "domain", SourceRole: "adapter-test", TargetOwnerKind: "platform", TargetRole: "public"},
		Rule{ID: "external-role", Scopes: []string{"external_test"}, SourceOwnerKind: "domain", SourceRole: "module-behavior-test", TargetClass: "test"},
		Rule{ID: "external-scope", Scopes: []string{"external_test"}, SourceOwner: "alpha", SourceRole: "application", TargetClass: "codec"},
		Rule{ID: "own-contract", Scopes: []string{"production"}, SourceRole: "application", TargetRole: "contract", SameOwner: true},
		Rule{ID: "platform-domain-port", Scopes: []string{"production"}, SourceOwnerKind: "platform", SourceRole: "application", TargetOwnerKind: "domain", TargetRole: "port"},
		Rule{ID: "overlap-first-a", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetOwner: "gamma", TargetRole: "public"},
		Rule{ID: "overlap-first-b", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetOwner: "gamma", TargetRole: "public"},
		Rule{ID: "overlap-external-a", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetClass: "orm-sql"},
		Rule{ID: "overlap-external-b", Scopes: []string{"production"}, SourceOwner: "alpha", SourceRole: "application", TargetClass: "orm-sql"},
	)
	p.Packages = append(p.Packages,
		Package{Path: "source-other/nested", Owner: "alpha", Role: "application", InternalTestRole: "application", ExternalTestRole: "application"},
		Package{Path: "scoped", Owner: "alpha", Role: "application", InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test"},
		Package{Path: "empty", Owner: "alpha", Role: "no-rules", InternalTestRole: "no-rules", ExternalTestRole: "no-rules"},
		Package{Path: "platform", Owner: "beta", Role: "application", InternalTestRole: "application", ExternalTestRole: "application"},
		Package{Path: "unknown-owner", Owner: "missing", Role: "application", InternalTestRole: "application", ExternalTestRole: "application"},
		Package{Path: "kernel-source", Owner: "kernel", Role: "application", InternalTestRole: "application", ExternalTestRole: "application"},
		Package{Path: "own-contract", Owner: "alpha", Role: "contract"},
		Package{Path: "kernel-contract", Owner: "kernel", Role: "contract"},
		Package{Path: "own-port", Owner: "alpha", Role: "port"},
		Package{Path: "platform-port", Owner: "beta", Role: "port"},
		Package{Path: "overlap", Owner: "gamma", Role: "public"},
	)
	return p
}

func permissionControlPackage(t *testing.T, p *Policy, path string) Package {
	t.Helper()
	for _, pkg := range p.Packages {
		if pkg.Path == path {
			return pkg
		}
	}
	t.Fatalf("missing permission control package %q", path)
	return Package{}
}

func assertPermissionControlDiagnostics(t *testing.T, base, candidate *Policy, want []string) {
	t.Helper()
	want = slices.Clone(want)
	slices.Sort(want)
	direct := directPermissionControlLoosenings(base, candidate)
	got := slices.Clone(importLoosenings(base, candidate))
	slices.Sort(direct)
	slices.Sort(got)
	if !slices.Equal(direct, want) || !slices.Equal(got, want) {
		t.Fatalf("complete import diagnostics:\nwant   %q\ndirect %q\nmemo   %q", want, direct, got)
	}
}

// The controls use equal epochs, no reviewed-rule shapes, unique package
// identities, and no new/deleted packages. This independent full pair walk uses
// only the original uncached evaluator, not the memo or its shortlist helpers.
func directPermissionControlLoosenings(base, candidate *Policy) []string {
	var problems []string
	for _, source := range candidate.Packages {
		for _, baseSource := range base.Packages {
			if source.Path != baseSource.Path {
				continue
			}
			for _, scope := range allScopes() {
				for _, target := range candidate.Packages {
					for _, baseTarget := range base.Packages {
						if target.Path == baseTarget.Path && policyAllowsFirstParty(candidate, scope, source, target) && !policyAllowsFirstParty(base, scope, baseSource, baseTarget) {
							problems = append(problems, string(scope)+"|imports.forbidden|"+candidate.absolutePackage(source.Path)+"|"+candidate.absolutePackage(target.Path))
						}
					}
				}
				for _, target := range candidate.ExternalPackages {
					for _, baseTarget := range base.ExternalPackages {
						if target.Path == baseTarget.Path && policyAllowsExternal(candidate, scope, source, target) && !policyAllowsExternal(base, scope, baseSource, baseTarget) {
							problems = append(problems, string(scope)+"|imports.forbidden|"+candidate.absolutePackage(source.Path)+"|"+target.Path)
						}
					}
				}
			}
		}
	}
	return problems
}
