package architecture

import "strings"

const studentPresenceLegacyPath = "modules/studentpresence/legacy"

// studentPresenceRetiredPackages is the nest #3422 dissolves, with the exact
// classification the immutable base must carry for the exception to apply.
var studentPresenceRetiredPackages = map[string]Package{
	studentPresenceLegacyPath + "/services/active": {
		Owner: "student-presence", Role: "adapter",
		InternalTestRole: "adapter-test", ExternalTestRole: "e2e-test",
	},
	studentPresenceLegacyPath + "/models/active": {
		Owner: "student-presence", Role: "domain",
		InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test",
	},
	studentPresenceLegacyPath + "/repositories/active": {
		Owner: "student-presence", Role: "postgres",
		InternalTestRole: "module-internal-test", ExternalTestRole: "e2e-test",
	},
	studentPresenceLegacyPath + "/statistics": {
		Owner: "student-presence", Role: "adapter",
		InternalTestRole: "adapter-test", ExternalTestRole: "module-behavior-test",
	},
}

// studentPresenceCutoverPermission is the one-time #3422 dissolution of the
// Student Presence legacy nest into its own capability and into the two owners
// whose rows it held (ADR 0036). Like the #3351 and #3427 cutovers it forgives
// no debt, changes no ownership, adds no external dependency and opens no
// owner internals to a stranger; ordinary graph checks still require every
// retired package to be absent from the tree.
//
// It grants three replacements, each anchored on a permission the base policy
// already granted in the same scope:
//
//   - a consumer that imported a retired package may import the public or
//     contract surface of the owner that now holds that behaviour — Student
//     Presence, and for the handed-back row types Workforce and Care Plan;
//   - the package roles that received the retired code inherit that code's own
//     reach, and the module's own suites reach the packages that now hold what
//     they test;
//   - two pinned same-owner bindings carry the row types across the hand-back.
func studentPresenceCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !studentPresenceRetired(base, candidate) {
		return false
	}
	if studentPresenceHandBackBinding(scope, source, target) {
		return true
	}
	retired := studentPresenceRetiredPoints(base)
	if studentPresenceReplacementTarget(scope, source, target) {
		if studentPresenceMovedSuite(base, candidate, scope, source, retired) {
			return true
		}
		for _, legacy := range retired {
			if policyAllowsFirstParty(base, scope, source, legacy) {
				return true
			}
		}
	}
	if studentPresenceReceivingRole(scope, source) || studentPresenceRowValueReach(scope, source, target) {
		for _, legacy := range retired {
			if policyAllowsFirstParty(base, scope, legacy, target) {
				return true
			}
		}
	}
	return false
}

// studentPresenceReplacementTarget lists the points a former consumer of the
// nest may name instead. Student Presence's public contract and Care Plan's
// and Workforce's row contracts replace the retired packages everywhere; the
// remaining points are narrower.
func studentPresenceReplacementTarget(scope Scope, source, target Package) bool {
	testScope := scope == ScopeInternalTest || scope == ScopeExternalTest
	switch target.Owner {
	case "student-presence":
		switch target.Role {
		case "public":
			return true
		case "compose":
			// Suites that constructed the retired service construct its native
			// replacement. In production the single pinned point is Facilities,
			// which implements the attendance-room port the retired adapter
			// declared; that port filters through a map and therefore cannot
			// move to the public contract.
			return testScope || source.Owner == "facilities" && source.Role == "compose"
		case "application", "port":
			// The nest's own suites moved to the packages that now hold the
			// code they test. This never reaches outside the owner.
			return testScope && source.Owner == "student-presence"
		}
	case "care-plan":
		switch target.Role {
		case "contract":
			return true
		case "test-support":
			// The shared fixture catalog inserts the handed-back rows; fixtures
			// reach fixtures, nothing else does.
			return source.Owner == "test-support" && source.inScope(scope).Role == "test-support"
		}
	case "workforce":
		// The nine staff and work-session row types were classified
		// student-presence/domain at the base. Naming them under their real
		// owner keeps them exactly as private as they were.
		return target.Role == "public" || target.Role == "adapter"
	}
	return false
}

// studentPresenceMovedSuite recognizes the nest's own suites in their new
// home. They moved into the module's native packages and the test role follows
// the suite, so the package they now sit in may carry a role its base
// classification did not have. The anchor is the nest itself: the point must
// be one the retired packages' own tests held, and those tests reached the
// nest at the base. It never leaves the owner or a test scope, and the target
// is still one of the replacement points.
func studentPresenceMovedSuite(base, candidate *Policy, scope Scope, source Package, retired []Package) bool {
	if scope != ScopeInternalTest && scope != ScopeExternalTest {
		return false
	}
	owner, role := source.Owner, source.inScope(scope).Role
	if source.Path != "" {
		// The comparison hands us the base classification of a package the
		// candidate reclassified. The suite now living there is the candidate's
		// point, so read the role the reviewed policy gives it.
		now, exists := candidate.packageMap()[candidate.ModulePath+"/"+source.Path]
		if !exists {
			return false
		}
		owner, role = now.Owner, now.inScope(scope).Role
	}
	if owner != "student-presence" {
		return false
	}
	for _, suite := range retired {
		if suite.inScope(scope).Role != role {
			continue
		}
		for _, legacy := range retired {
			if policyAllowsFirstParty(base, scope, suite, legacy) {
				return true
			}
		}
	}
	return false
}

// studentPresenceReceivingRole lists the Student Presence package roles that
// received the retired code. They keep the dependencies that code already had,
// and nothing more: every grant still needs the retired package's own base
// permission in the same scope.
func studentPresenceReceivingRole(scope Scope, source Package) bool {
	if source.Owner != "student-presence" {
		return false
	}
	switch source.inScope(scope).Role {
	case "public", "compose", "application", "port", "postgres", "test-support",
		"module-internal-test", "module-behavior-test", "adapter-test", "e2e-test":
		return true
	}
	return false
}

// studentPresenceRowValueReach keeps the canonical calendar-date value on the
// row types handed back to Care Plan and Workforce. Those packages received
// data, not behaviour, so they inherit exactly this one dependency of the
// retired packages and no other.
func studentPresenceRowValueReach(scope Scope, source, target Package) bool {
	if target.Owner != "legacy-shared" || target.Role != "domain" {
		return false
	}
	role := source.inScope(scope).Role
	switch source.Owner {
	case "care-plan":
		return role == "contract" || role == "test-support"
	case "workforce":
		return role == "adapter" || role == "adapter-test"
	}
	return false
}

// studentPresenceHandBackBinding pins the two production bindings that carry a
// handed-back row type into the contract that names it. Neither has a
// historical edge to inherit: at the base both sides were one package.
func studentPresenceHandBackBinding(scope Scope, source, target Package) bool {
	if scope != ScopeProduction || target.Owner != "care-plan" || target.Role != "contract" {
		return false
	}
	// The public status-day contract carries the Care Plan rows the retired
	// domain package held, and the relocated rows alias Care Plan's own
	// excused-request vocabulary instead of redeclaring the persisted values.
	return source.Owner == "student-presence" && source.Role == "public" ||
		source.Owner == "care-plan" && source.Role == "contract"
}

func studentPresenceRetiredPoints(base *Policy) []Package {
	packages := base.packageMap()
	points := make([]Package, 0, len(studentPresenceRetiredPackages))
	for path := range studentPresenceRetiredPackages {
		points = append(points, packages[base.ModulePath+"/"+path])
	}
	return points
}

func studentPresenceRetired(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	packages := base.packageMap()
	for path, want := range studentPresenceRetiredPackages {
		legacy, exists := packages[base.ModulePath+"/"+path]
		if !exists || legacy.Owner != want.Owner || legacy.Role != want.Role ||
			legacy.InternalTestRole != want.InternalTestRole || legacy.ExternalTestRole != want.ExternalTestRole {
			return false
		}
	}
	for _, pkg := range candidate.Packages {
		if pkg.Path == studentPresenceLegacyPath || strings.HasPrefix(pkg.Path, studentPresenceLegacyPath+"/") {
			return false
		}
	}
	return true
}
