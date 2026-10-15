package architecture

import (
	"slices"
	"strings"
)

// sessionTokenRuntimePath is the package ADR 0057 point 3 classifies as
// Identity & Access's http role.
const sessionTokenRuntimePath = "modules/identityaccess/sessiontoken"

// roleReclassification is one decided production-role change of existing
// packages (ADR 0057, #4070).
type roleReclassification struct {
	matches  func(path string) bool
	from, to string
}

// decidedReclassifications are the only role changes the comparison carries
// over, each bound to its packages: the session token runtime from adapter to
// http (point 3) and the retained models from domain to legacy-model (point
// 4). They apply only in the epoch that introduces them, to a package the
// base still classifies in the old role.
var decidedReclassifications = []roleReclassification{
	{matches: func(path string) bool { return path == sessionTokenRuntimePath }, from: "adapter", to: "http"},
	{matches: func(path string) bool {
		return path == legacyModelPath || strings.HasPrefix(path, legacyModelPath+"/")
	}, from: "domain", to: legacyModelRole},
}

// reclassifiedPackage is a package the candidate moves through a decided
// reclassification: same owner, same test roles, old role in the base, new
// role in the candidate.
type reclassifiedPackage struct {
	path, owner, from, to string
}

func activeReclassifications(base, candidate *Policy) []reclassifiedPackage {
	if candidate.PolicyEpoch != targetOnlyRulesEpoch || base.PolicyEpoch >= candidate.PolicyEpoch {
		return nil
	}
	basePackages := base.packageMap()
	var moved []reclassifiedPackage
	for _, current := range candidate.Packages {
		previous, exists := basePackages[base.absolutePackage(current.Path)]
		if !exists || previous.Owner != current.Owner ||
			previous.InternalTestRole != current.InternalTestRole || previous.ExternalTestRole != current.ExternalTestRole {
			continue
		}
		for _, decided := range decidedReclassifications {
			if decided.matches(current.Path) && previous.Role == decided.from && current.Role == decided.to {
				moved = append(moved, reclassifiedPackage{path: current.Path, owner: current.Owner, from: decided.from, to: decided.to})
			}
		}
	}
	return moved
}

// reclassifiedRolePermission reports whether a permission the base did not
// grant follows from a decided reclassification.
//
// Into the new role of an owner it holds when the base let the same source
// into the old role a reclassified package of that owner had, or into the
// new role itself: the reclassified package is reachable by exactly those
// sources, and the owner's other packages in the new role become reachable
// for them too, which is what the classification states.
//
// Out of a package it holds only for a reclassified package itself, judged
// by its old role or by the new role's reach. For a rule point the owner's
// new role must consist of reclassified packages only; an unchanged package
// in that role keeps its own base permissions.
func reclassifiedRolePermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	moved := activeReclassifications(base, candidate)
	if len(moved) == 0 {
		return false
	}
	sources := []Package{source}
	if scope == ScopeProduction {
		sources = append(sources, reclassifiedSources(candidate, moved, source)...)
	}
	targets := []Package{target}
	for _, from := range reclassifiedTargetRoles(candidate, moved, target) {
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

// reclassifiedTargetRoles returns the old roles of the packages moved into
// target's owner and role. A role the base gives no package (legacy-model)
// is new for every owner, so a point selected by owner kind maps as well.
func reclassifiedTargetRoles(candidate *Policy, moved []reclassifiedPackage, target Package) []string {
	newRole := roleIsNew(candidate, moved, target.Role)
	var roles []string
	for _, pkg := range moved {
		if pkg.to == target.Role && (pkg.owner == target.Owner || newRole) && !slices.Contains(roles, pkg.from) {
			roles = append(roles, pkg.from)
		}
	}
	return roles
}

// reclassifiedSources returns the other packages a source is judged as. An
// import names the base classification of a package: only a reclassified
// package is also judged by its new role. A rule point names an owner and a
// role: it maps to the old role only when every candidate package of that
// owner in the role was reclassified into it.
func reclassifiedSources(candidate *Policy, moved []reclassifiedPackage, source Package) []Package {
	var alternatives []Package
	if source.Path != "" {
		for _, pkg := range moved {
			if pkg.path == source.Path && pkg.from == source.Role {
				next := source
				next.Role = pkg.to
				alternatives = append(alternatives, next)
			}
		}
		return alternatives
	}
	newRole := roleIsNew(candidate, moved, source.Role)
	if !newRole {
		for _, pkg := range candidate.Packages {
			if pkg.Owner == source.Owner && pkg.Role == source.Role && !isReclassified(moved, pkg.Path) {
				return nil
			}
		}
	}
	for _, pkg := range moved {
		if pkg.to == source.Role && (pkg.owner == source.Owner || newRole) {
			previous := source
			previous.Role = pkg.from
			alternatives = append(alternatives, previous)
		}
	}
	return alternatives
}

// roleIsNew reports whether every candidate package in role was reclassified
// into it, so the role holds nothing an older permission did not cover.
func roleIsNew(candidate *Policy, moved []reclassifiedPackage, role string) bool {
	found := false
	for _, pkg := range candidate.Packages {
		if pkg.Role != role {
			continue
		}
		if !isReclassified(moved, pkg.Path) {
			return false
		}
		found = true
	}
	return found
}

func isReclassified(moved []reclassifiedPackage, path string) bool {
	for _, pkg := range moved {
		if pkg.path == path {
			return true
		}
	}
	return false
}
