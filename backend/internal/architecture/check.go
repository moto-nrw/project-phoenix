package architecture

import (
	"fmt"
	"sort"
	"strings"
)

type Violation struct {
	Scope     Scope      `json:"scope"`
	Rule      string     `json:"rule"`
	Source    string     `json:"source"`
	Target    string     `json:"target"`
	Detail    string     `json:"-"`
	Locations []Location `json:"-"`
}

// Location identifies one source declaration that causes a violation.
type Location struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Declaration string `json:"declaration"`
}

func (v Violation) Key() string {
	return fmt.Sprintf("%s|%s|%s|%s", v.Scope, v.Rule, v.Source, v.Target)
}

func Check(policy *Policy, graph *Graph) []Violation {
	packages := policy.packageMap()
	externalPackages := policy.externalPackageMap()
	violations := packageClassificationViolations(packages, graph.Packages, graph.PackageLocations)
	violations = append(violations, graph.SemanticViolations...)
	usedExternalPackages := make(map[string]struct{}, len(externalPackages))
	for _, edge := range graph.Edges {
		if violation := checkEdge(policy, packages, externalPackages, usedExternalPackages, edge); violation != nil {
			violation.Locations = append(violation.Locations, graph.ImportLocations[edge]...)
			violations = append(violations, *violation)
		}
	}
	violations = append(violations, staleExternalViolations(externalPackages, usedExternalPackages)...)
	return uniqueSortedViolations(violations)
}

func uniqueSortedViolations(violations []Violation) []Violation {
	byKey := make(map[string]Violation, len(violations))
	for _, violation := range violations {
		key := violation.Key()
		previous, exists := byKey[key]
		if !exists {
			byKey[key] = violation
			continue
		}
		previous.Locations = append(previous.Locations, violation.Locations...)
		if violation.Detail < previous.Detail {
			previous.Detail = violation.Detail
		}
		byKey[key] = previous
	}
	result := make([]Violation, 0, len(byKey))
	for _, violation := range byKey {
		violation.Locations = uniqueSortedLocations(violation.Locations)
		result = append(result, violation)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}

func uniqueSortedLocations(locations []Location) []Location {
	byKey := make(map[string]Location, len(locations))
	for _, location := range locations {
		key := fmt.Sprintf("%s\x00%d\x00%s", location.File, location.Line, location.Declaration)
		byKey[key] = location
	}
	result := make([]Location, 0, len(byKey))
	for _, location := range byKey {
		result = append(result, location)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Declaration < result[j].Declaration
	})
	return result
}

func (p *Policy) packageMap() map[string]Package {
	packages := make(map[string]Package, len(p.Packages))
	for _, pkg := range p.Packages {
		packages[p.absolutePackage(pkg.Path)] = pkg
	}
	return packages
}

func (p *Policy) externalPackageMap() map[string]ExternalPackage {
	packages := make(map[string]ExternalPackage, len(p.ExternalPackages))
	for _, pkg := range p.ExternalPackages {
		packages[pkg.Path] = pkg
	}
	return packages
}

func packageClassificationViolations(policyPackages map[string]Package, graphPackages []string, locations map[string][]Location) []Violation {
	graphSet := make(map[string]struct{}, len(graphPackages))
	var violations []Violation
	for _, path := range graphPackages {
		graphSet[path] = struct{}{}
		if _, ok := policyPackages[path]; !ok {
			violations = append(violations, Violation{
				Scope: ScopeProduction, Rule: "packages.unclassified", Source: path, Target: path,
				Detail: "first-party package has no owner or role", Locations: locations[path],
			})
		}
	}
	for path := range policyPackages {
		if _, ok := graphSet[path]; !ok {
			violations = append(violations, Violation{Scope: ScopeProduction, Rule: "packages.stale", Source: path, Target: path, Detail: "policy classification does not match a package in the fixed build context"})
		}
	}
	return violations
}

func checkEdge(policy *Policy, packages map[string]Package, external map[string]ExternalPackage, used map[string]struct{}, edge Edge) *Violation {
	source, sourceOK := packages[edge.Source]
	if !sourceOK {
		return nil
	}
	source = source.inScope(edge.Scope)
	if !isOwnPackage(policy.ModulePath, edge.Target) {
		used[edge.Target] = struct{}{}
		return checkExternalEdge(policy, source, external, edge)
	}
	target, targetOK := packages[edge.Target]
	if !targetOK {
		return nil
	}
	decision := decideRules(policy.firstPartyRules(edge.Scope, source, target))
	if len(decision.Overlaps) > 0 {
		violation := overlappingRulesViolation(edge, decision.Overlaps)
		return &violation
	}
	if decision.Allowed == nil {
		return &Violation{Scope: edge.Scope, Rule: "imports.forbidden", Source: edge.Source, Target: edge.Target, Detail: fmt.Sprintf("%s/%s may not import %s/%s", source.Owner, source.Role, target.Owner, target.Role)}
	}
	return nil
}

func checkExternalEdge(policy *Policy, source Package, packages map[string]ExternalPackage, edge Edge) *Violation {
	target, ok := packages[edge.Target]
	if !ok {
		return &Violation{Scope: edge.Scope, Rule: "external.unclassified", Source: edge.Source, Target: edge.Target, Detail: "external import has no dependency class"}
	}
	decision := decideRules(policy.externalRules(edge.Scope, source, target.Class))
	if len(decision.Overlaps) > 0 {
		violation := overlappingRulesViolation(edge, decision.Overlaps)
		return &violation
	}
	if decision.Allowed == nil {
		return &Violation{Scope: edge.Scope, Rule: "imports.forbidden", Source: edge.Source, Target: edge.Target, Detail: fmt.Sprintf("%s/%s may not import external class %s", source.Owner, source.Role, target.Class)}
	}
	return nil
}

type ruleDecision struct {
	Allowed  *Rule
	Overlaps []*Rule
}

func decideRules(matches []*Rule) ruleDecision {
	switch len(matches) {
	case 0:
		return ruleDecision{}
	case 1:
		return ruleDecision{Allowed: matches[0]}
	default:
		return ruleDecision{Overlaps: matches}
	}
}

func staleExternalViolations(policyPackages map[string]ExternalPackage, used map[string]struct{}) []Violation {
	var violations []Violation
	for path := range policyPackages {
		if _, ok := used[path]; !ok {
			violations = append(violations, Violation{Scope: ScopeProduction, Rule: "external.stale", Source: path, Target: path, Detail: "external dependency classification is not used in the fixed build context"})
		}
	}
	return violations
}

func overlappingRulesViolation(edge Edge, rules []*Rule) Violation {
	return Violation{
		Scope: edge.Scope, Rule: "policy.rules-overlap", Source: edge.Source, Target: edge.Target,
		Detail: "edge matches multiple rules: " + strings.Join(sortedRuleIDs(rules), ", "),
	}
}

func (p *Policy) externalRules(scope Scope, source Package, targetClass string) []*Rule {
	var matches []*Rule
	for i := range p.Rules {
		rule := &p.Rules[i]
		if !p.matchesSource(*rule, source) || rule.TargetClass != targetClass {
			continue
		}
		if ruleHasScope(*rule, scope) {
			matches = append(matches, rule)
		}
	}
	return matches
}

func FormatViolations(violations []Violation) error {
	if len(violations) == 0 {
		return nil
	}
	lines := make([]string, 0, len(violations))
	for _, violation := range violations {
		lines = appendViolationLines(lines, "", violation)
	}
	return fmt.Errorf("architecture check failed with %d violation(s):\n%s", len(violations), strings.Join(lines, "\n"))
}

func appendViolationLines(lines []string, indent string, violation Violation) []string {
	lines = append(lines, indent+violation.Key()+" -- "+violation.Detail)
	for _, location := range violation.Locations {
		lines = append(lines, fmt.Sprintf("%s  at %s:%d (%s)", indent, location.File, location.Line, location.Declaration))
	}
	return lines
}

func (p *Policy) absolutePackage(path string) string {
	if path == "." || path == "" {
		return p.ModulePath
	}
	return p.ModulePath + "/" + strings.TrimPrefix(path, "/")
}

func (p *Policy) firstPartyRules(scope Scope, source, target Package) []*Rule {
	var matches []*Rule
	for i := range p.Rules {
		rule := &p.Rules[i]
		if !p.matchesSource(*rule, source) || rule.TargetClass != "" || !p.matchesTarget(*rule, source, target) {
			continue
		}
		if ruleHasScope(*rule, scope) {
			matches = append(matches, rule)
		}
	}
	return matches
}

func (p *Policy) Explain(scope Scope, sourcePath, targetPath string) (string, error) {
	if !allowedScopes[string(scope)] {
		return "", fmt.Errorf("invalid scope %q", scope)
	}
	source, err := p.firstPartyPackage(sourcePath)
	if err != nil {
		return "", err
	}
	source = source.inScope(scope)
	if isOwnPackage(p.ModulePath, targetPath) {
		return p.explainFirstParty(scope, sourcePath, targetPath, source)
	}
	return p.explainExternal(scope, sourcePath, targetPath, source)
}

func (p Package) inScope(scope Scope) Package {
	switch scope {
	case ScopeInternalTest:
		p.Role = p.InternalTestRole
	case ScopeExternalTest:
		p.Role = p.ExternalTestRole
	}
	return p
}

func (p *Policy) explainFirstParty(scope Scope, sourcePath, targetPath string, source Package) (string, error) {
	target, err := p.firstPartyPackage(targetPath)
	if err != nil {
		return "", err
	}
	decision := decideRules(p.firstPartyRules(scope, source, target))
	if len(decision.Overlaps) > 0 {
		return "", overlappingRulesError(decision.Overlaps)
	}
	if decision.Allowed != nil {
		rule := decision.Allowed
		return fmt.Sprintf("ALLOWED %s -> %s in %s by rule %s: %s", sourcePath, targetPath, scope, rule.ID, rule.Description), nil
	}
	return fmt.Sprintf("FORBIDDEN %s -> %s in %s by rule imports.forbidden: %s/%s may not import %s/%s", sourcePath, targetPath, scope, source.Owner, source.Role, target.Owner, target.Role), nil
}

func (p *Policy) explainExternal(scope Scope, sourcePath, targetPath string, source Package) (string, error) {
	for _, external := range p.ExternalPackages {
		if external.Path != targetPath {
			continue
		}
		decision := decideRules(p.externalRules(scope, source, external.Class))
		if len(decision.Overlaps) > 0 {
			return "", overlappingRulesError(decision.Overlaps)
		}
		if decision.Allowed != nil {
			rule := decision.Allowed
			return fmt.Sprintf("ALLOWED %s -> %s in %s by rule %s: %s", sourcePath, targetPath, scope, rule.ID, rule.Description), nil
		}
		return fmt.Sprintf("FORBIDDEN %s -> %s in %s by rule imports.forbidden: %s/%s may not import external class %s", sourcePath, targetPath, scope, source.Owner, source.Role, external.Class), nil
	}
	return "", fmt.Errorf("target package %q has no external dependency class", targetPath)
}

func overlappingRulesError(rules []*Rule) error {
	return fmt.Errorf("edge matches overlapping rules: %s", strings.Join(sortedRuleIDs(rules), ", "))
}

func sortedRuleIDs(rules []*Rule) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	sort.Strings(ids)
	return ids
}

func (p *Policy) matchesSource(rule Rule, source Package) bool {
	return p.matchesOwner(rule.SourceOwner, rule.SourceOwnerKind, source.Owner) &&
		(rule.SourceRole == "" || rule.SourceRole == source.Role)
}

func (p *Policy) matchesTarget(rule Rule, source, target Package) bool {
	if rule.SameOwner && source.Owner != target.Owner {
		return false
	}
	return p.matchesOwner(rule.TargetOwner, rule.TargetOwnerKind, target.Owner) && rule.TargetRole == target.Role
}

func (p *Policy) matchesOwner(wantOwner, wantKind, actualOwner string) bool {
	if wantOwner != "" && wantOwner != actualOwner {
		return false
	}
	if wantKind == "" {
		return true
	}
	for _, owner := range p.Owners {
		if owner.ID == actualOwner {
			return owner.Kind == wantKind
		}
	}
	return false
}

func ruleHasScope(rule Rule, scope Scope) bool {
	for _, allowedScope := range rule.Scopes {
		if allowedScope == string(scope) {
			return true
		}
	}
	return false
}

func (p *Policy) firstPartyPackage(path string) (Package, error) {
	for _, pkg := range p.Packages {
		if p.absolutePackage(pkg.Path) == path {
			return pkg, nil
		}
	}
	return Package{}, fmt.Errorf("first-party package %q has no owner or role", path)
}
