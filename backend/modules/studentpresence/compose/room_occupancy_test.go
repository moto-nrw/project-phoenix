package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoomOccupancyUsesOneTenantScopedAggregate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	closeActiveGroup := func(groupID int64) {
		t.Helper()
		_, err := db.NewUpdate().
			TableExpr(`active.groups`).
			Set(`end_time = ?`, time.Now()).
			Where(`id = ?`, groupID).
			Exec(context.Background())
		require.NoError(t, err)
	}

	createSupervisorForTenant := func(tenantID, staffID, groupID int64) {
		t.Helper()
		_, err := db.NewRaw("INSERT INTO active.group_supervisors (tenant_id,staff_id,group_id,role,start_date) VALUES (?,?,?,'lead',CURRENT_DATE)", tenantID, staffID, groupID).Exec(context.Background())
		require.NoError(t, err)
	}

	addClosedGroup := func(activityID, roomID int64) {
		t.Helper()
		closed := testpkg.CreateTestActiveGroup(t, db, activityID, roomID)
		closeActiveGroup(closed.ID)
		student := testpkg.CreateTestStudent(t, db, "Emil", "Fuenf", "1b")
		testpkg.CreateTestVisit(t, db, student.ID, closed.ID, time.Now().Add(-time.Hour), nil)
	}

	addForeignOccupancy := func() int64 {
		t.Helper()
		tenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, tenantID)
		room := testpkg.CreateTestRoomForTenant(t, db, tenantID, "Fuchsbau")
		activity := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Sport")
		group := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, tenantID, activity.ID, room.ID)
		student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Fritz", "Sechs", "2a")
		testpkg.CreateTestVisitForTenant(t, db, tenantID, student.ID, group.ID, time.Now(), nil)
		staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Gabi", "Sieben")
		createSupervisorForTenant(tenantID, staff.ID, group.ID)
		return room.ID
	}

	setupOccupancyFixture := func() occupancyFixture {
		t.Helper()
		room := testpkg.CreateTestRoom(t, db, "Igelraum")
		activity := testpkg.CreateTestActivityGroup(t, db, "Atelier")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		studentA := testpkg.CreateTestStudent(t, db, "Anna", "Eins", "1a")
		studentB := testpkg.CreateTestStudent(t, db, "Berta", "Zwei", "1a")
		testpkg.CreateTestVisit(t, db, studentA.ID, group.ID, time.Now().Add(-time.Hour), nil)
		testpkg.CreateTestVisit(t, db, studentB.ID, group.ID, time.Now().Add(-time.Hour), nil)
		staffA := testpkg.CreateTestStaff(t, db, "Carla", "Drei")
		staffB := testpkg.CreateTestStaff(t, db, "Dora", "Vier")
		testpkg.CreateTestGroupSupervisor(t, db, staffA.ID, group.ID, "lead")
		testpkg.CreateTestGroupSupervisor(t, db, staffB.ID, group.ID, "support")
		addClosedGroup(activity.ID, room.ID)
		foreignRoomID := addForeignOccupancy()
		return occupancyFixture{
			tenantID: testpkg.Tenant(t), roomID: room.ID, foreignRoomID: foreignRoomID,
			activityID: activity.ID, staffIDs: []int64{staffA.ID, staffB.ID},
		}
	}

	var queryCount int
	repo, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { queryCount = o.Queries }})
	require.NoError(t, err)
	fixture := setupOccupancyFixture()

	var rows []studentpresence.RoomOccupancy
	err = testpkg.WithinTenantContext(t, context.Background(), db, fixture.tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.ListRoomOccupancy(ctx, []int64{fixture.roomID, fixture.foreignRoomID})
		return err
	})

	require.NoError(t, err)
	require.Equal(t, 1, queryCount)
	require.Len(t, rows, 1, "RLS must hide the foreign tenant aggregate")
	assert.Equal(t, fixture.roomID, rows[0].RoomID)
	assert.Equal(t, []int64{fixture.activityID}, rows[0].ActivityGroupIDs)
	assert.Equal(t, 2, rows[0].StudentCount, "join multiplication and closed sessions must not inflate the count")
	assert.ElementsMatch(t, fixture.staffIDs, rows[0].SupervisorStaffIDs)
}

type occupancyFixture struct {
	tenantID, roomID, foreignRoomID, activityID int64
	staffIDs                                    []int64
}

func TestRoomOccupancyEmptyReadRequiresTenantAndSkipsQuery(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var queryCount int
	repo, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { queryCount = o.Queries }})
	require.NoError(t, err)
	rows, err := repo.ListRoomOccupancy(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.NotNil(t, rows)
	require.Empty(t, rows)
	require.Zero(t, queryCount)
	_, err = repo.ListRoomOccupancy(context.Background(), nil)
	require.Error(t, err)
}
