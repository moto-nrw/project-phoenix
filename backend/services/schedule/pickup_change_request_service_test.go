package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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

// seedPickupChangeRequest books a pending parent pickup change for a future day.
func seedPickupChangeRequest(t *testing.T, f *careFixture, date timezone.Date) *scheduleModels.CareScheduleChangeRequest {
	t.Helper()
	req, err := f.svc.CreatePickupChangeRequest(
		f.staffCtx(f.staffAccount),
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC),
		"Arzttermin",
	)
	require.NoError(t, err)
	return req
}

// TestPickupChangeApprovalYieldsToStaffException: the OGS decides the day. If
// staff already set a pickup time for it, approving a parent request must not
// quietly overwrite that decision.
func TestPickupChangeApprovalYieldsToStaffException(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(3)
	req := seedPickupChangeRequest(t, f, date)

	staffPickup := time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC)
	staffReason := "Von der OGS gesetzt"
	require.NoError(t, f.repos.StudentPickupException.Create(ctx, &scheduleModels.StudentPickupException{
		TenantModel:   modelBase.TenantModel{TenantID: f.chain.TenantID},
		StudentID:     f.chain.StudentID,
		ExceptionDate: date,
		PickupTime:    &staffPickup,
		Reason:        &staffReason,
		Source:        scheduleModels.ExceptionSourceStaff,
		CreatedBy:     f.staffID,
	}))

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.ErrorIs(t, err, schedule.ErrPickupChangeConflict)

	kept, findErr := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, findErr)
	require.NotNil(t, kept)
	require.NotNil(t, kept.PickupTime)
	assert.Equal(t, "16:00", kept.PickupTime.Format("15:04"), "die Entscheidung der OGS bleibt stehen")
}

// TestPickupChangeApprovalYieldsToExcusedAbsence: a partial excusal already
// states when the child leaves. A pickup change on the same day would contradict
// it, so the approval has to fail instead of producing two answers.
func TestPickupChangeApprovalYieldsToExcusedAbsence(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(4)
	req := seedPickupChangeRequest(t, f, date)

	// Source stays "guardian" on purpose: that isolates the excusal rule from
	// the staff-source rule, which the test above already covers.
	excusedFrom := time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC)
	excusedReason := "Entschuldigt ab 13 Uhr"
	require.NoError(t, f.repos.StudentPickupException.Create(ctx, &scheduleModels.StudentPickupException{
		TenantModel:       modelBase.TenantModel{TenantID: f.chain.TenantID},
		StudentID:         f.chain.StudentID,
		ExceptionDate:     date,
		PickupTime:        &excusedFrom,
		ExcusedFrom:       &excusedFrom,
		ExcusedReason:     &excusedReason,
		ExcusedCreatedBy:  &f.staffID,
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &f.chain.AccountID,
	}))

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.ErrorIs(t, err, schedule.ErrPickupChangeConflict)
}

// TestWithdrawPickupChangeRequestClosesIt: a parent may take a request back
// while it is still pending, and it must not stay in the staff queue afterwards.
func TestWithdrawPickupChangeRequestClosesIt(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(5)
	req := seedPickupChangeRequest(t, f, date)

	withdrawn, err := f.svc.WithdrawPickupChangeRequest(ctx, req.ID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusWithdrawn, withdrawn.Status)

	pending, err := f.svc.ListPendingPickupChanges(ctx)
	require.NoError(t, err)
	for _, item := range pending {
		assert.NotEqual(t, req.ID, item.Request.ID, "eine zurueckgezogene Anfrage bleibt nicht in der Liste")
	}

	applied, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	assert.Nil(t, applied, "eine zurueckgezogene Anfrage aendert keine Abholzeit")
}

// TestListPickupChangeRequestsReturnsTheParentsOwn covers the read the parents
// app uses for its status list.
func TestListPickupChangeRequestsReturnsTheParentsOwn(t *testing.T) {
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(6)
	req := seedPickupChangeRequest(t, f, date)

	rows, err := f.svc.ListPickupChangeRequests(ctx, f.chain.StudentID, time.Now().AddDate(0, -2, 0))
	require.NoError(t, err)

	found := false
	for _, row := range rows {
		if row.ID == req.ID {
			found = true
			assert.Equal(t, scheduleModels.CareRequestStatusPending, row.Status)
		}
	}
	assert.True(t, found, "die eigene Anfrage muss in der Liste stehen")
}
