package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresenceTablesEnforceRLSWithoutTenantPredicates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "RLS", "Owner", "3a")
	staff := testpkg.CreateTestStaff(t, db, "RLS", "Staff")
	device := testpkg.CreateTestDevice(t, db, "presence-rls-owner")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	at := time.Now().Add(-time.Hour)
	ownAttendance := testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, at, nil)
	ownVisit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, at, nil)
	var foreignAttendanceID, foreignVisitID, foreignTenantID int64
	var foreignCtx context.Context
	t.Run("foreign fixtures", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignTenantID = testpkg.Tenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "RLS", "Foreign", "3a")
		staff := testpkg.CreateTestStaff(t, db, "RLS", "ForeignStaff")
		device := testpkg.CreateTestDevice(t, db, "presence-rls-foreign")
		group := testpkg.CreateTestActiveGroupForTenant(t, db, foreignTenantID)
		foreignAttendanceID = testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, at, nil).ID
		foreignVisitID = testpkg.CreateTestVisit(t, db, student.ID, group.ID, at, nil).ID
	})
	for _, tc := range []struct {
		table            string
		ownID, foreignID int64
	}{
		{table: "active.attendance", ownID: ownAttendance.ID, foreignID: foreignAttendanceID},
		{table: "active.visits", ownID: ownVisit.ID, foreignID: foreignVisitID},
	} {
		t.Run(tc.table, func(t *testing.T) {
			testpkg.AssertTenantRowIsolation(t, db, ctx, foreignCtx, tc.table, tc.ownID, tc.foreignID)
		})
	}
}

func TestPresenceQueriesAndCommandsRespectTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	var foreignCtx context.Context
	var studentID, groupID int64
	at := time.Now()
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "Presence", "Foreign", "3a")
		staff := testpkg.CreateTestStaff(t, db, "Presence", "Staff")
		device := testpkg.CreateTestDevice(t, db, "presence-foreign")
		activity := testpkg.CreateTestActivityGroup(t, db, "Presence foreign")
		room := testpkg.CreateTestRoom(t, db, "Presence foreign")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, at.Add(-time.Hour), nil)
		testpkg.CreateTestVisit(t, db, student.ID, group.ID, at.Add(-time.Hour), nil)
		studentID, groupID = student.ID, group.ID
	})
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		open, err := module.ListOpenPresence(txCtx, []int64{studentID})
		require.NoError(t, err)
		assert.Empty(t, open)
		day, err := module.LatestPresenceDate(txCtx, studentID)
		require.NoError(t, err)
		assert.Nil(t, day)
		count, err := module.CountAttendanceRecords(txCtx, studentID)
		require.NoError(t, err)
		assert.Zero(t, count)
		require.NoError(t, module.LockOpenPresence(txCtx, []int64{studentID}))
		require.NoError(t, module.LockOpenVisits(txCtx, groupID))
		rows, err := module.CloseOpenPresence(txCtx, []int64{studentID}, at)
		require.NoError(t, err)
		assert.Zero(t, rows)
		return nil
	}))
	require.NoError(t, tenant.WithinCurrentTenant(foreignCtx, func(txCtx context.Context) error {
		rows, err := module.CloseOpenPresence(txCtx, []int64{studentID}, at)
		require.NoError(t, err)
		assert.EqualValues(t, 2, rows, "neither table was changed by the other tenant")
		return nil
	}))
	_, err = module.ListOpenPresence(context.Background(), []int64{studentID})
	require.Error(t, err, "missing tenant must not become an unscoped read")
	_, err = module.CloseOpenPresence(ctx, []int64{studentID}, at)
	require.Error(t, err, "a write requires the caller's transaction")
}
