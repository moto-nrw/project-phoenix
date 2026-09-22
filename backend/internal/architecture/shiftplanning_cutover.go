package architecture

import "strings"

const shiftPlanningLegacyPath = "modules/workforce/legacy/shiftplanning"

// shiftPlanningRetiredPackage is the exact classification the immutable base
// must carry for the #3418 exception to apply.
var shiftPlanningRetiredPackage = Package{
	Path: shiftPlanningLegacyPath, Owner: "inbound-staff-shifts", Role: "adapter",
	InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
}

// shiftPlanningCutoverPermission is the one-time #3418 dissolution of the
// inbound-staff-shifts compatibility package into its owners (ADR 0037): the
// shift planning commands and queries into Workforce, the Tagesinformationen
// into Timetable, and the two cross-owner writes into the shift-plan-sync
// application workflow, whose points the candidate creates and which
// therefore need no exception. Like the #3351, #3427 and #3422 cutovers it
// forgives no debt, changes no ownership, adds no external dependency and
// opens no owner internals to a stranger; ordinary graph checks still require
// the retired package to be absent from the tree.
//
// It grants two replacements, each anchored on a permission the base policy
// already granted in the same scope:
//
//   - a consumer that imported the retired package may import the public
//     contract of the owner that now holds that behaviour, Workforce or
//     Timetable; its suites may instead construct the replacement through
//     that owner's compose role;
//   - the package roles that received the retired code inherit that code's
//     own reach, and nothing more.
func shiftPlanningCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !shiftPlanningRetired(base, candidate) {
		return false
	}
	retired := base.packageMap()[base.ModulePath+"/"+shiftPlanningLegacyPath]
	if shiftPlanningReplacementTarget(scope, source, target) && policyAllowsFirstParty(base, scope, source, retired) {
		return true
	}
	if shiftPlanningReceivingRole(scope, source) && policyAllowsFirstParty(base, scope, retired, target) {
		return true
	}
	if shiftPlanningMovedSuiteTarget(scope, source, target) && policyAllowsFirstParty(base, scope, retired.inScope(scope), retired) {
		return true
	}
	return shiftPlanningMovedContractSuite(base, scope, source, target)
}

// shiftPlanningMovedSuiteTarget lets the moved suites reach the package that
// now holds the code they test and the composition that builds it, in the
// test scopes only. The anchor is the retired package's own suites, which
// reached the retired package at the base.
func shiftPlanningMovedSuiteTarget(scope Scope, source, target Package) bool {
	if scope != ScopeInternalTest && scope != ScopeExternalTest {
		return false
	}
	if source.Owner != "workforce" || target.Owner != "workforce" {
		return false
	}
	switch source.inScope(scope).Role {
	case "module-internal-test", "module-behavior-test":
		return target.Role == "application" || target.Role == "compose"
	}
	return false
}

// shiftPlanningMovedContractSuite carries the planning contract suite from
// modules/workforce/contracttest into the planning package: its Workforce
// test role may reach, in a test scope, a target the contract suite's own
// e2e-test role could reach at the base in external_test scope. This carries
// the suite's fixtures, not new reach.
func shiftPlanningMovedContractSuite(base *Policy, scope Scope, source, target Package) bool {
	if scope != ScopeInternalTest && scope != ScopeExternalTest || source.Owner != "workforce" {
		return false
	}
	switch source.inScope(scope).Role {
	case "module-internal-test", "module-behavior-test":
	default:
		return false
	}
	contractSuite := Package{Owner: "workforce", Role: "e2e-test", InternalTestRole: "e2e-test", ExternalTestRole: "e2e-test"}
	return policyAllowsFirstParty(base, ScopeExternalTest, contractSuite, target)
}

// shiftPlanningReplacementTarget lists the points a former consumer of the
// retained package may name instead: the public contracts of the two owners
// that received the behaviour everywhere, and their compositions in the test
// scopes only, where a suite constructs the capability it drives. One
// production point is pinned: the plan export adapter named the retained
// room rows through the retired package's reader interface and now names
// their row package directly, which keeps them exactly as private as they
// were.
func shiftPlanningReplacementTarget(scope Scope, source, target Package) bool {
	if scope == ScopeProduction && source.Owner == "document-rendering" && source.Role == "adapter" &&
		target.Owner == "facilities" && target.Role == "domain" {
		return true
	}
	if target.Owner != "workforce" && target.Owner != "timetable-activities" {
		return false
	}
	switch target.Role {
	case "public":
		return true
	case "compose":
		return scope == ScopeInternalTest || scope == ScopeExternalTest
	default:
		return false
	}
}

// shiftPlanningReceivingRole lists the points that received the retired code.
// Workforce's application holds the planning commands and queries, its
// composition binds them and the retained-row adapters, and Timetable's
// composition holds the Tagesinformationen service and route; the test roles
// are those of the moved suites. They keep the dependencies that code already
// had: every grant still needs the retired package's own base permission in
// the same scope.
func shiftPlanningReceivingRole(scope Scope, source Package) bool {
	role := source.inScope(scope).Role
	switch source.Owner {
	case "workforce":
		switch role {
		case "application", "compose", "module-internal-test", "module-behavior-test", "workflow-integration-test":
			return true
		}
	case "timetable-activities":
		switch role {
		case "compose", "workflow-integration-test", "module-behavior-test":
			return true
		}
	}
	return false
}

func shiftPlanningRetired(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	want := shiftPlanningRetiredPackage
	legacy, exists := base.packageMap()[base.ModulePath+"/"+shiftPlanningLegacyPath]
	if !exists || legacy.Owner != want.Owner || legacy.Role != want.Role ||
		legacy.InternalTestRole != want.InternalTestRole || legacy.ExternalTestRole != want.ExternalTestRole {
		return false
	}
	for _, pkg := range candidate.Packages {
		if pkg.Path == shiftPlanningLegacyPath || strings.HasPrefix(pkg.Path, shiftPlanningLegacyPath+"/") {
			return false
		}
	}
	return true
}
