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
// The lock in api.TenantScopeMiddleware refuses every module route except
// the surfaces in moduleRoutesLockedElsewhere.
var moduleRouteRules = []moduleRouteRule{
	// Infrastructure, sign-in and the operator portal.
	{"/", moduleRouteBasis},
	{"/health", moduleRouteBasis},
	{"/ready", moduleRouteBasis},
	{"/analytics-consent", moduleRouteBasis},
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
	// The team resends a parent's invitation in the tenant chain (#4895).
	{"/auth/guardian-invitations/*/resend", settings.ModuleElternApp},
	{"/parent", settings.ModuleElternApp},
	{"/parent-sse", settings.ModuleElternApp},
	{"/parent-news-attachments", settings.ModuleElternApp},
	{"/parent/enrollments", settings.ModuleOnlineAnmeldung},
	{"/school", settings.ModuleUnterrichtVormittag},
	{"/school/auth", settings.ModuleUnterrichtVormittag},
	{"/school-sse", settings.ModuleUnterrichtVormittag},
	{"/traeger", moduleRouteBasis},
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
	// Staff and parent surveys (#4781); see docs/reference/modulschalter.md.
	{"/api/surveys", moduleRouteBasis},
	{"/api/timetable", moduleRouteBasis},
	{"/api/users", moduleRouteBasis},

	// NFC-Anwesenheit: armbands and cards of a child.
	{"/api/students/*/rfid", settings.ModuleNFCAnwesenheit},

	// Mittagessen und Catering.
	{"/api/meal-plan", settings.ModuleMittagessen},
	{"/api/meal-caterer", settings.ModuleMittagessen},
	{"/caterer", settings.ModuleMittagessen},

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

	// Online-Anmeldung: phases, forms, care offerings and requests, and the
	// public intake and status links.
	{"/api/enrollment", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/*/submit", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/*/uploads", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/phases/public", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/form-bootstrap/public", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/schema/public", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/care-offerings/public", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/legal", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/captcha-config", settings.ModuleOnlineAnmeldung},
	{"/api/enrollment/requests", settings.ModuleOnlineAnmeldung},

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

// moduleRoutesLockedElsewhere names, per rule prefix, the module routes the
// tenant-scope lock leaves alone and who locks them instead: a later issue
// for the portals and devices, or the owner where the route resolves its
// school from the path or a token rather than from a session.
var moduleRoutesLockedElsewhere = map[string]string{
	// The caterer portal spans schools: its session belongs to no school and
	// every school route re-checks the supply (ADR 0077).
	"/caterer": "#4895 Caterer-Portal: schulübergreifende Sitzung, Belieferung je Schule",
	// The parents portal spans schools: ParentScopeMiddleware locks the
	// account once none of its guardian schools books Eltern-App, login and
	// refresh refuse its session (#4895). Its enrollment routes additionally
	// need Online-Anmeldung at the school: enrollment.enabled counts only
	// while the module is booked.
	"/parent":                  "#4895 Elternportal: Eltern-Sitzung und ParentScopeMiddleware",
	"/parent-sse":              "#4895 Elternportal: Eltern-Sitzung und ParentScopeMiddleware",
	"/parent-news-attachments": "#4895 Elternportal: Eltern-Sitzung und ParentScopeMiddleware",
	"/parent/enrollments":      "#4895 Elternportal: Eltern-Sitzung, Anmeldung über enrollment.enabled",
	// The public sign-in of the school portal carries no session yet;
	// Identity & Access refuses its login, second factor and switch for a
	// school without Unterricht am Vormittag (#4895). Every other /school
	// route passes SchoolScopeMiddleware.
	"/school/auth": "#4895 Schulportal: Anmeldung prüft das Modul in Identity & Access",
	// Accepting an invitation creates the account, no session; the login
	// afterwards refuses a school without Eltern-App (#4895).
	"/auth/guardian-invitations": "#4895 Elternportal: Einladung annehmen stellt keine Sitzung aus, der Login sperrt",
	// The device administration under /api/iot runs in the staff chain, but
	// devices and their PyrePortal contract move as one in #4896.
	"/api/iot":             "#4896 Geräte und PyrePortal",
	"/api/iot/staff-clock": "#4896 Geräte und PyrePortal",
	"/api/students/*/rfid": "#4896 Geräte: Armband am Tablet zuordnen",
	// Public enrollment: enrollment.enabled counts only while the module is
	// booked (EnrollmentIntakeValues.EnrollmentEnabled), status links close
	// in Intake.requestByStatusToken.
	"/api/enrollment/*/submit":                "Anmeldung: enrollment.enabled nur mit gebuchtem Modul",
	"/api/enrollment/*/uploads":               "Anmeldung: enrollment.enabled nur mit gebuchtem Modul",
	"/api/enrollment/phases/public":           "Anmeldung: enrollment.enabled nur mit gebuchtem Modul",
	"/api/enrollment/form-bootstrap/public":   "Anmeldung: enrollment.enabled nur mit gebuchtem Modul",
	"/api/enrollment/requests":                "Anmeldung: Statuslink schließt mit dem Modul",
	"/api/enrollment/schema/public":           "Anmeldung: Formularaufbau einer Phase, keine Personendaten",
	"/api/enrollment/care-offerings/public":   "Anmeldung: Angebotsliste einer Phase, keine Personendaten",
	"/api/enrollment/legal":                   "Anmeldung: Rechtstexte der Schule, keine Personendaten",
	"/api/enrollment/captcha-config":          "Anmeldung: Captcha-Schlüssel, keine Personendaten",
	"/public/enrollment-legal-documents":      "Anmeldung: Rechtstexte der Schule, keine Personendaten",
	"/public/enrollment-form-legal-documents": "Anmeldung: Rechtstexte der Schule, keine Personendaten",
}

// moduleRouteFor returns the most specific rule covering a request path or
// a route pattern. Both shapes resolve alike: a rule segment "*" matches a
// concrete value and a {param} placeholder.
func moduleRouteFor(path string) (moduleRouteRule, bool) {
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
	return best, found
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
