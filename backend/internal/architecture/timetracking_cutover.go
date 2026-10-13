package architecture

import "strings"

const timeTrackingLegacyPath = "modules/workforce/legacy/timetracking"

// timeTrackingRetiredPackage is the exact classification the immutable base
// must carry for the #3413 exception to apply.
var timeTrackingRetiredPackage = Package{
	Path: timeTrackingLegacyPath, Owner: "workforce", Role: "adapter",
	InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test",
}

// timeTrackingCutoverPermission is the one-time #3413 dissolution of the
// retained time-tracking package into the Workforce application layer (ADR
// 0052): the services move to modules/workforce/internal/application/
// timetracking, and the adapters that served the public contracts from the
// composition root move into the Workforce composition with the private row
// mapping that replaces the work-session repository shim. Like the #3418 cutover it forgives no debt, changes no
// ownership, adds no external dependency and opens no owner internals to a
// stranger; ordinary graph checks still require the retired package to be
// absent from the tree.
//
// It grants one replacement, anchored on a permission the base policy
// already granted in the same scope: the Workforce roles that received the
// retired code inherit that code's own reach, and nothing more.
func timeTrackingCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !timeTrackingRetired(base, candidate) || !timeTrackingReceivingRole(scope, source) {
		return false
	}
	retired := base.packageMap()[base.ModulePath+"/"+timeTrackingLegacyPath]
	return policyAllowsFirstParty(base, scope, retired, target)
}

// timeTrackingReceivingRole lists the points that received the retired code:
// Workforce's application holds the services, its composition binds them
// and the row adapters, and the test roles are those of the moved suites.
func timeTrackingReceivingRole(scope Scope, source Package) bool {
	if source.Owner != "workforce" {
		return false
	}
	switch source.inScope(scope).Role {
	case "application", "compose", "module-internal-test", "module-behavior-test":
		return true
	}
	return false
}

func timeTrackingRetired(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	want := timeTrackingRetiredPackage
	legacy, exists := base.packageMap()[base.ModulePath+"/"+timeTrackingLegacyPath]
	if !exists || legacy.Owner != want.Owner || legacy.Role != want.Role ||
		legacy.InternalTestRole != want.InternalTestRole || legacy.ExternalTestRole != want.ExternalTestRole {
		return false
	}
	for _, pkg := range candidate.Packages {
		if pkg.Path == timeTrackingLegacyPath || strings.HasPrefix(pkg.Path, timeTrackingLegacyPath+"/") {
			return false
		}
	}
	return true
}
