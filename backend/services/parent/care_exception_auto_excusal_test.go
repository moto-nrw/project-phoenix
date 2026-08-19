package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildCareServiceWithAutoExcusal mirrors buildCareService but wires the
// pickup auto-excusal syncer (#2360) the production factory injects. It
// returns the careTestService shim so the submit calls below keep reading as
// "parent sets a pickup time for one day" — the parent submit now carries a
// mandatory reason and no arrival leg.
func buildCareServiceWithAutoExcusal(t *testing.T) (careTestService, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		PickupAutoExcusal: scheduleSvc.NewPickupAutoExcusalSyncer(
			repos.StudentPickupException,
			repos.StudentPickupSchedule,
			repos.InstanceStudent,
			db,
		),
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentPickupChangeEnabled: true},
		},
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		DB:          db,
		Logger:      slog.Default(),
	})
	return careTestService{Service: svc}, db
}

// nextMonday returns the first Monday at least seven days out, safely inside
// the parent submit window (today .. +2 months).
func nextMonday() timezone.Date {
	date := timezone.TodayDate().AddDays(7)
	for date.Weekday() != time.Monday {
		date = date.AddDays(1)
	}
	return date
}

func loadSlotRow(t *testing.T, db *bun.DB, rowID int64) *scheduleModels.InstanceStudent {
	t.Helper()
	row := new(scheduleModels.InstanceStudent)
	err := db.NewSelect().
		Model(row).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".id = ?`, rowID).
		Scan(context.Background())
	require.NoError(t, err)
	return row
}

func TestSubmitCareException_PullForwardCouplesAndReleases(t *testing.T) {
	svc, db := buildCareServiceWithAutoExcusal(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Parent", "Baseline")
	room := testpkg.CreateTestRoom(t, db, "Parent auto excusal room")

	date := nextMonday()
	weekly := testpkg.CreateTestPickupSchedule(t, db, chain.StudentID, 1, staff.ID, "16:00")
	block := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Nachmittags-AG",
	})
	slot := testpkg.CreateTestInstanceStudent(t, db, block.ID, chain.StudentID, scheduleModels.AttendanceStatusExpected)
	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(
			t, db,
			nil, nil, []int64{weekly.ID}, nil,
			[]int64{slot.ID}, []int64{block.ID},
		)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, room.ID)
	})

	// Parent pulls the pickup forward to 14:45 → the 15:00 block is excused.
	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID, date, wallClock(14, 45), nil)
	require.NoError(t, err)

	row := loadSlotRow(t, db, slot.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, row.Status)
	require.NotNil(t, row.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *row.Substatus)
	require.NotNil(t, row.PickupExceptionID)

	exception, err := repositories.NewFactory(db).StudentPickupException.FindByStudentIDAndDate(
		testpkg.TenantContext(chain.TenantID), chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, exception)
	assert.True(t, exception.ExcusedAuto)
	assert.Nil(t, exception.ExcusedCreatedBy)

	// The auto excusal must not lock the parent out of their own exception:
	// moving the time back past the baseline releases the block again.
	_, err = svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID, date, wallClock(16, 30), nil)
	require.NoError(t, err)

	row = loadSlotRow(t, db, slot.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, row.Status)
	assert.Nil(t, row.PickupExceptionID)
}

func TestDeleteCareException_ReleasesAutoExcusedBlocks(t *testing.T) {
	svc, db := buildCareServiceWithAutoExcusal(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Parent", "Withdraw")
	room := testpkg.CreateTestRoom(t, db, "Parent withdraw room")

	date := nextMonday()
	weekly := testpkg.CreateTestPickupSchedule(t, db, chain.StudentID, 1, staff.ID, "16:00")
	block := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Nachmittags-AG",
	})
	slot := testpkg.CreateTestInstanceStudent(t, db, block.ID, chain.StudentID, scheduleModels.AttendanceStatusExpected)
	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(
			t, db,
			nil, nil, []int64{weekly.ID}, nil,
			[]int64{slot.ID}, []int64{block.ID},
		)
		testpkg.CleanupActivityFixtures(t, db, staff.ID, room.ID)
	})

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID, date, wallClock(14, 45), nil)
	require.NoError(t, err)
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, loadSlotRow(t, db, slot.ID).Status)

	require.NoError(t, svc.DeleteCareException(context.Background(), chain.AccountID, chain.StudentID, date))

	row := loadSlotRow(t, db, slot.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, row.Status)
	assert.Nil(t, row.PickupExceptionID)
	assert.Nil(t, row.Substatus)

	exception, err := repositories.NewFactory(db).StudentPickupException.FindByStudentIDAndDate(
		testpkg.TenantContext(chain.TenantID), chain.StudentID, date)
	require.NoError(t, err)
	assert.Nil(t, exception, "withdrawing the override removes the guardian row")
}
