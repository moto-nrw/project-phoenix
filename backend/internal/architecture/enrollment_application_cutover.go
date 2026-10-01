package architecture

// enrollmentRetainedApplicationPath is the package the #2733 set dissolves
// group by group into the Enrollment and Care Plan owners.
const enrollmentRetainedApplicationPath = "services/enrollment"

// enrollmentModuleApplicationPath is the package that receives Enrollment's
// application behaviour from the retained package.
const enrollmentModuleApplicationPath = "modules/enrollment/internal/application"

// enrollmentApplicationPoint is the exact classification both packages carry:
// the retained package at the base, the module package in the candidate.
var enrollmentApplicationPoint = Package{
	Owner: "enrollment", Role: "application",
	InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
}

// enrollmentApplicationCutoverPermission is the #3562 move of Enrollment's
// application behaviour from services/enrollment into
// modules/enrollment/internal/application (ADR 0041). Every other module's
// composition constructs its own application services through a
// <owner>.compose.application rule; Enrollment's could not, because the
// retained package already holds the enrollment/application point at the
// base, so the rule is not anchored on a point the candidate creates.
//
// It grants exactly that one permission: the Enrollment composition may
// import the Enrollment application role, in production. Like the earlier
// cutovers it forgives no debt, changes no ownership, adds no external
// dependency and opens no owner internals to a stranger: the source and the
// target belong to the same owner, and no other role, owner or scope gains
// anything.
func enrollmentApplicationCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if scope != ScopeProduction || !enrollmentApplicationMoving(base, candidate) {
		return false
	}
	if source.Owner != "enrollment" || source.inScope(scope).Role != "compose" {
		return false
	}
	return target.Owner == "enrollment" && target.Role == "application"
}

// enrollmentApplicationMoving checks the anchor: a reviewed epoch, the
// retained package classified exactly as the base recorded it, and the
// module package classified at the same point in the candidate.
func enrollmentApplicationMoving(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	retained, exists := base.packageMap()[base.ModulePath+"/"+enrollmentRetainedApplicationPath]
	if !exists || !sameClassification(retained, enrollmentApplicationPoint) {
		return false
	}
	moved, exists := candidate.packageMap()[candidate.ModulePath+"/"+enrollmentModuleApplicationPath]
	return exists && sameClassification(moved, enrollmentApplicationPoint)
}

func sameClassification(pkg, want Package) bool {
	return pkg.Owner == want.Owner && pkg.Role == want.Role &&
		pkg.InternalTestRole == want.InternalTestRole && pkg.ExternalTestRole == want.ExternalTestRole
}
