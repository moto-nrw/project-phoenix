package common

import (
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// CodeReadOnlyPreview is the stable wire code the frontend maps to its
// read-only-preview error message (#2893).
const CodeReadOnlyPreview = "read_only_preview"

// msgReadOnlyPreview is shown verbatim when a write slips past the disabled
// UI (direct API call, stale tab). German because it reaches school users.
const msgReadOnlyPreview = "In der Vorschau können Sie nur lesen. Beenden Sie die Vorschau, um etwas zu ändern."

// readOnlyPOSTAllowlist names the POST routes that only READ data — exports,
// dry-run previews, and bulk lookups whose ID list travels in the body. A
// read-only preview token may call these; every other non-GET request is
// blocked. Patterns use chi syntax: a "{...}" segment matches exactly one
// path segment. TestReadOnlyPOSTAllowlistMatchesRouteTable pins this list
// against api/testdata/route_table.golden so entries cannot rot and new
// write routes cannot silently match.
//
// Deliberately NOT listed although they only read: the plaintext reveal
// endpoints (/api/staff/{id}/stammdaten/bank-steuer/reveal,
// /api/guardians/{id}/payment/reveal). They expose financial plaintext and
// write a data-access audit row in the TARGET's permission context — in a
// preview the admin should use their own session for that, so the preview
// blocks them.
var readOnlyPOSTAllowlist = []string{
	"/api/birthdays/staff-export",
	"/api/emergency/snapshot/export",
	"/api/enrollment/admin/reports/care-usage/export",
	"/api/enrollment/admin/reports/class-roster/export",
	"/api/enrollment/admin/students/{studentId}/requests/export",
	"/api/enrollment/phases/{id}/export",
	"/api/guardians/payment-overview/export",
	"/api/import/class-list-entries/preview",
	"/api/import/opening-balances/preview",
	"/api/import/students/preview",
	"/api/import/teachers/preview",
	"/api/rooms/export",
	"/api/schedules/check-conflict",
	"/api/schedules/find-available-slots",
	"/api/staff-shifts/export",
	"/api/students/arrival-schedules/status",
	"/api/students/arrival-times/bulk",
	"/api/students/care-end/preview",
	"/api/students/care-withdrawals/{completionId}/care-end/preview",
	"/api/students/export",
	"/api/students/offering-change-requests/{requestId}/preview",
	"/api/students/pickup-times/bulk",
	"/api/students/{id}/pickup-schedules/preview",
	"/api/timetable/betreuungsplan/export",
	"/api/timetable/lists/export",
	"/api/timetable/lists/options",
	"/api/timetable/lists/preview",
}

// readOnlyGETDenylist names the GET routes that DO change state despite the
// method, so a read-only preview must not call them. Today that is the staff
// calendar feed: reading the URL creates and persists a subscription token on
// the target's account when none exists yet — a write in the previewed
// person's name. TestReadOnlyGETDenylistMatchesRouteTable pins every entry
// against api/testdata/route_table.golden so a stale one cannot linger.
var readOnlyGETDenylist = []string{
	"/api/calendar/feed",
}

// ReadOnlyPreviewMiddleware enforces the write block for admin staff-view
// preview tokens (#2893). Requests without a read-only claim pass untouched;
// with one, only safe methods (GET/HEAD/OPTIONS, minus the denylisted
// state-changing GETs) and the allowlisted
// read-only POST routes go through. Mounted group-wide right after
// jwt.Authenticator: it reads only the parsed claims, does no DB work, and
// rejecting here means no tenant transaction is ever opened for a blocked
// write.
func ReadOnlyPreviewMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := jwt.ClaimsFromCtx(r.Context())
		if !claims.IsReadOnlyPreview() || isReadOnlySafeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		RenderError(w, r, ErrorForbiddenMessageWithCode(msgReadOnlyPreview, CodeReadOnlyPreview))
	})
}

// isReadOnlySafeRequest reports whether the request cannot change state:
// a safe HTTP method that is not one of the denylisted state-changing GETs,
// or one of the allowlisted read-only POST routes.
func isReadOnlySafeRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return !matchesPatterns(r.URL.Path, readOnlyGETDenylist)
	case http.MethodPost:
		return matchesPatterns(r.URL.Path, readOnlyPOSTAllowlist)
	default:
		return false
	}
}

func matchesPatterns(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if pathMatchesPattern(path, pattern) {
			return true
		}
	}
	return false
}

// pathMatchesPattern compares a concrete request path against a chi-style
// pattern where a "{...}" segment matches exactly one non-empty segment.
// No wildcard tails, no prefix matches — a pattern only ever matches the
// one route it names.
func pathMatchesPattern(path, pattern string) bool {
	pathSegs := strings.Split(strings.TrimSuffix(path, "/"), "/")
	patternSegs := strings.Split(pattern, "/")
	if len(pathSegs) != len(patternSegs) {
		return false
	}
	for i, pat := range patternSegs {
		if strings.HasPrefix(pat, "{") && strings.HasSuffix(pat, "}") {
			if pathSegs[i] == "" {
				return false
			}
			continue
		}
		if pathSegs[i] != pat {
			return false
		}
	}
	return true
}
