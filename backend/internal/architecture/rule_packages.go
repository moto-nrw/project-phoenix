package architecture

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Package selectors intersect the owner/role selectors; they never replace
// them. Paths are exact module-relative classifications, not prefixes/globs.
func (p *Policy) validateRulePackages(rule Rule) error {
	if rule.SourcePackage != "" {
		pkg, err := p.rulePackage(rule.ID, "source", rule.SourcePackage)
		if err != nil {
			return err
		}
		for _, scope := range rule.Scopes {
			if !p.matchesSource(rule, pkg.inScope(Scope(scope))) {
				return fmt.Errorf("rule %q source_package %q contradicts its owner/role selectors in %s", rule.ID, rule.SourcePackage, scope)
			}
		}
	}
	if rule.TargetPackage != "" {
		if rule.TargetClass != "" {
			return fmt.Errorf("rule %q target_package requires a first-party target", rule.ID)
		}
		pkg, err := p.rulePackage(rule.ID, "target", rule.TargetPackage)
		if err != nil {
			return err
		}
		if !p.matchesOwner(rule.TargetOwner, rule.TargetOwnerKind, pkg.Owner) || rule.TargetRole != pkg.Role {
			return fmt.Errorf("rule %q target_package %q contradicts its owner/role selectors", rule.ID, rule.TargetPackage)
		}
	}
	return nil
}

func (p *Policy) rulePackage(ruleID, side, selector string) (Package, error) {
	if selector != strings.TrimSpace(selector) || path.Clean(selector) != selector ||
		strings.HasPrefix(selector, "/") || strings.ContainsAny(selector, "*?[]\\") ||
		selector == ".." || strings.HasPrefix(selector, "../") {
		return Package{}, fmt.Errorf("rule %q %s_package %q must be an exact module-relative package path", ruleID, side, selector)
	}
	pkg, err := p.firstPartyPackage(p.absolutePackage(selector))
	if err != nil {
		return Package{}, fmt.Errorf("rule %q references unknown %s_package %q", ruleID, side, selector)
	}
	return pkg, nil
}

// overlappingScope intersects validated package classifications before comparing
// selectors. Source roles depend on scope; target roles always use production.
func (p *Policy) overlappingScope(left, right Rule, owners map[string]Owner) (string, bool) {
	if left.SourcePackage == "" && right.SourcePackage == "" && left.TargetPackage == "" && right.TargetPackage == "" {
		return overlappingScope(left, right, owners)
	}
	for _, scope := range left.Scopes {
		if !slices.Contains(right.Scopes, scope) {
			continue
		}
		if _, overlaps := overlappingScope(p.classifiedOverlapRule(left, scope), p.classifiedOverlapRule(right, scope), owners); overlaps {
			return scope, true
		}
	}
	return "", false
}

// classifiedOverlapRule is only used after package selectors have been validated
// against their owner/role selectors, so the exact classification can narrow them.
func (p *Policy) classifiedOverlapRule(rule Rule, scope string) Rule {
	rule.Scopes = []string{scope}
	for _, pkg := range p.Packages {
		if rule.SourcePackage == pkg.Path {
			rule.SourceOwner, rule.SourceOwnerKind = pkg.Owner, ""
			rule.SourceRole = pkg.inScope(Scope(scope)).Role
		}
		if rule.TargetPackage == pkg.Path {
			rule.TargetOwner, rule.TargetOwnerKind = pkg.Owner, ""
		}
	}
	return rule
}

func packageSelectorsOverlap(left, right string) bool {
	return left == "" || right == "" || left == right
}

func packageSelectorContains(base, candidate string) bool {
	return base == "" || base == candidate
}

// rulePermissionContains allows additional package restrictions on otherwise
// identical selectors. Removing or swapping a restriction is not narrowing.
func rulePermissionContains(base, candidate Rule) bool {
	if !packageSelectorContains(base.SourcePackage, candidate.SourcePackage) ||
		!packageSelectorContains(base.TargetPackage, candidate.TargetPackage) {
		return false
	}
	base.SourcePackage, candidate.SourcePackage = "", ""
	base.TargetPackage, candidate.TargetPackage = "", ""
	return sameRulePermission(base, candidate)
}

func (p *Policy) packageSelectedOwners(owners []string, selector string) []string {
	if selector == "" {
		return owners
	}
	pkg, err := p.firstPartyPackage(p.absolutePackage(selector))
	if err != nil || !slices.Contains(owners, pkg.Owner) {
		return nil
	}
	return []string{pkg.Owner}
}
