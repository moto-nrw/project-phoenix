package architecture

import "slices"

// reclassifiableRoles are the production-role changes of an existing package
// a reviewed epoch may make inside its owner (ADR 0057):
//
//   - adapter -> http: a shared runtime many owners legitimately use is
//     classified, not granted into a private role (point 3);
//   - domain -> legacy-model: the retained models/* packages get their own
//     role, which only their owner reaches (point 4).
var reclassifiableRoles = map[[2]string]bool{
	{"adapter", "http"}:         true,
	{"domain", legacyModelRole}: true,
}

// reclassifiedRolePermission reports whether a permission the base did not
// grant follows from an allowed reclassification of the source's or the
// target's production role. Into a reclassified package it holds when the
// base let the same source into the old role or into the new role of the
// owner. Out of a reclassified package it holds when the base let the old or
// the new role of the owner reach the target. A reclassified package thus has
// the reach its new role states, and keeps what its old role had. Owner
// changes, test-role changes and every other role change keep the ordinary
// loosening guards.
func reclassifiedRolePermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	sources := []Package{source}
	if scope == ScopeProduction {
		sources = append(sources, reclassifiedSourceRoles(base, candidate, source)...)
	}
	targets := []Package{target}
	for _, from := range reclassifiedFromRoles(base, candidate, target.Owner, target.Role) {
		targets = append(targets, Package{Owner: target.Owner, Role: from, InternalTestRole: from, ExternalTestRole: from})
	}
	if len(sources) == 1 && len(targets) == 1 {
		return false
	}
	for _, previousSource := range sources {
		for _, previousTarget := range targets {
			if policyAllowsFirstParty(base, scope, previousSource, previousTarget) {
				return true
			}
		}
	}
	return false
}

// reclassifiedSourceRoles returns the other production roles a reclassified
// source package is judged by: its old roles when source carries the new one
// (a rule point), or its new role when source is the base classification of
// a package the candidate reclassified (an import).
func reclassifiedSourceRoles(base, candidate *Policy, source Package) []Package {
	var alternatives []Package
	for _, from := range reclassifiedFromRoles(base, candidate, source.Owner, source.Role) {
		previous := source
		previous.Role = from
		alternatives = append(alternatives, previous)
	}
	if source.Path == "" {
		return alternatives
	}
	current, exists := candidate.packageMap()[candidate.absolutePackage(source.Path)]
	if exists && current.Owner == source.Owner && current.Role != source.Role &&
		slices.Contains(reclassifiedFromRoles(base, candidate, current.Owner, current.Role), source.Role) {
		next := source
		next.Role = current.Role
		alternatives = append(alternatives, next)
	}
	return alternatives
}

// reclassifiedFromRoles returns the base roles of packages the candidate
// moved into owner/role through an allowed reclassification. A role the base
// gives no package is new: it takes the old role's permissions for every
// owner, which is what a rule selecting owners by kind needs. The target-only
// validation keeps such a role private (legacy-model, ADR 0057).
func reclassifiedFromRoles(base, candidate *Policy, owner, role string) []string {
	basePackages := base.packageMap()
	newRole := true
	for _, pkg := range basePackages {
		if pkg.Role == role {
			newRole = false
			break
		}
	}
	seen := make(map[string]struct{})
	var roles []string
	for path, current := range candidate.packageMap() {
		if (current.Owner != owner && !newRole) || current.Role != role {
			continue
		}
		previous, exists := basePackages[path]
		if !exists || previous.Owner != current.Owner || previous.Role == role ||
			previous.InternalTestRole != current.InternalTestRole || previous.ExternalTestRole != current.ExternalTestRole ||
			!reclassifiableRoles[[2]string{previous.Role, role}] {
			continue
		}
		if _, duplicate := seen[previous.Role]; !duplicate {
			seen[previous.Role] = struct{}{}
			roles = append(roles, previous.Role)
		}
	}
	return roles
}
