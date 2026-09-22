package architecture

// timetablePlanningLegacyPath is the nest #3424 dissolves slice by slice.
const timetablePlanningLegacyPath = "modules/timetable/legacy/timetableplanning"

// timetablePlanningRetiredPackage is the exact classification the immutable
// base must carry for the exception to apply.
var timetablePlanningRetiredPackage = Package{
	Owner: "inbound-timetable", Role: "adapter",
	InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
}

// timetablePlanningCutoverPermission is the #3424 dissolution of the
// timetable planning nest into the School Calendar and the Timetable owner
// (ADR 0037). The nest is retired in slices, so unlike the #3351, #3427 and
// #3422 cutovers the retired package may still be classified in the
// candidate: each slice moves behaviour out of it and its consumers must
// already name the owner that now holds that behaviour. Like those cutovers
// it forgives no debt, changes no ownership, adds no external dependency and
// opens no owner internals to a stranger.
//
// It grants two replacements, each anchored on a permission the base policy
// already granted in the same scope:
//
//   - a consumer that imported the nest may import the public surface of
//     the owner that now holds the moved behaviour;
//   - the nest itself, and its own suites, may import that public surface
//     for the parts already moved out of it.
func timetablePlanningCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !timetablePlanningDissolving(base, candidate) {
		return false
	}
	if !timetablePlanningReplacementTarget(target) {
		return false
	}
	nest := timetablePlanningRetiredPoint(base)
	if timetablePlanningNestPoint(scope, source) {
		return true
	}
	return policyAllowsFirstParty(base, scope, source, nest)
}

// timetablePlanningReplacementTarget lists the points a former consumer of
// the nest may name instead: the School Calendar's public contract holds the
// calendar periods, closing days, holidays and dateframes slice S6 moved.
func timetablePlanningReplacementTarget(target Package) bool {
	return target.Owner == "school-calendar" && target.Role == "public"
}

// timetablePlanningNestPoint recognizes the retained nest and its own
// suites, which consume the replacement for the parts already moved.
func timetablePlanningNestPoint(scope Scope, source Package) bool {
	if source.Owner != timetablePlanningRetiredPackage.Owner {
		return false
	}
	switch source.inScope(scope).Role {
	case timetablePlanningRetiredPackage.Role,
		timetablePlanningRetiredPackage.InternalTestRole,
		timetablePlanningRetiredPackage.ExternalTestRole:
		return true
	}
	return false
}

func timetablePlanningRetiredPoint(base *Policy) Package {
	return base.packageMap()[base.ModulePath+"/"+timetablePlanningLegacyPath]
}

// timetablePlanningDissolving checks the anchor: a reviewed epoch and the
// nest classified exactly as the base recorded it. The candidate may still
// classify the nest while slices remain; the exception ends with the
// package.
func timetablePlanningDissolving(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	legacy, exists := base.packageMap()[base.ModulePath+"/"+timetablePlanningLegacyPath]
	if !exists {
		return false
	}
	want := timetablePlanningRetiredPackage
	return legacy.Owner == want.Owner && legacy.Role == want.Role &&
		legacy.InternalTestRole == want.InternalTestRole && legacy.ExternalTestRole == want.ExternalTestRole
}
