package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type excusalBlocks struct {
	applyErr   error
	releaseErr error
}

func (b *excusalBlocks) ApplyPartialAbsence(context.Context, int64) (int, error) {
	return 0, b.applyErr
}
func (b *excusalBlocks) ReleasePartialAbsence(context.Context, int64) (int, error) {
	return 0, b.releaseErr
}

type excusalBaseline struct{ careplan.PickupBaselineReader }

func (excusalBaseline) Project(_ context.Context, ids []int64, from, _ calendar.Date) (*careplan.PickupBaselineProjection, error) {
	plans := careplan.PickupPlansByStudent{}
	for _, id := range ids {
		plans[id] = careplan.PickupPlanByDate{from: careplan.PickupWeek{1: &careplan.PickupSchedule{PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)}}}
	}
	return &careplan.PickupBaselineProjection{WeeklyByStudentDate: plans}, nil
}

func TestNativePartialAbsenceCompositionRollsBackBlockFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Partial", "Transaction", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Staff")
	blockErr := errors.New("block update failed")
	blocks := &excusalBlocks{applyErr: blockErr}
	service, err := compose.NewPartialAbsences(db, owner, blocks, nil)
	require.NoError(t, err)
	input := careplan.PartialAbsenceInput{StudentID: student.ID, Date: "2099-01-05", FromTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), StaffID: staff.ID}
	_, err = service.CreatePartialAbsence(ctx, input)
	require.ErrorIs(t, err, blockErr)
	rows, err := owner.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Empty(t, rows, "a failed block write must roll back the exception insert")

	blocks.applyErr = nil
	row, err := service.CreatePartialAbsence(ctx, input)
	require.NoError(t, err)
	require.Equal(t, testpkg.Tenant(t), row.TenantID)
	require.True(t, row.ExcusedOwnsPickupTime)
	require.False(t, row.ExcusedAuto)
	require.Equal(t, "14:00", row.PickupTime.Format("15:04"))
	blocks.releaseErr = blockErr
	require.ErrorIs(t, service.DeletePartialAbsence(ctx, row.ID, student.ID), blockErr)
	persisted, err := owner.FindPickupException(ctx, row.ID, false)
	require.NoError(t, err)
	require.NotNil(t, persisted.ExcusedFrom)
	blocks.releaseErr = nil
	require.NoError(t, service.DeletePartialAbsence(ctx, row.ID, student.ID))
	_, err = owner.FindPickupException(ctx, row.ID, false)
	require.ErrorIs(t, err, careplan.ErrStudentScheduleNotFound)
}

func TestNativeExcusalCompositionRestoresAutomaticDerivationAfterManualOverride(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Auto", "Restore", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Auto", "Staff")
	blocks := &excusalBlocks{}
	auto, err := compose.NewPickupAutoExcusal(compose.PickupExcusalDependencies{DB: db, Records: owner, Baselines: excusalBaseline{}, Blocks: blocks})
	require.NoError(t, err)
	manual, err := compose.NewPartialAbsences(db, owner, blocks, auto)
	require.NoError(t, err)
	date := calendar.Date("2099-01-05")
	require.Equal(t, time.Monday, date.Weekday())
	pickup := time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
	row, err := owner.CreatePickupException(ctx, careplan.PickupException{
		StudentID: student.ID, ExceptionDate: careplan.Date(date), PickupTime: &pickup,
		Source: careplan.ExceptionSourceStaff, CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if err := owner.LockStudentAndExceptionDay(txCtx, student.ID, date.String()); err != nil {
			return err
		}
		_, err := auto.Sync(txCtx, row.ID)
		return err
	})
	require.NoError(t, err)
	automatic, err := owner.FindPickupException(ctx, row.ID, false)
	require.NoError(t, err)
	require.True(t, automatic.ExcusedAuto)
	require.Equal(t, "14:00", automatic.ExcusedFrom.Format("15:04"))

	override, err := manual.CreatePartialAbsence(ctx, careplan.PartialAbsenceInput{
		StudentID: student.ID, Date: date, FromTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), StaffID: staff.ID,
	})
	require.NoError(t, err)
	require.Equal(t, row.ID, override.ID)
	require.False(t, override.ExcusedAuto)
	require.False(t, override.ExcusedOwnsPickupTime)
	require.Equal(t, "14:00", override.PickupTime.Format("15:04"))
	require.Equal(t, "15:00", override.ExcusedFrom.Format("15:04"))
	require.NoError(t, manual.DeletePartialAbsence(ctx, row.ID, student.ID))
	restored, err := owner.FindPickupException(ctx, row.ID, false)
	require.NoError(t, err)
	require.True(t, restored.ExcusedAuto)
	require.Equal(t, "14:00", restored.ExcusedFrom.Format("15:04"))
	require.Nil(t, restored.ExcusedCreatedBy)
	require.ErrorIs(t, manual.DeletePartialAbsence(ctx, row.ID, student.ID), careplan.ErrPartialAbsenceAutoManaged)
}
