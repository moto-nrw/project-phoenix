package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin bypass is granted by the validated HTTP principal, not by
// visits:update or the presence of a staff profile (#4736).
func checkMoveBypassAuthorization(t *testing.T, graph *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Move", "Actor")
	source := testpkg.CreateTestActiveGroup(t, db,
		testpkg.CreateTestActivityGroup(t, db, "Bypass source").ID,
		testpkg.CreateTestRoom(t, db, "Bypass source").ID)
	target := testpkg.CreateTestActiveGroup(t, db,
		testpkg.CreateTestActivityGroup(t, db, "Bypass target").ID,
		testpkg.CreateTestRoom(t, db, "Bypass target").ID)
	child := testpkg.CreateTestStudent(t, db, "Move", "Child", "3a")
	device := testpkg.CreateTestDevice(t, db, "bypass-contract")
	now := time.Now().Add(-time.Minute)
	testpkg.CreateTestAttendance(t, db, child.ID, staff.ID, device.ID, now, nil)
	testpkg.CreateTestVisit(t, db, child.ID, source.ID, now, nil)
	body := fmt.Sprintf(`{"student_ids":[%d],"target_active_group_id":%d}`, child.ID, target.ID)
	request := func(token string) *httptest.ResponseRecorder {
		return checkpointRequest(graph, checkpointScenario{
			Method: http.MethodPost, Path: "/api/active/visits/move-to-group", Authenticated: token != "", Body: body,
		}, token)
	}
	staffToken := testpkg.MintLiveSessionJWT(t, db, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Permissions: []string{"visits:update"},
	})
	refused := request(staffToken)
	require.Equal(t, http.StatusForbidden, refused.Code, refused.Body.String())
	assert.Contains(t, refused.Body.String(), `"code":"general.permission"`)
	assert.Equal(t, []int64{source.ID}, openVisitSessions(t, db, child.ID))
	anonymous := request("")
	require.Equal(t, http.StatusUnauthorized, anonymous.Code, anonymous.Body.String())
	assert.Equal(t, []int64{source.ID}, openVisitSessions(t, db, child.ID))

	// Admin resource access is school-wide, never cross-school. Give the
	// foreign admin a valid target in their own school, but this school's child.
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Admin")
	foreignTarget := testpkg.CreateTestActiveGroupForTenant(t, db, foreignTenant)
	foreignToken := testpkg.MintLiveSessionJWT(t, db, testutil.AdminTestClaimsForTenant(int(foreignAccount.ID), foreignTenant))
	foreignMove := checkpointRequest(graph, checkpointScenario{
		Method: http.MethodPost, Path: "/api/active/visits/move-to-group", Authenticated: true,
		Body: fmt.Sprintf(`{"student_ids":[%d],"target_active_group_id":%d}`, child.ID, foreignTarget.ID),
	}, foreignToken)
	require.Equal(t, http.StatusConflict, foreignMove.Code, foreignMove.Body.String())
	assert.Contains(t, foreignMove.Body.String(), `"code":"general.business_rejection"`)
	assert.Equal(t, []int64{source.ID}, openVisitSessions(t, db, child.ID), "foreign admin cannot move the child")

	adminToken := testpkg.MintLiveSessionJWT(t, db, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	moved := request(adminToken)
	require.Equal(t, http.StatusOK, moved.Code, moved.Body.String())
	assert.Equal(t, []int64{target.ID}, openVisitSessions(t, db, child.ID), "admin needs no source or target supervision")
}
