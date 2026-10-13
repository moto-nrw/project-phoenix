package architecture

// serviceFactoryRetainedPath is the legacy composition package #2747
// contracts: its service Factory dissolves into the owner compositions.
const serviceFactoryRetainedPath = "services"

// serviceFactoryPoint is the exact classification the base records for the
// retained package.
var serviceFactoryPoint = Package{
	Owner: "legacy-composition", Role: "compose",
	InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
}

// serviceFactoryCutoverGrants are the production permissions the #2747
// contraction needs between points that exist at the base (ADR 0050). Each
// one lets the legacy composition name an owner's retained service through
// that owner's own composition or contract instead of its application
// package, or lets an owner composition construct its own retained service.
var serviceFactoryCutoverGrants = map[[2]ownerRolePair]bool{
	{{"legacy-composition", "compose"}, {"document-rendering", "contract"}}: true,
}

type ownerRolePair struct {
	Owner string
	Role  string
}

// serviceFactoryCutoverPermission grants exactly serviceFactoryCutoverGrants
// in production while the reviewed epoch moves the retained package. It
// forgives no debt, changes no ownership, adds no external dependency, and no
// other owner, role or scope gains anything.
func serviceFactoryCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if scope != ScopeProduction || !serviceFactoryMoving(base, candidate) {
		return false
	}
	grant := [2]ownerRolePair{
		{source.Owner, source.inScope(scope).Role},
		{target.Owner, target.inScope(scope).Role},
	}
	return serviceFactoryCutoverGrants[grant]
}

// serviceFactoryMoving checks the anchor: a reviewed epoch and the retained
// package classified exactly as the base recorded it.
func serviceFactoryMoving(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	retained, exists := base.packageMap()[base.ModulePath+"/"+serviceFactoryRetainedPath]
	return exists && sameClassification(retained, serviceFactoryPoint)
}
