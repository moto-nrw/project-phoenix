package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestPickupChangeRequestAppliesOnlyAfterStaffApproval(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(2)
	pickup := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  f.chain.StudentID,
		Weekday:    weekday,
		PickupTime: timezone.WallClock(time.Date(1, 1, 1, 15, 30, 0, 0, time.UTC)),
		CreatedBy:  f.staffID,
	}))
	t.Cleanup(func() {
		require.NoError(t, f.sf.PickupSchedule.DeleteStudentPickupSchedule(ctx, f.chain.StudentID))
	})

	req, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		pickup,
		"Arzttermin",
	)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestKindPickupChange, req.RequestKind)
	assert.Equal(t, "15:30", req.Payload["previous_pickup_time"])

	before, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	assert.Nil(t, before)

	items, err := f.svc.ListPendingPickupChanges(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "14:30", items[0].Diff[0].New)

	decided, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, decided.Request.Status)

	applied, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, applied)
	require.NotNil(t, applied.PickupTime)
	assert.Equal(t, "14:30", applied.PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, applied.Source)
	assert.Equal(t, f.staffID, applied.CreatedBy)
	require.NotNil(t, applied.Reason)
	assert.Equal(t, "Arzttermin", *applied.Reason)
}
