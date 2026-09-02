package api

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staffRouteSurface is the complete route table the composed /api/staff
// router serves, as "METHOD /pattern". The School Membership adapter and the
// workforce admin resource register on one router (#2667); the frontend proxy
// routes under frontend/src/app/api/staff/ address exactly these patterns, so
// a route that moves between the two halves, disappears, or gets shadowed by
// a differently ordered registration must fail here before it fails a screen.
var staffRouteSurface = []string{
	"DELETE /{id}",
	"DELETE /{id}/absences/{absenceId}",
	"DELETE /{id}/documents/{documentId}",
	"DELETE /{id}/time-tracking/adjustments/{adjustmentId}",
	"DELETE /{id}/vacation/opening",
	"GET /",
	"GET /absences/pending",
	"GET /absences/requests",
	"GET /available",
	"GET /by-role",
	"GET /dashboard-summary",
	"GET /documents-directory",
	"GET /documents-profile/{id}",
	"GET /financial-profile/{id}",
	"GET /pin",
	"GET /time-tracking/audit-log",
	"GET /time-tracking/export",
	"GET /time-tracking/export/datev-report",
	"GET /time-tracking/month-close",
	"GET /time-tracking/overview",
	"GET /{id}",
	"GET /{id}/absences",
	"GET /{id}/avatar",
	"GET /{id}/documents",
	"GET /{id}/documents/{documentId}/download",
	"GET /{id}/groups",
	"GET /{id}/payroll-number",
	"GET /{id}/schedule",
	"GET /{id}/school-classes",
	"GET /{id}/stammdaten",
	"GET /{id}/stammdaten/bank-steuer",
	"GET /{id}/time-tracking/adjustments",
	"GET /{id}/time-tracking/comp-time-preview",
	"GET /{id}/time-tracking/export",
	"GET /{id}/time-tracking/history",
	"GET /{id}/time-tracking/month-summary",
	"GET /{id}/time-tracking/schedule-targets",
	"GET /{id}/time-tracking/sessions/{sessionId}/edits",
	"GET /{id}/vacation/quota",
	"POST /",
	"POST /absences/{absenceId}/approve",
	"POST /absences/{absenceId}/deny",
	"POST /absences/{absenceId}/question",
	"POST /time-tracking/month-close",
	"POST /{id}/absences",
	"POST /{id}/documents",
	"POST /{id}/stammdaten/bank-steuer/reveal",
	"POST /{id}/time-tracking/adjustments",
	"POST /{id}/time-tracking/month-close/reopen",
	"POST /{id}/time-tracking/opening",
	"POST /{id}/time-tracking/reset",
	"POST /{id}/time-tracking/sessions",
	"POST /{id}/vacation/opening",
	"PUT /pin",
	"PUT /{id}",
	"PUT /{id}/payroll-number",
	"PUT /{id}/schedule",
	"PUT /{id}/school-classes",
	"PUT /{id}/stammdaten/arbeitsvertrag",
	"PUT /{id}/stammdaten/bank-steuer",
	"PUT /{id}/stammdaten/kontakt",
	"PUT /{id}/stammdaten/person",
	"PUT /{id}/stammdaten/qualifikationen",
	"PUT /{id}/time-tracking/sessions/{sessionId}",
	"PUT /{id}/vacation/quota",
}

// TestStaffCompositionServesTheCompleteRouteSurface pins the union of both
// halves of the composed /api/staff router against the route table the
// pre-composition resource served.
func TestStaffCompositionServesTheCompleteRouteSurface(t *testing.T) {
	t.Parallel()
	composition := setupStaffCompositionRoute(t)

	var served []string
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// The builder mounts the composed router at /staff; chi.Walk reports
		// the mounted patterns with that prefix and a trailing slash.
		route = strings.TrimPrefix(route, "/staff")
		if route != "/" {
			route = strings.TrimSuffix(route, "/")
		}
		if route == "" {
			route = "/"
		}
		served = append(served, method+" "+route)
		return nil
	}
	require.NoError(t, chi.Walk(composition.router, walk))
	sort.Strings(served)

	expected := append([]string(nil), staffRouteSurface...)
	sort.Strings(expected)
	assert.Equal(t, expected, served)
}
