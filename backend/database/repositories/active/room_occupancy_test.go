package active_test

import (
	"context"
	"testing"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestGroupRepositoryListRoomOccupancyUsesOneTenantScopedAggregate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := activeRepo.NewGroupRepository(db)
	fixture := setupOccupancyFixture(t, db)

	var rows []activeModels.RoomOccupancy
	err := testpkg.WithTenantTx(t, context.Background(), db, fixture.tenantID, func(ctx context.Context, _ bun.Tx) error {
		var err error
		rows, err = repo.ListRoomOccupancy(ctx, []int64{fixture.roomID, fixture.foreignRoomID})
		return err
	})

	require.NoError(t, err)
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

func setupOccupancyFixture(t *testing.T, db *bun.DB) occupancyFixture {
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
	addClosedGroup(t, db, activity.ID, room.ID)
	foreignRoomID := addForeignOccupancy(t, db)
	return occupancyFixture{
		tenantID: testpkg.Tenant(t), roomID: room.ID, foreignRoomID: foreignRoomID,
		activityID: activity.ID, staffIDs: []int64{staffA.ID, staffB.ID},
	}
}

func addClosedGroup(t *testing.T, db *bun.DB, activityID, roomID int64) {
	t.Helper()
	closed := testpkg.CreateTestActiveGroup(t, db, activityID, roomID)
	closeActiveGroup(t, db, closed.ID)
	student := testpkg.CreateTestStudent(t, db, "Emil", "Fuenf", "1b")
	testpkg.CreateTestVisit(t, db, student.ID, closed.ID, time.Now().Add(-time.Hour), nil)
}

func addForeignOccupancy(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, "Fuchsbau")
	activity := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Sport")
	group := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, tenantID, activity.ID, room.ID)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Fritz", "Sechs", "2a")
	testpkg.CreateTestVisitForTenant(t, db, tenantID, student.ID, group.ID, time.Now(), nil)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Gabi", "Sieben")
	createSupervisorForTenant(t, db, tenantID, staff.ID, group.ID)
	return room.ID
}

func closeActiveGroup(t *testing.T, db *bun.DB, groupID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr(`active.groups`).
		Set(`end_time = ?`, time.Now()).
		Where(`id = ?`, groupID).
		Exec(context.Background())
	require.NoError(t, err)
}

func createSupervisorForTenant(t *testing.T, db *bun.DB, tenantID, staffID, groupID int64) {
	t.Helper()
	supervisor := &activeModels.GroupSupervisor{
		StaffID: staffID, GroupID: groupID, Role: "lead", StartDate: timezone.TodayDate(),
	}
	supervisor.SetTenantID(tenantID)
	err := db.NewInsert().Model(supervisor).ModelTableExpr(`active.group_supervisors`).Scan(context.Background())
	require.NoError(t, err)
}

func TestGroupRepositoryListRoomOccupancySkipsDatabaseForEmptyIDs(t *testing.T) {
	t.Parallel()
	repo := activeRepo.NewGroupRepository(nil)

	rows, err := repo.ListRoomOccupancy(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.NotNil(t, rows)
}
