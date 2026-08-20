package testdb

import "slices"

// LeftoverAllowlist names the (package, table) pairs that are still allowed to
// differ between a clone's start and end state — shared rows a test writes and
// does not take back (#2419 goal 2).
//
// It is shrink-only. A pair that is not listed fails the package whose test
// binary produced it, which is what makes the leftover check a gate rather
// than a report. Removing a pair means the package's tests stopped writing
// into shared state (they moved the row under their own tenant, or they delete
// it again); adding one means the opposite and needs a written reason here.
//
// 27 packages / 100 pairs became 9 / 38 when the tests that pinned a tenant to
// a literal ID below TenantIDBase moved to testpkg.UniqueTestTenantID, and the
// fixtures for tenant-less rows (operators, announcements, system roles,
// permissions) started taking their rows back themselves (#2419 goal 4).
//
// What is left, and why:
//
//	auth.accounts / password_reset      accounts created through a SERVICE
//	                                    path (register, invitation, guardian
//	                                    flows) instead of the fixture, so
//	                                    nothing claims them for a tenant.
//
//	auth.roles / permissions /          the RBAC catalog is partly global
//	role_permissions                    (system roles have a NULL tenant and a
//	                                    clone-wide unique name), so a test
//	                                    that grants one writes into it.
//
//	platform.operator* + the operator   the operator portal sits above the
//	portal's schools/organizations      tenant boundary by construction.
//
//	api/active, database/migrations     tests that provision a whole school
//	                                    through the service path: the school
//	                                    gets a sequence ID, and everything
//	                                    hanging off it inherits that tenant.
var LeftoverAllowlist = map[string][]string{
	"api/active":                       {"active.group_supervisors", "active.groups", "active.work_sessions", "activities.categories", "activities.groups", "facilities.rooms", "platform.organizations", "platform.schools", "users.persons", "users.staff"},
	"api/auth":                         {"auth.accounts", "auth.password_reset_tokens"},
	"database/migrations":              {"activities.categories", "config.setting_audit", "config.setting_values", "platform.organizations", "platform.schools"},
	"database/repositories/enrollment": {"platform.organizations", "platform.schools"},
	"database/repositories/platform":   {"platform.operators"},
	"database/repositories/users":      {"auth.accounts"},
	"services/auth":                    {"auth.permissions", "auth.role_permissions", "platform.organizations", "platform.schools"},
	"services/platform":                {"auth.account_roles", "auth.account_tenants", "auth.accounts", "auth.roles", "platform.operator_audit_log", "platform.operator_mfa_credentials", "platform.operator_mfa_email_challenges", "platform.operator_mfa_trusted_devices", "platform.operators", "platform.organizations", "platform.schools"},
	"services/schedule":                {"platform.organizations", "platform.schools"},
}

// leftoverAllowed reports whether a leftover in table is tolerated for the
// package that produced the clone. An unlabeled clone (no package stamp) is
// never tolerated: the gate must not be switched off by a missing label.
func leftoverAllowed(pkg, table string) bool {
	if pkg == "" {
		return false
	}
	return slices.Contains(LeftoverAllowlist[pkg], table)
}

// UnallowedTables filters a package's leftovers down to the ones the
// allowlist does not cover — the ones that fail the package.
func UnallowedTables(pkg string, deltas []TableDelta) []TableDelta {
	var out []TableDelta
	for _, d := range deltas {
		if !leftoverAllowed(pkg, d.Table) {
			out = append(out, d)
		}
	}
	return out
}
