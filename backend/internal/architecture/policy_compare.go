package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// CompareCandidatePolicyStrictness applies the strict base-policy comparison
// while allowing ownership declarations for tables that a new migration file
// in this candidate actually creates. Without this narrow exception the
// ratchet would freeze the schema: a table cannot exist in the base policy
// before the migration that introduces it. Existing tables and ownership
// changes remain strict policy loosenings.
func CompareCandidatePolicyStrictness(project, baseRef string, base, candidate *Policy) error {
	createdDataObjects, err := candidateMigrationDataObjects(project, baseRef)
	if err != nil {
		return err
	}
	createdPackages, err := candidateGoPackages(project, baseRef, candidate)
	if err != nil {
		return err
	}
	deletedLegacySymbols, err := candidateDeletedLegacySymbols(project, base, candidate)
	if err != nil {
		return err
	}
	return comparePolicyStrictness(base, candidate, createdDataObjects, createdPackages, deletedLegacySymbols)
}

func comparePolicyStrictness(base, candidate *Policy, createdDataObjects, createdPackages, deletedLegacySymbols map[string]struct{}) error {
	problems := modulePathLoosenings(base, candidate)
	problems = append(problems, ownershipLoosenings(base, candidate, createdDataObjects, createdPackages)...)
	problems = append(problems, classificationLoosenings(base, candidate, createdPackages)...)
	problems = append(problems, readProjectionLoosenings(base, candidate, createdPackages)...)
	problems = append(problems, compositionLoosenings(base, candidate, deletedLegacySymbols)...)
	problems = append(problems, importLoosenings(base, candidate)...)
	problems = append(problems, ruleLoosenings(base, candidate, candidateOnlyRolePoints(candidate, createdPackages))...)
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("architecture policy loosening detected (%d):\n%s", len(problems), strings.Join(problems, "\n"))
}

func modulePathLoosenings(base, candidate *Policy) []string {
	if candidate.ModulePath == base.ModulePath {
		return nil
	}
	return []string{fmt.Sprintf("module_path changed from %s to %s", base.ModulePath, candidate.ModulePath)}
}

func ownershipLoosenings(base, candidate *Policy, createdDataObjects, createdPackages map[string]struct{}) []string {
	var problems []string
	baseOwners := ownersByID(base)
	candidateOwners := ownersByID(candidate)
	for id, owner := range candidateOwners {
		if _, exists := baseOwners[id]; !exists {
			if owner.Kind != "projection" || !ownerPackagesAreCandidateCreated(candidate, id, createdPackages) {
				problems = append(problems, fmt.Sprintf("owner %s with kind %s was added", id, owner.Kind))
			}
		}
	}
	for id, baseOwner := range baseOwners {
		if current, exists := candidateOwners[id]; exists && current.Kind != baseOwner.Kind {
			problems = append(problems, fmt.Sprintf("owner %s changed kind from %s to %s", id, baseOwner.Kind, current.Kind))
		}
	}
	baseObjects := dataObjectsByName(base)
	for name, current := range dataObjectsByName(candidate) {
		baseObject, exists := baseObjects[name]
		_, createdByCandidateMigration := createdDataObjects[name]
		if !exists {
			if !createdByCandidateMigration {
				problems = append(problems, fmt.Sprintf("data object %s was newly assigned to owner %s", name, current.WriteOwner))
			}
			continue
		}
		if current.WriteOwner != baseObject.WriteOwner {
			problems = append(problems, fmt.Sprintf("data object %s changed write owner from %s to %s", name, baseObject.WriteOwner, current.WriteOwner))
		}
	}
	return problems
}

func classificationLoosenings(base, candidate *Policy, createdPackages map[string]struct{}) []string {
	basePackages := base.packageMap()
	var problems []string
	for path, current := range candidate.packageMap() {
		previous, exists := basePackages[path]
		if !exists {
			if _, created := createdPackages[path]; !created {
				problems = append(problems, fmt.Sprintf("package %s was newly classified", path))
			}
			continue
		}
		if current.Owner != previous.Owner {
			problems = append(problems, fmt.Sprintf("package %s changed owner from %s to %s", path, previous.Owner, current.Owner))
		}
		for _, roles := range []struct {
			scope     string
			base      string
			candidate string
		}{
			{scope: "production", base: previous.Role, candidate: current.Role},
			{scope: "internal_test", base: previous.InternalTestRole, candidate: current.InternalTestRole},
			{scope: "external_test", base: previous.ExternalTestRole, candidate: current.ExternalTestRole},
		} {
			if semanticRoleLoosens(roles.base, roles.candidate) {
				problems = append(problems, fmt.Sprintf("package %s changed %s role from %s to %s and disables semantic checks", path, roles.scope, roles.base, roles.candidate))
			}
		}
	}
	baseExternal := base.externalPackageMap()
	for path, current := range candidate.externalPackageMap() {
		previous, exists := baseExternal[path]
		if !exists {
			problems = append(problems, fmt.Sprintf("external package %s was newly classified as %s", path, current.Class))
		} else if current.Class != previous.Class {
			problems = append(problems, fmt.Sprintf("external package %s changed class from %s to %s", path, previous.Class, current.Class))
		}
	}
	return problems
}

func semanticRoleLoosens(base, candidate string) bool {
	baseContract := base == "public" || base == "contract"
	candidateContract := candidate == "public" || candidate == "contract"
	baseRuntime := base != "migration" && base != "test-support"
	candidateRuntime := candidate != "migration" && candidate != "test-support"
	baseDirectDB := base == "postgres" || base == "migration" || base == "test-support"
	candidateDirectDB := candidate == "postgres" || candidate == "migration" || candidate == "test-support"
	return (baseContract && !candidateContract) || (baseRuntime && !candidateRuntime) || (!baseDirectDB && candidateDirectDB)
}

func ownerPackagesAreCandidateCreated(candidate *Policy, owner string, createdPackages map[string]struct{}) bool {
	found := false
	for path, pkg := range candidate.packageMap() {
		if pkg.Owner != owner {
			continue
		}
		found = true
		if _, created := createdPackages[path]; !created {
			return false
		}
	}
	return found
}

func readProjectionLoosenings(base, candidate *Policy, createdPackages map[string]struct{}) []string {
	baseGrants := projectionGrants(base)
	baseOwners := ownersByID(base)
	candidatePackages := candidate.packageMap()
	candidateOwners := ownersByID(candidate)
	var problems []string
	for grant := range projectionGrants(candidate) {
		if _, exists := baseGrants[grant]; !exists {
			packagePath := strings.SplitN(grant, "|", 2)[0]
			pkg, classified := candidatePackages[packagePath]
			_, packageCreated := createdPackages[packagePath]
			owner, ownerExists := candidateOwners[pkg.Owner]
			_, ownerExisted := baseOwners[pkg.Owner]
			if !classified || !packageCreated || !ownerExists || ownerExisted || owner.Kind != "projection" {
				problems = append(problems, "new tenant-safe read projection grant "+grant)
			}
		}
	}
	return problems
}

func compositionLoosenings(base, candidate *Policy, deletedSymbols map[string]struct{}) []string {
	candidateSymbols := compositionSymbols(candidate)
	var problems []string
	for symbol := range compositionSymbols(base) {
		if _, exists := candidateSymbols[symbol]; !exists {
			if _, deleted := deletedSymbols[symbol]; !deleted {
				problems = append(problems, "legacy composition symbol is no longer guarded: "+symbol)
			}
		}
	}
	return problems
}

func importLoosenings(base, candidate *Policy) []string {
	basePackages := base.packageMap()
	candidatePackages := candidate.packageMap()
	var problems []string
	for sourcePath, source := range candidatePackages {
		baseSource, sourceExists := basePackages[sourcePath]
		if !sourceExists {
			continue
		}
		problems = append(problems, firstPartyImportLoosenings(base, candidate, sourcePath, baseSource, source, basePackages, candidatePackages)...)
		problems = append(problems, externalImportLoosenings(base, candidate, sourcePath, baseSource, source)...)
	}
	return uniqueStrings(problems)
}

func ruleLoosenings(base, candidate *Policy, candidateOnlyPoints map[string]struct{}) []string {
	owners := ruleUniverseOwners(base, candidate)
	roles := sortedAllowedRoles()
	baseEvaluator := policyWithOwners(base, owners)
	candidateEvaluator := policyWithOwners(candidate, owners)
	var problems []string
	for _, rule := range candidate.Rules {
		if candidateRuleCoveredDirectly(rule, base.Rules) {
			continue
		}
		if problem := uncoveredRulePermission(rule, baseEvaluator, candidateEvaluator, owners, roles); problem != "" {
			if !ruleAnchoredToCandidateOnlyPoint(rule, candidateOnlyPoints) {
				problems = append(problems, problem)
			}
		}
	}
	return problems
}

func candidateOnlyRolePoints(candidate *Policy, createdPackages map[string]struct{}) map[string]struct{} {
	created := make(map[string]struct{})
	existing := make(map[string]struct{})
	for path, pkg := range candidate.packageMap() {
		target := existing
		if _, ok := createdPackages[path]; ok {
			target = created
		}
		for _, scope := range allScopes() {
			scoped := pkg.inScope(scope)
			target[rolePointKey("source", scope, scoped.Owner, scoped.Role)] = struct{}{}
			target[rolePointKey("target", scope, pkg.Owner, pkg.Role)] = struct{}{}
		}
	}
	for point := range existing {
		delete(created, point)
	}
	return created
}

func ruleAnchoredToCandidateOnlyPoint(rule Rule, candidateOnlyPoints map[string]struct{}) bool {
	if rule.SourceOwnerKind != "" || rule.TargetOwnerKind != "" || rule.SourceOwner == "" || rule.SourceRole == "" {
		return false
	}
	for _, rawScope := range rule.Scopes {
		scope := Scope(rawScope)
		_, sourceCreated := candidateOnlyPoints[rolePointKey("source", scope, rule.SourceOwner, rule.SourceRole)]
		targetOwner := rule.TargetOwner
		if targetOwner == "" && rule.SameOwner {
			targetOwner = rule.SourceOwner
		}
		_, targetCreated := candidateOnlyPoints[rolePointKey("target", scope, targetOwner, rule.TargetRole)]
		if !sourceCreated && (rule.TargetClass != "" || !targetCreated) {
			return false
		}
	}
	return true
}

func rolePointKey(direction string, scope Scope, owner, role string) string {
	return direction + "|" + string(scope) + "|" + owner + "|" + role
}

func candidateRuleCoveredDirectly(candidate Rule, baseRules []Rule) bool {
	for _, base := range baseRules {
		if sameRulePermission(candidate, base) && scopesContain(base.Scopes, candidate.Scopes) {
			return true
		}
	}
	return false
}

func sameRulePermission(left, right Rule) bool {
	return left.SourceOwner == right.SourceOwner && left.SourceOwnerKind == right.SourceOwnerKind && left.SourceRole == right.SourceRole &&
		left.TargetOwner == right.TargetOwner && left.TargetOwnerKind == right.TargetOwnerKind && left.TargetRole == right.TargetRole &&
		left.TargetClass == right.TargetClass && left.SameOwner == right.SameOwner
}

func scopesContain(container, values []string) bool {
	for _, value := range values {
		if !sliceContains(container, value) {
			return false
		}
	}
	return true
}

func sliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uncoveredRulePermission(rule Rule, base, candidate *Policy, owners []Owner, roles []string) string {
	for _, rawScope := range rule.Scopes {
		scope := Scope(rawScope)
		for _, source := range matchingOwnerRoles(candidate, rule, owners, roles) {
			if rule.TargetClass != "" {
				if !policyAllowsExternal(base, scope, rulePointPackage(source), ExternalPackage{Class: rule.TargetClass}) {
					return fmt.Sprintf("rule %s newly allows %s %s/%s -> external class %s", rule.ID, scope, source.Owner.ID, source.Role, rule.TargetClass)
				}
				continue
			}
			if problem := uncoveredFirstPartyPermission(rule, base, candidate, scope, source, owners); problem != "" {
				return problem
			}
		}
	}
	return ""
}

type ownerRole struct {
	Owner Owner
	Role  string
}

func matchingOwnerRoles(policy *Policy, rule Rule, owners []Owner, roles []string) []ownerRole {
	var matches []ownerRole
	for _, owner := range owners {
		for _, candidateRole := range roles {
			point := ownerRole{Owner: owner, Role: candidateRole}
			if policy.matchesSource(rule, rulePointPackage(point)) {
				matches = append(matches, point)
			}
		}
	}
	return matches
}

func uncoveredFirstPartyPermission(rule Rule, base, candidate *Policy, scope Scope, source ownerRole, owners []Owner) string {
	sourcePackage := rulePointPackage(source)
	for _, target := range owners {
		targetPackage := rulePointPackage(ownerRole{Owner: target, Role: rule.TargetRole})
		if !candidate.matchesTarget(rule, sourcePackage, targetPackage) {
			continue
		}
		if !policyAllowsFirstParty(base, scope, sourcePackage, targetPackage) {
			return fmt.Sprintf("rule %s newly allows %s %s/%s -> %s/%s", rule.ID, scope, source.Owner.ID, source.Role, target.ID, rule.TargetRole)
		}
	}
	return ""
}

func rulePointPackage(point ownerRole) Package {
	return Package{Owner: point.Owner.ID, Role: point.Role, InternalTestRole: point.Role, ExternalTestRole: point.Role}
}

func ruleUniverseOwners(base, candidate *Policy) []Owner {
	byKey := make(map[string]Owner)
	for _, owner := range append(append([]Owner{}, base.Owners...), candidate.Owners...) {
		byKey[owner.ID+"|"+owner.Kind] = owner
	}
	for kind := range allowedOwnerKinds {
		owner := Owner{ID: "__kind_" + kind, Kind: kind}
		byKey[owner.ID+"|"+owner.Kind] = owner
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	owners := make([]Owner, 0, len(keys))
	for _, key := range keys {
		owners = append(owners, byKey[key])
	}
	return owners
}

func policyWithOwners(policy *Policy, owners []Owner) *Policy {
	policyCopy := *policy
	policyCopy.Owners = owners
	return &policyCopy
}

func sortedAllowedRoles() []string {
	roles := make([]string, 0, len(allowedRoles))
	for role := range allowedRoles {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func firstPartyImportLoosenings(base, candidate *Policy, sourcePath string, baseSource, source Package, basePackages, candidatePackages map[string]Package) []string {
	var problems []string
	for targetPath, target := range candidatePackages {
		baseTarget, exists := basePackages[targetPath]
		if !exists {
			continue
		}
		for _, scope := range allScopes() {
			if policyAllowsFirstParty(candidate, scope, source, target) && !policyAllowsFirstParty(base, scope, baseSource, baseTarget) {
				problems = append(problems, Violation{Scope: scope, Rule: "imports.forbidden", Source: sourcePath, Target: targetPath}.Key())
			}
		}
	}
	return problems
}

func externalImportLoosenings(base, candidate *Policy, sourcePath string, baseSource, source Package) []string {
	baseExternal := base.externalPackageMap()
	var problems []string
	for targetPath, target := range candidate.externalPackageMap() {
		baseTarget, exists := baseExternal[targetPath]
		if !exists {
			continue
		}
		for _, scope := range allScopes() {
			if policyAllowsExternal(candidate, scope, source, target) && !policyAllowsExternal(base, scope, baseSource, baseTarget) {
				problems = append(problems, Violation{Scope: scope, Rule: "imports.forbidden", Source: sourcePath, Target: targetPath}.Key())
			}
		}
	}
	return problems
}

func policyAllowsFirstParty(policy *Policy, scope Scope, source, target Package) bool {
	source = source.inScope(scope)
	decision := decideRules(policy.firstPartyRules(scope, source, target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0
}

func policyAllowsExternal(policy *Policy, scope Scope, source Package, target ExternalPackage) bool {
	source = source.inScope(scope)
	decision := decideRules(policy.externalRules(scope, source, target.Class))
	return decision.Allowed != nil && len(decision.Overlaps) == 0
}

func ownersByID(policy *Policy) map[string]Owner {
	result := make(map[string]Owner, len(policy.Owners))
	for _, owner := range policy.Owners {
		result[owner.ID] = owner
	}
	return result
}

func dataObjectsByName(policy *Policy) map[string]DataObject {
	result := make(map[string]DataObject, len(policy.DataObjects))
	for _, object := range policy.DataObjects {
		result[object.Name] = object
	}
	return result
}

func projectionGrants(policy *Policy) map[string]struct{} {
	result := make(map[string]struct{})
	for _, projection := range policy.ReadProjections {
		for _, object := range projection.DataObjects {
			result[policy.absolutePackage(projection.Package)+"|"+object] = struct{}{}
		}
	}
	return result
}

func compositionSymbols(policy *Policy) map[string]struct{} {
	result := make(map[string]struct{})
	for _, legacy := range policy.LegacyComposition {
		for _, symbol := range legacy.Symbols {
			result[policy.absolutePackage(legacy.Package)+"."+symbol] = struct{}{}
		}
	}
	return result
}

func allScopes() []Scope {
	return []Scope{ScopeProduction, ScopeInternalTest, ScopeExternalTest}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
