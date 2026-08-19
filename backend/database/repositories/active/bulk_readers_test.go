package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestGroupSupervisorRepository_ListActiveSupervisedRooms pins the whole-tenant
// (staff, room) projection that replaces the per-staff FindActiveByStaffID plus
// GetActiveGroupsByIDs walk.
func TestGroupSupervisorRepository_ListActiveSupervisedRooms(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.TenantContext(1)

	room := testpkg.CreateTestRoom(t, db, "BulkSupervisedRoom")
	otherRoom := testpkg.CreateTestRoom(t, db, "BulkSupervisedRoomOther")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "BulkSupervisedActivity")
	staff := testpkg.CreateTestStaff(t, db, "BulkSupervised", "Person")

	openGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	closedGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, otherRoom.ID)

	// Close the second session: a room whose session already ended has no
	// presence left and must not surface as supervised.
	endTime := time.Now()
	_, err := db.NewUpdate().
		TableExpr("active.groups").
		Set("end_time = ?", endTime).
		Where("id = ?", closedGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	openSupervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, openGroup.ID, "primary")
	closedSupervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, closedGroup.ID, "primary")

	defer testpkg.CleanupActivityFixtures(t, db,
		openSupervision.ID, closedSupervision.ID,
		openGroup.ID, closedGroup.ID,
		activityGroup.ID, room.ID, otherRoom.ID,
	)
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

	t.Run("returns the room of an open supervised session", func(t *testing.T) {
		rows, err := repo.ListActiveSupervisedRooms(ctx)
		require.NoError(t, err)

		var openFound, closedFound bool
		for _, row := range rows {
			if row.StaffID != staff.ID {
				continue
			}
			if row.RoomID == room.ID {
				openFound = true
			}
			if row.RoomID == otherRoom.ID {
				closedFound = true
			}
		}

		assert.True(t, openFound, "open supervision should surface its room")
		assert.False(t, closedFound, "a session with end_time set must not surface its room")
	})

	t.Run("excludes a supervision that ended before today", func(t *testing.T) {
		// end_date in the past mirrors IsSupervisorActive returning false.
		yesterday := timezone.TodayDate().AddDays(-1)
		_, err := db.NewUpdate().
			TableExpr("active.group_supervisors").
			Set("end_date = ?", yesterday).
			Where("id = ?", openSupervision.ID).
			Exec(ctx)
		require.NoError(t, err)

		defer func() {
			_, resetErr := db.NewUpdate().
				TableExpr("active.group_supervisors").
				Set("end_date = NULL").
				Where("id = ?", openSupervision.ID).
				Exec(ctx)
			require.NoError(t, resetErr)
		}()

		rows, err := repo.ListActiveSupervisedRooms(ctx)
		require.NoError(t, err)

		for _, row := range rows {
			assert.False(t, row.StaffID == staff.ID && row.RoomID == room.ID,
				"an expired supervision must not surface")
		}
	})

	t.Run("uses the Berlin date independently of the database timezone", func(t *testing.T) {
		today := timezone.TodayDate()
		err := tenant.WithTenantTx(context.Background(), db, 1, func(txCtx context.Context, tx bun.Tx) error {
			var databaseDate string
			for _, zone := range []string{"Pacific/Kiritimati", "Etc/GMT+12"} {
				var configuredZone string
				if err := tx.NewSelect().
					ColumnExpr(`set_config('TimeZone', ?, true)`, zone).
					Scan(txCtx, &configuredZone); err != nil {
					return err
				}
				if err := tx.NewSelect().ColumnExpr("CURRENT_DATE::text").Scan(txCtx, &databaseDate); err != nil {
					return err
				}
				if databaseDate != today.String() {
					break
				}
			}
			require.NotEqual(t, today.String(), databaseDate)

			endDate := today
			wantFound := false
			if databaseDate > today.String() {
				endDate = today.AddDays(1)
				wantFound = true
			}
			if _, err := tx.NewUpdate().
				TableExpr("active.group_supervisors").
				Set("end_date = ?", endDate).
				Where("id = ?", openSupervision.ID).
				Exec(txCtx); err != nil {
				return err
			}

			rows, err := repo.ListActiveSupervisedRooms(txCtx)
			if err != nil {
				return err
			}
			found := false
			for _, row := range rows {
				if row.StaffID == staff.ID && row.RoomID == room.ID {
					found = true
					break
				}
			}
			assert.Equal(t, wantFound, found,
				"database session date must not change Berlin supervision expiry")
			return nil
		})
		require.NoError(t, err)

		_, err = db.NewUpdate().
			TableExpr("active.group_supervisors").
			Set("end_date = NULL").
			Where("id = ?", openSupervision.ID).
			Exec(ctx)
		require.NoError(t, err)
	})

	t.Run("does not leak another tenant's supervisions", func(t *testing.T) {
		const otherTenant int64 = 99042
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignRoom := testpkg.CreateTestRoomForTenant(t, db, otherTenant, "ForeignSupervisedRoom")
		foreignActivity := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenant, "ForeignSupervisedActivity")
		foreignStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Foreign", "Supervisor")
		foreignGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, otherTenant, foreignActivity.ID, foreignRoom.ID)

		foreignSupervision := &active.GroupSupervisor{
			StaffID:   foreignStaff.ID,
			GroupID:   foreignGroup.ID,
			Role:      "primary",
			StartDate: timezone.TodayDate(),
		}
		foreignSupervision.SetTenantID(otherTenant)
		_, err := db.NewInsert().
			Model(foreignSupervision).
			ModelTableExpr("active.group_supervisors").
			Exec(ctx)
		require.NoError(t, err)

		defer testpkg.CleanupActivityFixturesForTenant(t, db, otherTenant,
			foreignSupervision.ID, foreignGroup.ID, foreignActivity.ID, foreignRoom.ID)
		defer testpkg.CleanupStaffFixtures(t, db, foreignStaff.ID)

		rows, err := repo.ListActiveSupervisedRooms(ctx)
		require.NoError(t, err)

		for _, row := range rows {
			assert.NotEqual(t, foreignStaff.ID, row.StaffID,
				"tenant 1 must not see another tenant's supervisions")
		}
	})
}

// TestVisitRepository_ListOpenVisitStudentIDsByRoom pins the whole-tenant
// room -> present students projection that replaces the per-room query loop.
func TestVisitRepository_ListOpenVisitStudentIDsByRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)

	room := testpkg.CreateTestRoom(t, db, "BulkVisitRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "BulkVisitActivity")
	openGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	present := testpkg.CreateTestStudent(t, db, "Present", "Child", "1a")
	departed := testpkg.CreateTestStudent(t, db, "Departed", "Child", "1a")

	entry := time.Now().Add(-time.Hour)
	exit := time.Now().Add(-10 * time.Minute)
	openVisit := testpkg.CreateTestVisit(t, db, present.ID, openGroup.ID, entry, nil)
	closedVisit := testpkg.CreateTestVisit(t, db, departed.ID, openGroup.ID, entry, &exit)

	defer testpkg.CleanupActivityFixtures(t, db,
		openVisit.ID, closedVisit.ID, openGroup.ID, activityGroup.ID, room.ID,
		present.ID, departed.ID,
	)

	t.Run("groups open visits by room and drops checked-out children", func(t *testing.T) {
		byRoom, err := repo.ListOpenVisitStudentIDsByRoom(ctx)
		require.NoError(t, err)

		assert.Contains(t, byRoom[room.ID], present.ID,
			"a child with an open visit belongs to its room")
		assert.NotContains(t, byRoom[room.ID], departed.ID,
			"a child who already left must not be reported as present")
	})

	t.Run("agrees with the single-room reader it replaces", func(t *testing.T) {
		byRoom, err := repo.ListOpenVisitStudentIDsByRoom(ctx)
		require.NoError(t, err)

		single, err := repo.ListActiveStudentIDsByRoomID(ctx, room.ID)
		require.NoError(t, err)

		assert.ElementsMatch(t, single, byRoom[room.ID],
			"the bulk reader must return exactly what the per-room reader returns")
	})

	t.Run("reports nothing for a room whose session has ended", func(t *testing.T) {
		_, err := db.NewUpdate().
			TableExpr("active.groups").
			Set("end_time = ?", time.Now()).
			Where("id = ?", openGroup.ID).
			Exec(ctx)
		require.NoError(t, err)

		defer func() {
			_, resetErr := db.NewUpdate().
				TableExpr("active.groups").
				Set("end_time = NULL").
				Where("id = ?", openGroup.ID).
				Exec(ctx)
			require.NoError(t, resetErr)
		}()

		byRoom, err := repo.ListOpenVisitStudentIDsByRoom(ctx)
		require.NoError(t, err)

		assert.NotContains(t, byRoom[room.ID], present.ID,
			"a closed session leaves no presence behind")
	})
}
