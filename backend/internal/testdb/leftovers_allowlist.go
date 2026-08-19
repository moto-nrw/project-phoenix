package testdb

import (
	"slices"
	"sort"
)

// LeftoverAllowlist names the (package, table) pairs that are still allowed to
// differ between a clone's start and end state — shared, tenant-less rows a
// test writes and does not take back (#2419 goal 2).
//
// It is shrink-only. A pair that is not listed fails the sweep, which is what
// makes the leftover check a gate rather than a report. Removing a pair means
// the package's tests stopped writing into shared state (they moved the row
// under their own tenant, or they delete it again); adding one means the
// opposite and needs a written reason here.
//
// Seeded from the measured state at the time the gate was introduced. Three
// families make up the whole list, and the reason is what a future migration
// removes, not the entry:
//
//	auth.accounts and its                accounts carry no tenant_id at all —
//	account_roles/account_tenants        the mapping to a school is a separate
//	                                     row. A test account is therefore
//	                                     shared state by construction, and
//	                                     only an explicit delete takes it back.
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
	"api/active":                       {"active.group_supervisors", "active.groups", "active.work_sessions", "activities.categories", "activities.groups", "auth.accounts", "facilities.rooms", "platform.organizations", "platform.schools", "users.persons", "users.staff", "users.students"},
	"api/activities":                   {"auth.accounts"},
	"api/admin":                        {"auth.accounts"},
	"api/auth":                         {"auth.accounts", "auth.password_reset_tokens", "auth.permissions", "auth.roles"},
	"api/birthdays":                    {"auth.accounts"},
	"api/classday":                     {"auth.accounts"},
	"api/classlistentries":             {"auth.accounts"},
	"api/database":                     {"auth.accounts"},
	"api/display":                      {"active.groups", "active.visits", "activities.categories", "activities.groups", "auth.accounts", "config.setting_audit", "config.setting_values", "facilities.rooms", "iot.devices", "platform.organizations", "platform.schools", "users.persons", "users.staff", "users.students"},
	"api/guardians":                    {"auth.accounts"},
	"api/import":                       {"auth.accounts"},
	"api/iot/data":                     {"auth.accounts"},
	"api/parent":                       {"auth.accounts"},
	"api/rooms":                        {"auth.accounts", "facilities.rooms", "platform.organizations", "platform.schools"},
	"api/school":                       {"auth.accounts"},
	"api/sse":                          {"auth.accounts"},
	"api/staff":                        {"auth.accounts"},
	"api/students":                     {"auth.accounts", "education.groups", "platform.organizations", "platform.schools"},
	"api/suggestions":                  {"auth.accounts"},
	"api/timetable":                    {"platform.organizations", "platform.schools"},
	"api/usercontext":                  {"auth.accounts"},
	"api/users":                        {"auth.accounts"},
	"auth/authorize":                   {"auth.accounts"},
	"database/migrations":              {"activities.categories", "auth.accounts", "config.setting_audit", "config.setting_values", "platform.organizations", "platform.schools"},
	"database/repositories/active":     {"activities.categories", "auth.accounts", "platform.organizations", "platform.schools", "users.persons", "users.staff"},
	"database/repositories/audit":      {"platform.organizations", "platform.schools"},
	"database/repositories/auth":       {"auth.permissions", "platform.organizations", "platform.schools"},
	"database/repositories/base":       {"auth.accounts"},
	"database/repositories/config":     {"platform.organizations", "platform.schools"},
	"database/repositories/education":  {"platform.organizations", "platform.schools"},
	"database/repositories/enrollment": {"platform.organizations", "platform.schools"},
	"database/repositories/facilities": {"platform.organizations", "platform.schools"},
	"database/repositories/parent":     {"platform.organizations", "platform.schools"},
	"database/repositories/platform":   {"platform.operators"},
	"database/repositories/schedule":   {"platform.organizations", "platform.schools"},
	"database/repositories/users":      {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/absence":                 {"auth.accounts"},
	"services/active":                  {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/auth":                    {"auth.accounts", "auth.permissions", "auth.role_permissions", "auth.roles", "platform.organizations", "platform.schools"},
	"services/enrollment":              {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/notifications":           {"auth.account_roles", "auth.account_tenants", "auth.accounts", "platform.organizations", "platform.schools"},
	"services/parent":                  {"auth.accounts", "platform.organizations", "platform.schools"},
	"services/parentmessaging":         {"auth.accounts"},
	"services/platform":                {"auth.account_roles", "auth.account_tenants", "auth.accounts", "auth.roles", "platform.operator_audit_log", "platform.operator_mfa_credentials", "platform.operator_mfa_email_challenges", "platform.operator_mfa_trusted_devices", "platform.operators", "platform.organizations", "platform.schools"},
	"services/schedule":                {"platform.organizations", "platform.schools"},
	"services/scheduler":               {"auth.accounts"},
	"services/suggestions":             {"auth.accounts"},
	"services/usercontext":             {"auth.accounts"},
	"services/users":                   {"auth.accounts", "platform.organizations", "platform.schools"},
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

// UnallowedLeftovers filters a sweep result down to the leftovers the
// allowlist does not cover — the ones that fail the run.
func UnallowedLeftovers(leftovers []CloneLeftovers) []CloneLeftovers {
	var out []CloneLeftovers
	for _, l := range leftovers {
		var tables []TableDelta
		for _, d := range l.Tables {
			if !leftoverAllowed(l.Package, d.Table) {
				tables = append(tables, d)
			}
		}
		if len(tables) > 0 {
			out = append(out, CloneLeftovers{Clone: l.Clone, Package: l.Package, Tables: tables})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}
