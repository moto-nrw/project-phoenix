package testdb

import "slices"

// LeftoverAllowlist names the (package, table) pairs that are still allowed to
// differ between a clone's start and end state — shared, tenant-less rows a
// test writes and does not take back (#2419 goal 2).
//
// It is shrink-only. A pair that is not listed fails the package whose test
// binary produced it, which is what makes the leftover check a gate rather
// than a report. Removing a pair means
// the package's tests stopped writing into shared state (they moved the row
// under their own tenant, or they delete it again); adding one means the
// opposite and needs a written reason here.
//
// What is left after CreateTestAccount started claiming its account for the
// tenant of the test that created it (#2419): 49 packages became 27, 132
// pairs became 100, and auth.accounts went from 38 packages to 6.
//
//	auth.accounts                        the accounts NOT created by the
//	                                     fixture: invitation, enrollment,
//	                                     guardian and operator flows create
//	                                     them through the service path, which
//	                                     has no test tenant to claim them for.
//
//	auth.account_tenants /               rows whose tenant is a literal ID
//	auth.account_roles                   below TenantIDBase (older tests pin
//	                                     e.g. 1021001) instead of the tenant
//	                                     the test owns.
//
//	auth.roles / permissions /           the RBAC catalog is partly global
//	role_permissions                     (system roles have a NULL tenant and
//	                                     a clone-wide unique name), so a test
//	                                     that creates one writes into it.
//
//	platform.operator* and the           the operator portal sits above the
//	remaining platform.schools /         tenant boundary; the schools and
//	platform.organizations rows          organizations left here are the ones
//	                                     tests still create with a literal ID
//	                                     below TenantIDBase instead of
//	                                     testpkg.UniqueTestTenantID. Those are
//	                                     the cheapest entries to remove.
var LeftoverAllowlist = map[string][]string{
	"api/active":                       {"active.group_supervisors", "active.groups", "active.work_sessions", "activities.categories", "activities.groups", "facilities.rooms", "platform.organizations", "platform.schools", "users.persons", "users.staff", "users.students"},
	"api/auth":                         {"auth.accounts", "auth.password_reset_tokens", "auth.permissions", "auth.roles"},
	"api/display":                      {"active.groups", "active.visits", "activities.categories", "activities.groups", "config.setting_audit", "config.setting_values", "facilities.rooms", "iot.devices", "platform.organizations", "platform.schools", "users.persons", "users.staff", "users.students"},
	"api/staff":                        {"auth.accounts"},
	"api/rooms":                        {"facilities.rooms", "platform.organizations", "platform.schools"},
	"api/students":                     {"education.groups", "platform.organizations", "platform.schools"},
	"api/timetable":                    {"platform.organizations", "platform.schools"},
	"database/migrations":              {"activities.categories", "config.setting_audit", "config.setting_values", "platform.organizations", "platform.schools"},
	"database/repositories/active":     {"activities.categories", "platform.organizations", "platform.schools", "users.persons", "users.staff"},
	"database/repositories/audit":      {"platform.organizations", "platform.schools"},
	"database/repositories/auth":       {"auth.permissions", "platform.organizations", "platform.schools"},
	"database/repositories/config":     {"platform.organizations", "platform.schools"},
	"database/repositories/education":  {"platform.organizations", "platform.schools"},
	"database/repositories/enrollment": {"platform.organizations", "platform.schools"},
	"database/repositories/facilities": {"platform.organizations", "platform.schools"},
	"database/repositories/parent":     {"platform.organizations", "platform.schools"},
	"database/repositories/platform":   {"platform.operators"},
	"database/repositories/schedule":   {"platform.organizations", "platform.schools"},
	"database/repositories/users":      {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/active":                  {"platform.organizations", "platform.schools"},
	"services/auth":                    {"auth.permissions", "auth.role_permissions", "auth.roles", "platform.organizations", "platform.schools"},
	"services/enrollment":              {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/notifications":           {"auth.account_roles", "auth.account_tenants", "platform.organizations", "platform.schools"},
	"services/parent":                  {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/platform":                {"auth.account_roles", "auth.account_tenants", "auth.accounts", "auth.roles", "platform.operator_audit_log", "platform.operator_mfa_credentials", "platform.operator_mfa_email_challenges", "platform.operator_mfa_trusted_devices", "platform.operators", "platform.organizations", "platform.schools"},
	"services/schedule":                {"platform.organizations", "platform.schools"},
	"services/users":                   {"platform.organizations", "platform.schools"},
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
