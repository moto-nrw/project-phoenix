package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// checkGuestRoutePermissions is the per-route permission test of the system
// role guest („Gast“, access level „Nur ansehen“, ADR 0091). A token with
// exactly the role's migrated permissions reads children, rooms, groups and
// AGs, and is refused on everything that carries parent data, messages,
// notes, exports and staff.
func checkGuestRoutePermissions(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	granted := systemRolePermissions(t, db, identityaccess.GuestRoleName)
	require.Equal(t, []string{"activities:read", "groups:read", "rooms:read", "students:read"}, granted)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Gast", "Konto")
	claims := testutil.TenantUserTestClaims(int(account.ID), tenantID, granted...)
	claims.Roles = []string{identityaccess.GuestRoleName}
	token := testpkg.MintLiveSessionJWT(t, db, claims)

	child := testpkg.CreateTestStudent(t, db, "Mila", "Kind", "2a")
	testpkg.SetTestTenantSetting(t, db, tenantID, "operations.student_photos_enabled", "true")
	// The current-visit read answers about a visit, so the child has one.
	room := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	testpkg.CreateTestVisit(t, db, child.ID, room.ID, time.Now().Add(-time.Hour), nil)
	request := func(method, path string) (int, string) {
		response := checkpointRequest(api, checkpointScenario{Method: method, Path: path, Authenticated: true, Body: "{}"}, token)
		return response.Code, response.Body.String()
	}

	t.Run("reads", func(t *testing.T) {
		for _, path := range []string{
			"/api/students/",
			fmt.Sprintf("/api/students/%d", child.ID),
			fmt.Sprintf("/api/students/%d/current-visit", child.ID),
			"/api/students/school-classes",
			"/api/students/ogs-group-live",
			"/api/groups/",
			"/api/rooms/",
			"/api/activities/",
		} {
			status, body := request(http.MethodGet, path)
			require.Equalf(t, http.StatusOK, status, "GET %s: %s", path, body)
		}
		// The child has no photo: the gate lets the guest through to the
		// handler, which answers that the file does not exist.
		status, body := request(http.MethodGet, fmt.Sprintf("/api/students/%d/photo/none.jpg", child.ID))
		require.Equalf(t, http.StatusNotFound, status, "photo: %s", body)
	})

	t.Run("refused", func(t *testing.T) {
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/students/status-days"},
			{http.MethodGet, "/api/guardians/"},
			{http.MethodGet, "/api/messages/"},
			{http.MethodGet, fmt.Sprintf("/api/messages/students/%d/guardians", child.ID)},
			{http.MethodGet, fmt.Sprintf("/api/students/%d/notes", child.ID)},
			{http.MethodGet, fmt.Sprintf("/api/students/%d/visit-history", child.ID)},
			{http.MethodGet, fmt.Sprintf("/api/students/%d/attendance-history/export", child.ID)},
			{http.MethodPost, "/api/students/export"},
			{http.MethodGet, "/api/students/day-log/export"},
			{http.MethodGet, "/api/staff/"},
			{http.MethodPut, fmt.Sprintf("/api/students/%d", child.ID)},
			{http.MethodGet, "/api/time-tracking/current"},
		} {
			status, body := request(route.method, route.path)
			require.Equalf(t, http.StatusForbidden, status, "%s %s: %s", route.method, route.path, body)
		}
	})
}
