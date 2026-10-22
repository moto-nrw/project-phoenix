package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// checkExternRoleRoutePermissions is the per-route permission test of the
// school role „Extern“ (#4573) on the assembled production router. A token
// with exactly the role's default permissions runs the caller's own block of
// the day („Heute“: planned-now, roster, start, check-in, check-out,
// complete) and is refused on time tracking, staff, settings, roles, parent
// messaging, student directory and documents, guardians and finance.
func checkExternRoleRoutePermissions(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	external, account := testpkg.CreateTestTeacherWithAccount(t, db, "Extern", "Konto")
	claims := testutil.TenantUserTestClaims(int(account.ID), tenantID, identityaccess.ExternalCaregiverDefaultPermissions...)
	claims.Roles = []string{identityaccess.ExternalCaregiverRoleName}
	token := testutil.MintTestJWT(t, claims)

	testpkg.EnsureWebManualDevice(t, db)
	room := testpkg.CreateTestRoom(t, db, "AG-Raum")
	block := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title: "AG Musik", StartHHMM: "00:00", EndHHMM: "23:59",
	})
	testpkg.CreateTestInstanceStaff(t, db, block.ID, external.StaffID, testpkg.InstanceStaffOpts{IsPrimary: true})
	child := testpkg.CreateTestStudent(t, db, "Mila", "Kind", "2a")
	testpkg.CreateTestInstanceStudent(t, db, block.ID, child.ID, "expected")
	staffMember := testpkg.CreateTestStaff(t, db, "Andere", "Kollegin")
	operations := fmt.Sprintf("/api/timetable/operations/instances/%d", block.ID)

	request := func(method, path, body string) (int, string) {
		response := checkpointRequest(api, checkpointScenario{Method: method, Path: path, Authenticated: true, Body: body}, token)
		return response.Code, response.Body.String()
	}

	t.Run("own block of the day", func(t *testing.T) {
		for _, step := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/timetable/operations/planned-now", ""},
			{http.MethodGet, operations + "/roster", ""},
			{http.MethodPost, operations + "/start", "{}"},
			{http.MethodPost, fmt.Sprintf("%s/students/%d/check-in", operations, child.ID), "{}"},
			{http.MethodPost, fmt.Sprintf("%s/students/%d/check-out", operations, child.ID), "{}"},
			{http.MethodPost, operations + "/complete", "{}"},
		} {
			status, body := request(step.method, step.path, step.body)
			require.Truef(t, status >= 200 && status < 300, "%s %s: %d %s", step.method, step.path, status, body)
		}
	})

	t.Run("refused areas", func(t *testing.T) {
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/time-tracking/current"},
			{http.MethodGet, "/api/time-tracking/history"},
			{http.MethodGet, "/api/time-tracking/absences"},
			{http.MethodGet, "/api/time-tracking/month-summary"},
			{http.MethodPost, "/api/time-tracking/check-in"},
			{http.MethodGet, "/api/staff/"},
			{http.MethodGet, fmt.Sprintf("/api/staff/%d", staffMember.ID)},
			{http.MethodGet, fmt.Sprintf("/api/staff/financial-profile/%d", staffMember.ID)},
			{http.MethodGet, fmt.Sprintf("/api/staff/%d/documents", staffMember.ID)},
			{http.MethodPost, "/api/staff/externals"},
			{http.MethodGet, "/api/settings/payroll-status"},
			{http.MethodPut, "/api/settings/values/enrollment.enabled"},
			{http.MethodGet, "/auth/roles/"},
			{http.MethodPost, "/auth/roles/"},
			{http.MethodGet, "/api/messages/"},
			{http.MethodGet, "/api/guardians/"},
			{http.MethodGet, "/api/students/"},
			{http.MethodGet, fmt.Sprintf("/api/students/%d/documents", child.ID)},
		} {
			status, body := request(route.method, route.path, "{}")
			require.Equalf(t, http.StatusForbidden, status, "%s %s: %s", route.method, route.path, body)
		}
	})
}
