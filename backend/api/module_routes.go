package api

import (
	"strings"

	settings "github.com/moto-nrw/project-phoenix/modules/settings"
)

// moduleRouteBasis marks a route of moto Basis: every school has it, no
// module switch locks it.
const moduleRouteBasis settings.ModuleKey = ""

// moduleRouteRule assigns every route below prefix to one module or to
// Basis (#4893). A prefix segment "*" stands for exactly one path segment,
// a path parameter such as {id} in the route table.
type moduleRouteRule struct {
	prefix string
	module settings.ModuleKey
}

// moduleRouteRules is the module map of the production router. Every route
// matches at least one rule; the most specific one wins (more segments,
// then fewer wildcards). There is deliberately no catch-all: a route under
// a new path prefix fails TestFullProductionRouterGolden until it gets a
// rule here. The reasons for borderline cases are in
// docs/reference/modulschalter.md, section "Routen".
//
// Only routes behind the tenant scope (api.TenantScopeMiddleware) are locked
// by this PR. The portals (/parent, /school, guardian invitations) follow in
// #4895, the devices (/api/iot) in #4896; their rules already name the
// module they belong to.
var moduleRouteRules = []moduleRouteRule{
	// Infrastructure, sign-in and the operator portal.
	{"/", moduleRouteBasis},
	{"/health", moduleRouteBasis},
	{"/ready", moduleRouteBasis},
	{"/analytics-policy", moduleRouteBasis},
	{"/internal", moduleRouteBasis},
	{"/auth", moduleRouteBasis},
	{"/operator", moduleRouteBasis},
	{"/.well-known", moduleRouteBasis},
	{"/public/calendar", moduleRouteBasis},
	{"/public/login-image", moduleRouteBasis},
	{"/public/request-feed", moduleRouteBasis},
	{"/public/enrollment-legal-documents", settings.ModuleOnlineAnmeldung},
	{"/public/enrollment-form-legal-documents", settings.ModuleOnlineAnmeldung},

	// Portals and devices (#4895, #4896).
	{"/auth/guardian-invitations", settings.ModuleElternApp},
	{"/parent", settings.ModuleElternApp},
	{"/parent-sse", settings.ModuleElternApp},
	{"/parent-news-attachments", settings.ModuleElternApp},
	{"/parent/enrollments", settings.ModuleOnlineAnmeldung},
	{"/school", settings.ModuleUnterrichtVormittag},
	{"/school-sse", settings.ModuleUnterrichtVormittag},
	{"/api/iot", settings.ModuleNFCAnwesenheit},
	{"/api/iot/staff-clock", settings.ModuleZeiterfassung},

	// moto Basis under /api.
	{"/api/active", moduleRouteBasis},
	{"/api/activities", moduleRouteBasis},
	{"/api/admin", moduleRouteBasis},
	{"/api/birthdays", moduleRouteBasis},
	{"/api/caldav", moduleRouteBasis},
	{"/api/calendar", moduleRouteBasis},
	{"/api/class-list-entries", moduleRouteBasis},
	{"/api/database", moduleRouteBasis},
	{"/api/display", moduleRouteBasis},
	{"/api/emergency", moduleRouteBasis},
	{"/api/feedback", moduleRouteBasis},
	{"/api/files", moduleRouteBasis},
	{"/api/groups", moduleRouteBasis},
	{"/api/guardians", moduleRouteBasis},
	{"/api/import", moduleRouteBasis},
	{"/api/me", moduleRouteBasis},
	{"/api/notifications", moduleRouteBasis},
	{"/api/platform", moduleRouteBasis},
	{"/api/pwa", moduleRouteBasis},
	{"/api/reminders", moduleRouteBasis},
	{"/api/rooms", moduleRouteBasis},
	{"/api/schedules", moduleRouteBasis},
	{"/api/school-setup", moduleRouteBasis},
	{"/api/settings", moduleRouteBasis},
	{"/api/sse", moduleRouteBasis},
	{"/api/staff", moduleRouteBasis},
	{"/api/staff-messages", moduleRouteBasis},
	{"/api/staff-notices", moduleRouteBasis},
	{"/api/staff-onboarding", moduleRouteBasis},
	{"/api/statistics", moduleRouteBasis},
	{"/api/students", moduleRouteBasis},
	{"/api/substitutions", moduleRouteBasis},
	{"/api/timetable", moduleRouteBasis},
	{"/api/users", moduleRouteBasis},

	// NFC-Anwesenheit: armbands and cards of a child.
	{"/api/students/*/rfid", settings.ModuleNFCAnwesenheit},

	// Mittagessen und Catering.
	{"/api/meal-plan", settings.ModuleMittagessen},

	// Eltern-App: messages and letters to parents, parent-portal invitations.
	{"/api/messages", settings.ModuleElternApp},
	{"/api/parent-announcements", settings.ModuleElternApp},
	{"/api/announcement-attachments", settings.ModuleElternApp},
	{"/api/guardians/invitations", settings.ModuleElternApp},
	{"/api/guardians/invitable", settings.ModuleElternApp},
	{"/api/guardians/bulk-invite", settings.ModuleElternApp},
	{"/api/guardians/without-account", settings.ModuleElternApp},
	{"/api/guardians/*/invite", settings.ModuleElternApp},
	{"/api/guardians/students/*/invite", settings.ModuleElternApp},

	// Online-Anmeldung: phases, forms, care offerings and requests.
	{"/api/enrollment", settings.ModuleOnlineAnmeldung},

	// Dienstplan.
	{"/api/staff-shifts", settings.ModuleDienstplan},
	{"/api/shift-types", settings.ModuleDienstplan},
	{"/api/timetable/shift-coverage", settings.ModuleDienstplan},

	// Zeiterfassung: clock, work-time models, staff absences and vacation,
	// balances and the payroll export.
	{"/api/time-tracking", settings.ModuleZeiterfassung},
	// Basis inside /api/time-tracking: the own care-plan blocks on the home
	// page, and the school's holidays and closing days the planning shows.
	{"/api/time-tracking/assignments", moduleRouteBasis},
	{"/api/time-tracking/holidays", moduleRouteBasis},
	{"/api/time-tracking/closing-days", moduleRouteBasis},
	// The own shifts behind "Mein Dienstplan".
	{"/api/time-tracking/shifts", settings.ModuleDienstplan},
	{"/api/work-time-models", settings.ModuleZeiterfassung},
	{"/api/absence-types", settings.ModuleZeiterfassung},
	{"/api/import/opening-balances", settings.ModuleZeiterfassung},
	{"/api/settings/payroll-status", settings.ModuleZeiterfassung},
	{"/api/staff/absences", settings.ModuleZeiterfassung},
	{"/api/staff/time-tracking", settings.ModuleZeiterfassung},
	{"/api/staff/*/absences", settings.ModuleZeiterfassung},
	{"/api/staff/*/time-tracking", settings.ModuleZeiterfassung},
	{"/api/staff/*/vacation", settings.ModuleZeiterfassung},
	{"/api/staff/*/target-overrides", settings.ModuleZeiterfassung},
	{"/api/staff/*/schedule", settings.ModuleZeiterfassung},
	{"/api/staff/*/payroll-number", settings.ModuleZeiterfassung},

	// Personalakte: HR file, documents and bank and tax data.
	{"/api/staff/*/stammdaten", settings.ModulePersonalakte},
	{"/api/staff/*/documents", settings.ModulePersonalakte},
	{"/api/staff/documents-directory", settings.ModulePersonalakte},
	{"/api/staff/documents-profile", settings.ModulePersonalakte},
	{"/api/staff/financial-profile", settings.ModulePersonalakte},
}

// moduleForRoute returns the module of a request path or a route pattern
// and whether any rule covers it. Both shapes resolve alike: a rule segment
// "*" matches a concrete value and a {param} placeholder.
func moduleForRoute(path string) (settings.ModuleKey, bool) {
	segments := routeSegments(path)
	best, found := moduleRouteRule{}, false
	bestLength, bestWildcards := -1, 0
	for _, rule := range moduleRouteRules {
		ruleSegments := routeSegments(rule.prefix)
		wildcards, ok := matchRoutePrefix(ruleSegments, segments)
		if !ok {
			continue
		}
		if len(ruleSegments) > bestLength || (len(ruleSegments) == bestLength && wildcards < bestWildcards) {
			best, found = rule, true
			bestLength, bestWildcards = len(ruleSegments), wildcards
		}
	}
	return best.module, found
}

// matchRoutePrefix reports whether prefix covers the leading path segments
// and how many of its segments were wildcards.
func matchRoutePrefix(prefix, path []string) (int, bool) {
	// The root rule "/" covers only the root route, never every path.
	if len(prefix) > len(path) || (len(prefix) == 0 && len(path) > 0) {
		return 0, false
	}
	wildcards := 0
	for i, segment := range prefix {
		if segment == "*" {
			wildcards++
			continue
		}
		if segment != path[i] {
			return 0, false
		}
	}
	return wildcards, true
}

func routeSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
