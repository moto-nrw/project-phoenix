package compose

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// failingAnchorRebase composes the module with a School Membership stand-in
// that fails on the anchor rebase. That call sits between the two
// authoritative writes of a template update — the entries have been replaced
// and the running schedule versions closed, but the new versions are not
// inserted yet — so it is the sharpest point to prove the whole unit of work
// rolls back.
func failingAnchorRebase(t *testing.T, db *bun.DB, failure error) workforce.Capability {
	t.Helper()
	runtime := testpkg.ConfigRuntime(db)
	capability, err := New(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment,
		DB:               db,
		AssignedStaffIDs: runtime.AssignedStaffIDs,
		RebaseStaffAnchor: func(context.Context, int64, string) ([]int64, error) {
			return nil, failure
		},
		Observe: func(Observation) {},
	})
	require.NoError(t, err)
	return capability
}

func TestWorkTimeModelUpdate_RollsBackEveryWriteAndRetriesCleanly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Template")
	capability := buildWorkforce(t, db)

	created, err := capability.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{
		WorkTimeModelFields: workforce.WorkTimeModelFields{
			Name:               "Rollback safety",
			RotationLength:     1,
			RotationAnchorDate: "2026-01-05",
			Entries: []workforce.WorkTimeModelEntry{
				{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 300},
			},
		},
	})
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewUpdate().Table("users.staff").
			Set("work_time_model_id = NULL").Where("id = ?", staff.ID).Exec(ctx)
		_ = capability.DeleteWorkTimeModel(ctx, created.ID)
	}()

	schedules := repositoryadapter.NewStaffWorkScheduleRepository(capability)
	require.NoError(t, schedules.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayMonday, TargetMinutes: 300},
	}, configModel.CalendarDate("")))
	_, err = db.NewUpdate().Table("users.staff").
		Set("work_time_model_id = ?", created.ID).
		Where("id = ?", staff.ID).Exec(ctx)
	require.NoError(t, err)

	revision := workforce.UpdateWorkTimeModel{
		ID: created.ID,
		WorkTimeModelFields: workforce.WorkTimeModelFields{
			Name:               "Rollback safety",
			RotationLength:     1,
			RotationAnchorDate: "2026-06-01",
			Entries: []workforce.WorkTimeModelEntry{
				{WeekIndex: 0, DayOfWeek: configModel.DayTuesday, TargetMinutes: 420},
			},
		},
	}

	failure := errors.New("school membership unavailable")
	_, err = failingAnchorRebase(t, db, failure).UpdateWorkTimeModel(ctx, revision)
	require.ErrorIs(t, err, failure)

	// Nothing the failed call wrote may survive: not the template entry, not
	// the closed schedule version.
	rolledBack, err := capability.FindWorkTimeModel(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "2026-01-05", rolledBack.RotationAnchorDate, "the template anchor must be rolled back")
	require.Len(t, rolledBack.Entries, 1)
	assert.Equal(t, configModel.DayMonday, rolledBack.Entries[0].DayOfWeek, "the template entry must be rolled back")

	current, err := capability.CurrentStaffSchedule(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, current, 1, "the running schedule version must not have been closed")
	assert.Equal(t, configModel.DayMonday, current[0].DayOfWeek)
	assert.Equal(t, 300, current[0].TargetMinutes)

	var closed int
	require.NoError(t, db.NewSelect().
		TableExpr("config.staff_work_schedules").
		ColumnExpr("COUNT(*)").
		Where("staff_id = ?", staff.ID).
		Where("valid_until IS NOT NULL").
		Scan(ctx, &closed))
	assert.Zero(t, closed, "the rolled-back close must leave no tombstoned version")

	// Retrying the identical revision against a healthy owner succeeds and
	// leaves exactly one current version per template entry.
	applied, err := capability.UpdateWorkTimeModel(ctx, revision)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01", applied.RotationAnchorDate)

	current, err = capability.CurrentStaffSchedule(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	assert.Equal(t, configModel.DayTuesday, current[0].DayOfWeek)
	assert.Equal(t, 420, current[0].TargetMinutes)
}

func TestReplaceStaffSchedule_RollsBackWhenAnEntryIsInvalid(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Schedule")
	capability := buildWorkforce(t, db)

	require.NoError(t, capability.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{
		StaffID: staff.ID,
		Entries: []workforce.StaffWorkScheduleEntry{
			{WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayMonday, TargetMinutes: 300},
		},
	}))

	// A rejected entry must not close the running version on its way out.
	err := capability.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{
		StaffID: staff.ID,
		Entries: []workforce.StaffWorkScheduleEntry{
			{WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayTuesday, TargetMinutes: 240},
			{WeekIndex: 0, RotationLength: 1, DayOfWeek: 9, TargetMinutes: 240},
		},
	})
	require.ErrorIs(t, err, workforce.ErrInvalidWorkTime)

	current, err := capability.CurrentStaffSchedule(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	assert.Equal(t, configModel.DayMonday, current[0].DayOfWeek)
	assert.Equal(t, 300, current[0].TargetMinutes)
}
