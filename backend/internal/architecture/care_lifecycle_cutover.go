package architecture

import "strings"

const careLifecycleLegacyPath = "modules/careplan/legacy/carelifecycle"

// careLifecycleCutoverPermission is the one-time #3427 replacement of the
// inbound-students care-lifecycle adapter by its Care Plan owner (ADR 0035).
// Like the #3351 care-schedule cutover it does not forgive debt, change
// ownership, add external dependencies, or open owner internals, and ordinary
// graph checks still require the retired package to be absent from the tree.
//
// It grants exactly two replacements, each only where the base policy already
// allowed the retired package in the same scope:
//
//   - a consumer that imported the retired adapter may import Care Plan's
//     public API and contracts instead; its tests may also construct the
//     native composition, as they constructed the retired service;
//   - the retired adapter's own behavior suites move into Care Plan's
//     contract suite and keep the fixture reach they had, never the retired
//     owner itself.
func careLifecycleCutoverPermission(base, candidate *Policy, scope Scope, source, target Package) bool {
	if !careLifecycleRetired(base, candidate) {
		return false
	}
	legacy := base.packageMap()[base.ModulePath+"/"+careLifecycleLegacyPath]
	if target.Owner == "care-plan" && careLifecycleNativeRole(scope, target.Role) {
		return policyAllowsFirstParty(base, scope, source, legacy)
	}
	if scope == ScopeExternalTest && source.Owner == "care-plan" && source.inScope(scope).Role == "e2e-test" {
		return target.Owner != legacy.Owner && policyAllowsFirstParty(base, scope, legacy, target)
	}
	return false
}

// careLifecycleNativeRole lists the Care Plan points a former consumer may
// name: the public API and its contracts everywhere, and the composition in
// test scopes only, where suites construct the capability they drive.
func careLifecycleNativeRole(scope Scope, role string) bool {
	switch role {
	case "public", "contract":
		return true
	case "compose":
		return scope == ScopeInternalTest || scope == ScopeExternalTest
	default:
		return false
	}
}

func careLifecycleRetired(base, candidate *Policy) bool {
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" || candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	legacy, exists := base.packageMap()[base.ModulePath+"/"+careLifecycleLegacyPath]
	if !exists || legacy.Owner != "inbound-students" || legacy.Role != "adapter" ||
		legacy.InternalTestRole != "module-internal-test" || legacy.ExternalTestRole != "module-behavior-test" {
		return false
	}
	for _, pkg := range candidate.Packages {
		if pkg.Path == careLifecycleLegacyPath || strings.HasPrefix(pkg.Path, careLifecycleLegacyPath+"/") {
			return false
		}
	}
	return true
}
