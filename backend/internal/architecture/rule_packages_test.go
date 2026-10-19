package architecture

import (
	"strings"
	"testing"
)

func selectorPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy(fixturePath(t, "rules-live.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{1, 2} {
		pkg := p.Packages[index]
		pkg.Path += "-other"
		p.Packages = append(p.Packages, pkg)
	}
	return p
}

func TestPackageSelectorValidation(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"source_package", "target_package"} {
		for _, tc := range []struct {
			name  string
			value any
		}{
			{"empty", ""}, {"null", nil}, {"whitespace", " "}, {"not string", 3},
			{"unknown", "missing"}, {"absolute", "/source"}, {"glob", "source/*"},
			{"prefix", "source/"}, {"unclean", "other/../source"},
			{"module path", "example.test/architecture-fixture/source"},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				document := mutatePolicy(t, readFile(t, fixturePath(t, "rules-live.json")), func(doc map[string]any) {
					doc["rules"].([]any)[0].(map[string]any)[field] = tc.value
				})
				if _, err := DecodePolicy(strings.NewReader(document)); err == nil {
					t.Fatalf("accepted %s=%#v", field, tc.value)
				}
			})
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Policy)
	}{
		{"source role mismatch", func(p *Policy) {
			p.Rules[0].SourcePackage, p.Rules[0].SourceRole = "source", "domain"
			p.Rules[0].Scopes = []string{"production"}
		}},
		{"source owner mismatch", func(p *Policy) {
			p.Owners = append(p.Owners, Owner{ID: "other", Kind: "platform"})
			p.Rules[0].SourcePackage, p.Rules[0].SourceOwnerKind = "source", "platform"
		}},
		{"target role mismatch", func(p *Policy) { p.Rules[0].TargetPackage = "source" }},
		{"external target", func(p *Policy) {
			p.ExternalClasses = []string{"stdlib"}
			p.Rules[0].TargetPackage, p.Rules[0].TargetClass, p.Rules[0].TargetRole, p.Rules[0].SameOwner = "target", "stdlib", "", false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := selectorPolicy(t)
			tc.mutate(p)
			if err := p.Validate(); err == nil {
				t.Fatal("selector overrode owner/role/target restrictions")
			}
		})
	}
}

func TestPackageSelectorsNarrowMatchingAndMemoization(t *testing.T) {
	t.Parallel()
	p := selectorPolicy(t)
	p.Rules[0].SourcePackage, p.Rules[0].TargetPackage = "source", "target"
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	memo := newPolicyPermissionMemo(p)
	for _, scope := range allScopes() {
		for _, source := range []string{"source", "source-other"} {
			for _, target := range []string{"target", "target-other"} {
				s, _ := p.firstPartyPackage(p.absolutePackage(source))
				d, _ := p.firstPartyPackage(p.absolutePackage(target))
				want := source == "source" && target == "target"
				if got := policyAllowsFirstParty(p, scope, s, d); got != want {
					t.Fatalf("%s %s -> %s allowed=%v, want %v", scope, source, target, got, want)
				}
				if got := memo.allowsFirstParty(scope, s, d); got != want {
					t.Fatalf("%s %s -> %s memo=%v, want %v", scope, source, target, got, want)
				}
			}
		}
	}
	// A package selector is not a prefix that includes nested packages.
	s, _ := p.firstPartyPackage(p.absolutePackage("source"))
	d, _ := p.firstPartyPackage(p.absolutePackage("target"))
	s.Path = "source/nested"
	if memo.allowsFirstParty(ScopeProduction, s, d) {
		t.Fatal("selected source included a child package")
	}
	// The same source restriction applies to external dependency classes.
	p.Rules[0].TargetPackage, p.Rules[0].TargetRole, p.Rules[0].SameOwner = "", "", false
	p.Rules[0].TargetClass = "stdlib"
	memo = newPolicyPermissionMemo(p)
	for _, source := range []string{"source-other", "source"} {
		s, _ := p.firstPartyPackage(p.absolutePackage(source))
		if got := memo.allowsExternal(ScopeProduction, s, ExternalPackage{Class: "stdlib"}); got != (source == "source") {
			t.Fatalf("external source %s allowed=%v", source, got)
		}
	}
}

func TestPackageSelectorStrictness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                             string
		baseSource, baseTarget           string
		candidateSource, candidateTarget string
		allowed                          bool
	}{
		{"source narrows", "", "", "source", "", true},
		{"target narrows", "", "", "", "target", true},
		{"both narrow", "", "", "source", "target", true},
		{"unchanged selectors", "source", "target", "source", "target", true},
		{"remove source widens", "source", "target", "", "target", false},
		{"remove target widens", "source", "target", "source", "", false},
		{"change source widens", "source", "target", "source-other", "target", false},
		{"change target widens", "source", "target", "source", "target-other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := selectorPolicy(t), selectorPolicy(t)
			base.Rules[0].SourcePackage, base.Rules[0].TargetPackage = tc.baseSource, tc.baseTarget
			candidate.Rules[0].SourcePackage, candidate.Rules[0].TargetPackage = tc.candidateSource, tc.candidateTarget
			if err := base.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := candidate.Validate(); err != nil {
				t.Fatal(err)
			}
			err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil, nil)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, error=%v", tc.allowed, err)
			}
		})
	}
}

func TestPackageSelectorOverlap(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, target string
		overlap              bool
	}{
		{"distinct sources", "source-other", "target", false},
		{"distinct targets", "source", "target-other", false},
		{"same endpoints", "source", "target", true},
		{"unbounded source", "", "target", true},
		{"unbounded target", "source", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := selectorPolicy(t)
			p.Rules[0].SourcePackage, p.Rules[0].TargetPackage = "source", "target"
			r := p.Rules[0]
			r.ID, r.SourcePackage, r.TargetPackage = "second", tc.source, tc.target
			p.Rules = append(p.Rules, r)
			err := p.Validate()
			if (err != nil) != tc.overlap {
				t.Fatalf("overlap=%v, error=%v", tc.overlap, err)
			}
		})
	}
}
func TestPackageSelectorsPreserveTargetSafety(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, target, role string }{
		{"foreign private", "beta/application", "application"},
		{"foreign rows", "models/beta", "legacy-model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := acyclicFixture(t, acyclicTargetEpoch, func(doc map[string]any) {
				r := doc["rules"].([]any)[0].(map[string]any)
				r["source_package"], r["target_package"], r["target_role"] = "alpha/application", tc.target, tc.role
			})
			if err == nil {
				t.Fatal("package selector bypassed target-only validation")
			}
		})
	}
	err := acyclicFixture(t, acyclicTargetEpoch, func(doc map[string]any) {
		closeAlphaBetaCycle(doc)
		for _, raw := range doc["rules"].([]any) {
			r := raw.(map[string]any)
			if r["id"] == "beta.application.alpha-public" {
				r["source_package"], r["target_package"] = "beta/application", "alpha"
			}
		}
	})
	if err == nil || !strings.Contains(err.Error(), "cyclic component") {
		t.Fatalf("package-selected cycle accepted: %v", err)
	}
}
func TestPackageSelectorsNarrowOwnerProjection(t *testing.T) {
	t.Parallel()
	p, err := LoadPolicy(fixturePath(t, "target-only-rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := &p.Rules[0]
	r.SourceOwner, r.SourceOwnerKind, r.SourcePackage = "", "domain", "alpha/application"
	r.TargetOwner, r.TargetOwnerKind, r.TargetPackage = "", "domain", "beta"
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, edge := range TargetProjection(p).Edges {
		for _, id := range edge.Rules {
			if id != r.ID {
				continue
			}
			seen = true
			if edge.Source != "alpha" || edge.Target != "beta" {
				t.Fatalf("selector gained owner edge: %+v", edge)
			}
		}
	}
	if !seen {
		t.Fatal("selected alpha -> beta owner edge vanished")
	}
}

func TestPackageSelectorOverlapClassifications(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*Rule, *Rule)
		overlap bool
	}{
		{"source owner disjoint", func(a, b *Rule) {
			a.SourcePackage, b.SourceOwner, b.SourceOwnerKind = "alpha/application", "beta", ""
		}, false},
		{"source owner overlaps", func(a, b *Rule) {
			a.SourcePackage, b.SourceOwner, b.SourceOwnerKind = "alpha/application", "alpha", ""
		}, true},
		{"source role disjoint", func(a, b *Rule) {
			a.SourcePackage, b.SourceRole = "alpha/application", "public"
		}, false},
		{"source test role overlaps", func(a, b *Rule) {
			a.SourcePackage, b.SourceRole = "alpha/application", "module-internal-test"
			a.Scopes, b.Scopes = []string{"internal_test"}, []string{"internal_test"}
		}, true},
		{"later scope overlaps", func(a, b *Rule) {
			a.SourcePackage, b.SourceRole = "alpha/application", "module-behavior-test"
			a.Scopes, b.Scopes = []string{"production", "external_test"}, []string{"production", "external_test"}
		}, true},
		{"source test role disjoint", func(a, b *Rule) {
			a.SourcePackage, b.SourceRole = "alpha/application", "application"
			a.Scopes, b.Scopes = []string{"internal_test"}, []string{"internal_test"}
		}, false},
		{"target owner disjoint", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind = "domain", ""
			a.TargetPackage, b.TargetOwner = "alpha/public", "beta"
		}, false},
		{"target owner overlaps", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind = "domain", ""
			a.TargetPackage, b.TargetOwner = "alpha/public", "alpha"
		}, true},
		{"same owner source disjoint", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind = "", ""
			a.SameOwner = true
			a.SourcePackage, b.TargetOwner = "alpha/application", "beta"
		}, false},
		{"same owner target disjoint", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind = "domain", ""
			b.SameOwner = true
			a.TargetPackage, b.SourceOwner, b.SourceOwnerKind = "alpha/public", "beta", ""
		}, false},
		{"same owner overlaps", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind = "", ""
			a.SameOwner, b.SameOwner = true, true
			a.SourcePackage, b.TargetPackage = "alpha/application", "alpha/public"
		}, true},
		{"external source disjoint", func(a, b *Rule) {
			a.TargetOwnerKind, b.TargetOwnerKind, a.TargetRole, b.TargetRole = "", "", "", ""
			a.TargetClass, b.TargetClass = "stdlib", "stdlib"
			a.SourcePackage, b.SourceOwner, b.SourceOwnerKind = "alpha/application", "beta", ""
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := selectorPolicy(t)
			p.PolicyEpoch = targetOnlyRulesEpoch
			p.Owners = []Owner{{ID: "alpha", Kind: "domain"}, {ID: "beta", Kind: "domain"}, {ID: "runtime", Kind: "platform"}}
			p.Roles = append(p.Roles, "public")
			p.ExternalClasses = []string{"stdlib"}
			p.Packages = nil
			for _, owner := range p.Owners {
				for _, role := range []string{"application", "public"} {
					p.Packages = append(p.Packages, Package{
						Path: owner.ID + "/" + role, Owner: owner.ID, Role: role,
						InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
					})
				}
			}
			a := Rule{ID: "first", Description: "First permission.", Scopes: []string{"production"}, SourceOwnerKind: "domain", TargetOwnerKind: "platform", TargetRole: "public"}
			b := a
			b.ID = "second"
			tc.mutate(&a, &b)
			// Each rule must be valid independently; only overlap may reject the pair.
			for _, rule := range []Rule{a, b} {
				p.Rules = []Rule{rule}
				if err := p.Validate(); err != nil {
					t.Fatal(err)
				}
			}
			for _, rules := range [][]Rule{{a, b}, {b, a}} {
				p.Rules = rules
				err := p.Validate()
				if tc.overlap {
					if err == nil || !strings.Contains(err.Error(), "overlaps") {
						t.Fatalf("expected overlap rejection, got %v", err)
					}
				} else if err != nil {
					t.Fatalf("disjoint rules rejected: %v", err)
				}
			}
		})
	}
}
