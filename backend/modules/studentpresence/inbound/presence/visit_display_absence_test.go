package presence_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	tenantsettings "github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sick and excused status with their timestamps need users:read, like in the
// child list and group view (ADR 0091). A guest reads the visit display with
// groups:read only.
func TestVisitDisplayAbsenceStatusRequiresUsersRead(t *testing.T) {
	t.Parallel()
	tc, router := setupFullCoverageRouter(t)
	require.NoError(t, tc.settings.SetString(
		testpkg.Ctx(t), tenantsettings.KeyOperationalOverviewScope, tenantsettings.OverviewScopeOwn, nil, nil,
	))

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Gast", "Aufsicht")
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("AbsenceRoom %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("AbsenceGroup %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "supervisor")
	child := testpkg.CreateTestStudent(t, tc.db, "Mila", "Krank", "2a")
	testpkg.CreateTestVisit(t, tc.db, child.ID, activeGroup.ID, time.Now().Add(-time.Hour), nil)
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.student_care_profiles SET sick = true, sick_since = now(), excused = true, excused_since = now()
		WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`,
		child.ID)
	require.NoError(t, err)

	for _, granted := range [][]string{
		{permissions.GroupsRead, permissions.StudentsRead},
		{permissions.GroupsRead, permissions.UsersRead},
	} {
		claims := testutil.Claims{
			ID: int(account.ID), TenantID: testpkg.Tenant(t), Sub: "absence@example.com",
			Roles: []string{"staff"}, Permissions: granted,
		}
		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/groups/%d/visits/display", activeGroup.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, granted)
		require.Equalf(t, http.StatusOK, rr.Code, "%v: %s", granted, rr.Body.String())

		var body struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Len(t, body.Data, 1)
		visit := body.Data[0]
		allowed := granted[1] == permissions.UsersRead
		assert.Equalf(t, allowed, visit["sick"], "%v", granted)
		assert.Equalf(t, allowed, visit["excused"], "%v", granted)
		_, hasSickSince := visit["sick_since"]
		_, hasExcusedSince := visit["excused_since"]
		assert.Equalf(t, allowed, hasSickSince, "%v", granted)
		assert.Equalf(t, allowed, hasExcusedSince, "%v", granted)
	}
}
