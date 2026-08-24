package users_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func createWithdrawalCompletion(
	t *testing.T,
	db *bun.DB,
	studentID, actorID int64,
	firstGap timezone.Date,
) *userModels.CareWithdrawalCompletion {
	t.Helper()
	row := &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actorID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}
	require.NoError(t, repositories.NewFactory(db).CareWithdrawal.UpsertPending(testpkg.Ctx(t), row))
	return row
}

func TestCareWithdrawalLifecycle_AllowsRetroactiveExitButNotBeforeAttendance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Lina", "Rueckwirkend", "2a")
	today := timezone.TodayDate()
	_, err := db.NewUpdate().TableExpr("users.students").
		Set("enrolled_from = ?", today.AddDays(-10)).
		Set("status = ?", userModels.StudentStatusActive).
		Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Erika", "Betreuung")
	device := testpkg.CreateTestDevice(t, db, "retro-withdrawal-device")
	attendanceInstant := today.AddDays(-2).BerlinMidnight().Add(8 * time.Hour)
	checkedOut := attendanceInstant.Add(time.Hour)
	attendance := testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, attendanceInstant, &checkedOut)
	_, err = db.NewUpdate().TableExpr("active.attendance").Set("date = ?", today.AddDays(-2)).Where("id = ?", attendance.ID).Exec(ctx)
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, "Anwesenheitsraum")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Anwesenheitsgruppe")
	instance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(-1), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	checkedInAt := today.AddDays(-1).BerlinMidnight().Add(9 * time.Hour)
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID,
		scheduleModels.AttendanceStatusPresent, testpkg.InstanceStudentOpts{CheckedInAt: &checkedInAt})
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, today)

	_, err = svc.PreviewWithdrawalCareEnd(ctx, completion.ID, userService.CareExitInput{
		LastCareDay: today.AddDays(-2), Reason: userModels.CareExitReasonNoCareNeed,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), today.AddDays(-1).Format("02.01.2006"))

	input := userService.CareExitInput{LastCareDay: today.AddDays(-1), Reason: userModels.CareExitReasonNoCareNeed}
	preview, err := svc.PreviewWithdrawalCareEnd(ctx, completion.ID, input)
	require.NoError(t, err)
	_, err = svc.ConfirmWithdrawalCareEnd(ctx, completion.ID, preview.Token, input, actorID)
	require.NoError(t, err)
	stored := loadStudent(t, db, ctx, student.ID)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, today.AddDays(-1), *stored.EnrolledUntil)
}

func TestCareWithdrawalLifecycle_ConcurrentCompletionWritesOneResult(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Mika", "Parallel", "3a")
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate().AddDays(1))
	input := userService.CareExitInput{LastCareDay: timezone.TodayDate(), Reason: userModels.CareExitReasonNoCareNeed}
	preview, err := svc.PreviewWithdrawalCareEnd(ctx, completion.ID, input)
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			err := tenant.WithTenantTx(context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				_, confirmErr := svc.ConfirmWithdrawalCareEnd(txCtx, completion.ID, preview.Token, input, actorID)
				return confirmErr
			})
			errs[index] = err
		}(i)
	}
	close(start)
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, userModels.ErrCareWithdrawalAlreadyResolved):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, conflicts)
}

func TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	bookingGateCalls := 0
	svc := newCareLifecycleServiceWithLock(t, db, func(context.Context) error {
		bookingGateCalls++
		return nil
	})
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Nele", "Storno", "2b")
	firstGap := timezone.TodayDate().AddDays(2)
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, firstGap)
	input := userService.CareExitInput{
		LastCareDay: firstGap.AddDays(-1), Reason: userModels.CareExitReasonNoCareNeed,
	}
	preview, err := svc.PreviewWithdrawalCareEnd(ctx, completion.ID, input)
	require.NoError(t, err)
	_, err = svc.ConfirmWithdrawalCareEnd(ctx, completion.ID, preview.Token, input, actorID)
	require.NoError(t, err)
	bookingGateCalls = 0

	rows, _, err := svc.ListPendingWithdrawals(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: student.ID})
	require.NoError(t, err)
	assert.Empty(t, rows)

	_, err = svc.Cancel(ctx, []int64{student.ID}, actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, bookingGateCalls, "cancellation restores source bookings under the shared booking gate")
	rows, _, err = svc.ListPendingWithdrawals(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: student.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	pending := rows[0]
	require.NotNil(t, pending)
	assert.NotEqual(t, completion.ID, pending.ID)
	assert.Equal(t, firstGap, pending.FirstBookinglessDay)
}

func TestCareWithdrawalLifecycle_CancellingLaterOrdinaryExitDoesNotRestoreOldTask(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Nele", "Neuer Austritt", "2b")
	firstGap := timezone.TodayDate().AddDays(2)
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, firstGap)
	withdrawal := userService.CareExitInput{
		LastCareDay: firstGap.AddDays(-1), Reason: userModels.CareExitReasonNoCareNeed,
	}
	preview, err := svc.PreviewWithdrawalCareEnd(ctx, completion.ID, withdrawal)
	require.NoError(t, err)
	_, err = svc.ConfirmWithdrawalCareEnd(ctx, completion.ID, preview.Token, withdrawal, actorID)
	require.NoError(t, err)

	later := userService.CareExitInput{
		StudentIDs: []int64{student.ID}, LastCareDay: firstGap.AddDays(7),
		Reason: userModels.CareExitReasonMovedAway,
	}
	laterPreview, err := svc.Preview(ctx, later)
	require.NoError(t, err)
	_, err = svc.Confirm(ctx, laterPreview.Token, later, actorID)
	require.NoError(t, err)
	_, err = svc.Cancel(ctx, []int64{student.ID}, actorID)
	require.NoError(t, err)

	rows, _, err := svc.ListPendingWithdrawals(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: student.ID})
	require.NoError(t, err)
	assert.Empty(t, rows, "an unrelated later exit must clear the old withdrawal provenance")
}
