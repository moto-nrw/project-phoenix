package schedule_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// nextSchoolDay moves a date off the weekend. A pickup schedule only knows
// Monday to Friday, so a plain "today + 2" made this test fail every Thursday
// and Friday — the calendar, not the code, decided whether it was green
// (#2419).
func nextSchoolDay(d timezone.Date) timezone.Date {
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDays(1)
	}
	return d
}

func TestPickupChangeRequestAppliesOnlyAfterStaffApproval(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := nextSchoolDay(timezone.TodayDate().AddDays(2))
	pickup := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)
	weekday := int(date.Weekday())
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
	require.NotNil(t, items[0].Reason)
	assert.Equal(t, "Arzttermin", *items[0].Reason)

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

func TestPickupChangeApprovalExcusesBlocksAfterEarlierPickup(t *testing.T) {
	t.Parallel()

	f, date, slotID := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)

	req, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC),
		"Arzttermin",
	)
	require.NoError(t, err)
	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	got, err := f.repos.InstanceStudent.FindByID(ctx, slotID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status)
	require.NotNil(t, got.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *got.Substatus)
	require.NotNil(t, got.PickupExceptionID)

	exception, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, exception)
	assert.True(t, exception.ExcusedAuto)
}

func TestPendingPickupChangeNamesBlocksRemovedByApproval(t *testing.T) {
	t.Parallel()

	f, date, _ := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	_, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC),
		"Arzttermin",
	)
	require.NoError(t, err)

	items, _, err := f.svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].AffectedBlocks, 1)
	assert.Equal(t, "Nachmittags-AG", items[0].AffectedBlocks[0].Title)
	assert.Equal(t, "15:00", items[0].AffectedBlocks[0].StartTime.Format("15:04"))
	assert.Equal(t, "16:00", items[0].AffectedBlocks[0].EndTime.Format("15:04"))
}

func TestPendingPickupChangeMarksUnavailableImpact(t *testing.T) {
	t.Parallel()
	f, date, _ := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	req := seedPickupChangeRequest(t, f, date)
	req.Payload["pickup_time"] = "bad"
	_, err := f.db.NewUpdate().Model(req).Column("payload").WherePK().Exec(ctx)
	require.NoError(t, err)

	items, _, err := f.svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.False(t, items[0].ImpactAvailable)
	assert.Empty(t, items[0].AffectedBlocks)
	assert.Empty(t, items[0].ImpactToken)
}

func TestPickupApprovalRejectsStaleAffectedBlockList(t *testing.T) {
	t.Parallel()
	f, date, _ := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	req := seedPickupChangeRequest(t, f, date)
	items, _, err := f.svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	expected := items[0].ImpactToken

	room := testpkg.CreateTestRoom(t, f.db, "Changed pickup impact")
	block := testpkg.CreateTestActivityInstance(t, f.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:30", EndHHMM: "16:30", Title: "Späte AG",
	})
	testpkg.CreateTestInstanceStudent(t, f.db, block.ID, f.chain.StudentID, scheduleModels.AttendanceStatusExpected)

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
		ExpectedImpactToken: &expected,
	})
	require.ErrorIs(t, err, schedule.ErrPickupChangeImpactChanged)
	kept, findErr := f.repos.CareScheduleChangeRequest.FindByID(ctx, req.ID)
	require.NoError(t, findErr)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, kept.Status)
}

func TestPickupApprovalRequiresImpactTokenForHTTPDecision(t *testing.T) {
	t.Parallel()
	f, date, _ := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	req := seedPickupChangeRequest(t, f, date)

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
		RequireImpactToken: true,
	})
	require.ErrorIs(t, err, schedule.ErrPickupChangeImpactRequired)
	kept, findErr := f.repos.CareScheduleChangeRequest.FindByID(ctx, req.ID)
	require.NoError(t, findErr)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, kept.Status)
}

func TestPickupChangeApprovalReleasesAutoExcusalAfterLaterPickup(t *testing.T) {
	t.Parallel()

	f, date, slotID := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	earlierPickup := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)
	f.seedGuardianPickupAutoExcusal(t, date, earlierPickup)

	req, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 16, 30, 0, 0, time.UTC),
		"Späterer Termin",
	)
	require.NoError(t, err)
	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)

	got, err := f.repos.InstanceStudent.FindByID(ctx, slotID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status)
	assert.Nil(t, got.PickupExceptionID)

	exception, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, exception)
	assert.False(t, exception.ExcusedAuto)
	assert.Nil(t, exception.ExcusedFrom)
}

func TestPickupChangeApprovalRollsBackWhenAutoExcusalFails(t *testing.T) {
	t.Parallel()

	f, date, slotID := newPickupApprovalAutoExcusalScenario(t)
	ctx := f.staffCtx(f.staffAccount)
	req, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC),
		"Arzttermin",
	)
	require.NoError(t, err)

	releaseSlot := holdInstanceStudentLock(t, f, slotID)
	err = tenant.WithTenantTx(ctx, f.db, f.chain.TenantID, func(txCtx context.Context, tx bun.Tx) error {
		if _, timeoutErr := tx.ExecContext(txCtx, `SET LOCAL lock_timeout = '100ms'`); timeoutErr != nil {
			return timeoutErr
		}
		_, decideErr := f.svc.Decide(txCtx, schedule.CareRequestDecideInput{
			RequestID:  req.ID,
			Approve:    true,
			ReviewedBy: f.staffAccount,
		})
		return decideErr
	})
	require.ErrorContains(t, err, "sync approved pickup exception")
	releaseSlot()
	assertPickupApprovalRolledBack(t, f, ctx, date, slotID, req.ID)
}

func assertPickupApprovalRolledBack(
	t *testing.T, f *careFixture, ctx context.Context, date timezone.Date, slotID, requestID int64,
) {
	t.Helper()
	exception, err := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, err)
	assert.Nil(t, exception, "the exception write must roll back with the failed sync")
	request, err := f.repos.CareScheduleChangeRequest.FindByID(ctx, requestID)
	require.NoError(t, err)
	require.NotNil(t, request)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, request.Status)
	slot, err := f.repos.InstanceStudent.FindByID(ctx, slotID)
	require.NoError(t, err)
	require.NotNil(t, slot)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, slot.Status)
}

func holdInstanceStudentLock(t *testing.T, f *careFixture, slotID int64) func() {
	t.Helper()
	ready := make(chan error, 1)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		tx, err := f.db.BeginTx(context.Background(), nil)
		if err != nil {
			ready <- err
			done <- err
			return
		}
		var lockedID int64
		err = tx.NewRaw(
			`SELECT id FROM schedule.instance_students WHERE id = ? AND tenant_id = ? FOR UPDATE`,
			slotID,
			f.chain.TenantID,
		).Scan(context.Background(), &lockedID)
		ready <- err
		if err == nil {
			<-release
		}
		err = errors.Join(err, tx.Rollback())
		done <- err
	}()
	require.NoError(t, <-ready)
	var once sync.Once
	unlock := func() {
		once.Do(func() {
			close(release)
			require.NoError(t, <-done)
		})
	}
	t.Cleanup(unlock)
	return unlock
}

func newPickupApprovalAutoExcusalScenario(t *testing.T) (*careFixture, timezone.Date, int64) {
	t.Helper()
	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := nextSchoolDay(timezone.TodayDate().AddDays(2))
	require.NoError(t, f.sf.PickupSchedule.UpsertStudentPickupSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  f.chain.StudentID,
		Weekday:    int(date.Weekday()),
		PickupTime: timezone.WallClock(time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)),
		CreatedBy:  f.staffID,
	}))
	room := testpkg.CreateTestRoom(t, f.db, "Pickup request auto excusal")
	block := testpkg.CreateTestActivityInstance(t, f.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00",
		EndHHMM:   "16:00",
		Title:     "Nachmittags-AG",
	})
	slot := testpkg.CreateTestInstanceStudent(
		t,
		f.db,
		block.ID,
		f.chain.StudentID,
		scheduleModels.AttendanceStatusExpected,
	)
	return f, date, slot.ID
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
	t.Parallel()

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
	t.Parallel()

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

func TestPickupChangeApprovalRejectsCompletedSameDayPickup(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate()
	req, err := f.svc.CreatePickupChangeRequest(
		ctx,
		f.chain.StudentID,
		f.chain.AccountID,
		date,
		time.Date(2000, 1, 1, 16, 30, 0, 0, time.UTC),
		"Späterer Termin",
	)
	require.NoError(t, err)
	device := testpkg.CreateTestDeviceForTenant(t, f.db, f.chain.TenantID, "pickup-review-checkout")
	checkedOutAt := timezone.Now()
	attendance := &activeModels.Attendance{
		StudentID:    f.chain.StudentID,
		Date:         date,
		CheckInTime:  checkedOutAt.Add(-time.Hour),
		CheckOutTime: &checkedOutAt,
		DeviceID:     device.ID,
	}
	attendance.SetTenantID(f.chain.TenantID)
	require.NoError(t, f.repos.Attendance.Create(ctx, attendance))

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})

	require.ErrorIs(t, err, schedule.ErrPickupChangeAlreadyCompleted)
}

func TestPickupChangeApprovalRejectsDateAfterPlannedCareEnd(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	date := timezone.TodayDate().AddDays(4)
	req := seedPickupChangeRequest(t, f, date)
	lastCareDay := date.AddDays(-1)
	_, err := f.db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", lastCareDay).
		Where("id = ?", f.chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	require.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound)

	exception, findErr := f.repos.StudentPickupException.FindByStudentIDAndDate(ctx, f.chain.StudentID, date)
	require.NoError(t, findErr)
	assert.Nil(t, exception, "approval must not create a pickup exception after care ends")
}

// TestPickupChangeRequestsForDifferentDaysCoexist: pending uniqueness for
// pickup changes is per requested DAY, not per child — an Arzttermin on
// Tuesday must not block an independent request for Wednesday. Only a second
// request for the SAME day collides, and an open weekly-schedule request
// coexists with open pickup requests (separate partial unique indexes).
func TestPickupChangeRequestsForDifferentDaysCoexist(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	tuesday := timezone.TodayDate().AddDays(7)
	wednesday := tuesday.AddDays(1)

	first := seedPickupChangeRequest(t, f, tuesday)

	second, err := f.svc.CreatePickupChangeRequest(
		ctx, f.chain.StudentID, f.chain.AccountID, wednesday,
		time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC), "Früherer Termin",
	)
	require.NoError(t, err, "ein Antrag für einen anderen Tag darf nicht blockiert werden")
	assert.NotEqual(t, first.ID, second.ID)

	_, err = f.svc.CreatePickupChangeRequest(
		ctx, f.chain.StudentID, f.chain.AccountID, tuesday,
		time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC), "Doppelt",
	)
	require.ErrorIs(t, err, schedule.ErrCareRequestAlreadyPending)

	_, err = f.svc.CreateRequest(ctx, f.chain.StudentID, f.chain.AccountID,
		careWeekdays(map[string]any{"weekday": 1, "pickup": "15:00"}))
	require.NoError(t, err, "ein Wochenplan-Antrag koexistiert mit offenen Abholanträgen")

	pending, err := f.svc.ListPendingPickupChanges(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 2)
}

// TestPickupChangeApprovalRejectsExpiredRequest: a request whose day passed
// while it sat in the queue cannot be approved (the day is over), but staff
// must still be able to close it by rejecting.
func TestPickupChangeApprovalRejectsExpiredRequest(t *testing.T) {
	t.Parallel()

	f := newCareFixture(t)
	ctx := f.staffCtx(f.staffAccount)
	// CreatePickupChangeRequest refuses past dates, so an expired request can
	// only exist by aging in the queue — seed it directly at the repo.
	req := &scheduleModels.CareScheduleChangeRequest{
		StudentID:   f.chain.StudentID,
		SubmittedBy: f.chain.AccountID,
		RequestKind: scheduleModels.CareRequestKindPickupChange,
		Payload: map[string]any{
			"date":        timezone.TodayDate().AddDays(-1).String(),
			"pickup_time": "14:30",
			"reason":      "Arzttermin",
		},
		Status: scheduleModels.CareRequestStatusPending,
	}
	require.NoError(t, f.repos.CareScheduleChangeRequest.Create(ctx, req))

	_, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.ErrorIs(t, err, schedule.ErrPickupChangeExpired)

	decided, err := f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID:  req.ID,
		Approve:    false,
		Reason:     "Der Tag liegt in der Vergangenheit.",
		ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err, "eine abgelaufene Anfrage bleibt per Ablehnung abschließbar")
	assert.Equal(t, scheduleModels.CareRequestStatusRejected, decided.Request.Status)
}

// TestWithdrawPickupChangeRequestClosesIt: a parent may take a request back
// while it is still pending, and it must not stay in the staff queue afterwards.
func TestWithdrawPickupChangeRequestClosesIt(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
