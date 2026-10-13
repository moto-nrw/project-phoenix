package architecture

// repositoryFactoryRetainedPath is the root legacy composition package #2743
// dissolves into the compositions that may own its parts.
const repositoryFactoryRetainedPath = "database/repositories"

// repositoryFactoryPoint is the exact classification the base records for the
// retained package.
var repositoryFactoryPoint = Package{
	Owner: "legacy-composition", Role: "compose",
	InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
}

// repositoryFactoryCutoverPermission is the #2743 move of Audit and Settings
// persistence construction out of the repository Factory into the owners'
// compositions (ADR 0049). Audit Platform persistence is constructed by
// modules/auditlog/compose, which no composition consuming it could reach;
// Settings Platform persistence is constructed by modules/settings/compose,
// which could not import its own Postgres adapter. Both points exist at the
// base, so the rules are not anchored on a point the candidate creates.
//
// It grants exactly two production permissions: legacy-composition/compose
// may import audit-platform/compose, and settings-platform/compose may import
// settings-platform/postgres. It forgives no debt, changes no ownership, adds
// no external dependency, and no other owner, role or scope gains anything.
func repositoryFactoryCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if scope != ScopeProduction || !repositoryFactoryMoving(base, candidate) {
		return false
	}
	sourceRole := source.inScope(scope).Role
	switch {
	case source.Owner == "legacy-composition" && sourceRole == "compose":
		return target.Owner == "audit-platform" && target.Role == "compose"
	case source.Owner == "settings-platform" && sourceRole == "compose":
		return target.Owner == "settings-platform" && target.Role == "postgres"
	}
	return false
}

// repositoryFactoryMoving checks the anchor: a reviewed epoch and the retained
// root package classified exactly as the base recorded it.
func repositoryFactoryMoving(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	retained, exists := base.packageMap()[base.ModulePath+"/"+repositoryFactoryRetainedPath]
	return exists && sameClassification(retained, repositoryFactoryPoint)
}
