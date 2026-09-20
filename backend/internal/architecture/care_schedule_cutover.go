package architecture

import "strings"

const careScheduleLegacyPath = "modules/careplan/legacy/careschedule"

// careScheduleCutoverPermission is the one-time #3351 replacement of the
// inbound-schedules adapter by its Care Plan owner. It does not forgive debt,
// ownership changes, external dependencies, or imports of owner internals.
// Ordinary graph checks still require the retired package to be absent from
// the source tree, not merely omitted from the candidate classifications.
func careScheduleCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !careScheduleRetired(base, candidate) {
		return false
	}
	legacy := base.packageMap()[base.ModulePath+"/"+careScheduleLegacyPath]
	if careScheduleNativePublicReplacement(scope, source, target) {
		return policyAllowsFirstParty(base, scope, source, legacy)
	}
	if careScheduleNativeFixtureReplacement(scope, source, target) {
		return policyAllowsFirstParty(base, scope, source, legacy)
	}
	// Tests that constructed the retired service may construct its native
	// replacement in the same test scope. This never grants runtime callers
	// composition access or lets tests reach application implementations.
	if (scope == ScopeInternalTest || scope == ScopeExternalTest) &&
		target.Owner == "care-plan" && target.Role == "compose" {
		return source.Owner != "inbound-schedules" && policyAllowsFirstParty(base, scope, source, legacy)
	}
	if target.Owner == "care-plan" && (target.Role == "public" || target.Role == "contract") {
		// The retired adapter's implementation becomes native: its ports use
		// owner values, and composition/application bind the request contracts.
		// This is not permission to expose an implementation or to grant new
		// cross-owner or test-support dependencies.
		if scope == ScopeProduction && source.Owner == "care-plan" &&
			(source.Role == "port" || target.Role == "contract" && (source.Role == "compose" || source.Role == "application")) {
			return true
		}
		// Replace only a permission the same source point already had in this
		// scope. A production consumer cannot lend permission to its tests.
		return source.Owner != "inbound-schedules" && policyAllowsFirstParty(base, scope, source, legacy)
	}
	if source.Owner != "care-plan" || target.Owner != "shared-kernel" || target.Role != "contract" {
		return false
	}
	// Native value contracts use the canonical Date/WallClock package rather
	// than continuing to import the retained internal/timezone forwarding
	// wrapper. Only the retired service's existing per-scope permission can
	// authorize this replacement; it cannot open other shared dependencies.
	role := source.inScope(scope).Role
	switch scope {
	case ScopeProduction:
		if role != "public" && role != "contract" && role != "compose" && role != "application" && role != "domain" && role != "port" {
			return false
		}
	case ScopeInternalTest, ScopeExternalTest:
		if role != "module-internal-test" && role != "module-behavior-test" {
			return false
		}
	default:
		return false
	}
	basePackages, candidatePackages := base.packageMap(), candidate.packageMap()
	calendarPath := base.ModulePath + "/sharedkernel/calendar"
	calendar, exists := basePackages[calendarPath]
	if !exists || calendar.Owner != "shared-kernel" || calendar.Role != "contract" || calendar != candidatePackages[calendarPath] {
		return false
	}
	// The rule vocabulary is owner/role based. Refuse it if another existing
	// shared-kernel package would gain permission alongside calendar.
	for path, pkg := range candidatePackages {
		if pkg.Owner == "shared-kernel" && pkg.Role == "contract" && path != calendarPath {
			return false
		}
	}
	wrapper, exists := basePackages[base.ModulePath+"/internal/timezone"]
	return exists && wrapper.Owner == "legacy-shared" && wrapper.Role == "domain" && policyAllowsFirstParty(base, scope, legacy, wrapper)
}

// The retired adapter also exposed Timetable's scheduling contracts and
// Presence's room-capacity sentinel. These consumers now name the real owner
// rather than retaining cross-owner aliases on Care Plan's public surface.
func careScheduleNativePublicReplacement(scope Scope, source, target Package) bool {
	if target.Role != "public" {
		return false
	}
	role := source.inScope(scope).Role
	if scope == ScopeProduction {
		return source.Owner == "timetable-activities" && role == "compose" && target.Owner == "student-presence"
	}
	if target.Owner != "timetable-activities" {
		return false
	}
	switch scope {
	case ScopeInternalTest:
		return source.Owner == "inbound-timetable" && role == "module-internal-test"
	case ScopeExternalTest:
		return role == "module-behavior-test" && (source.Owner == "inbound-timetable" || source.Owner == "enrollment")
	default:
		return false
	}
}

// The two retained arrival-integration fixtures must construct Timetable's
// class projections and bind them through Care Plan composition themselves.
// This is test-only construction. It never reaches the root composition
// packages, whose role also covers the repository graph.
func careScheduleNativeFixtureReplacement(scope Scope, source, target Package) bool {
	if target.Role != "compose" || target.Owner != "timetable-activities" {
		return false
	}
	role := source.inScope(scope).Role
	return scope == ScopeInternalTest && source.Owner == "inbound-timetable" && role == "adapter-test" ||
		scope == ScopeExternalTest && source.Owner == "enrollment" && role == "module-behavior-test"
}

func careScheduleRetired(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	legacy, exists := base.packageMap()[base.ModulePath+"/"+careScheduleLegacyPath]
	if !exists || legacy.Owner != "inbound-schedules" || legacy.Role != "adapter" ||
		legacy.InternalTestRole != "module-internal-test" || legacy.ExternalTestRole != "module-behavior-test" {
		return false
	}
	for _, pkg := range candidate.Packages {
		if pkg.Owner == "inbound-schedules" || pkg.Path == careScheduleLegacyPath || strings.HasPrefix(pkg.Path, careScheduleLegacyPath+"/") {
			return false
		}
	}
	return true
}
