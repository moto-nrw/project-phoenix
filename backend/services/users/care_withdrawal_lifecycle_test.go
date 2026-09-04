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
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	userService "github.com/moto-nrw/project-phoenix/services/users"
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
	today := timezone.NewDate(2026, 8, 24)
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

func TestCareWithdrawalLifecycle_CompletionEndsBookingsFromEveryEnrollmentRequest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	scope := testpkg.TenantScope{TenantID: testpkg.Tenant(t)}
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Alle", "Buchungen", "2c")
	createCareBooking(t, db, scope, student.ID, "first-request", nil, nil)
	createCareBooking(t, db, scope, student.ID, "second-request", nil, nil)

	var requestChildIDs []int64
	err := db.NewSelect().TableExpr("enrollment.request_children").
		Column("id").Where("created_student_id = ?", student.ID).Order("id ASC").Scan(ctx, &requestChildIDs)
	require.NoError(t, err)
	require.Len(t, requestChildIDs, 2)
	studentID := student.ID
	firstGap := timezone.NewDate(2026, 8, 24).AddDays(1)
	completion := &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
		SourceRequestChildID:  &requestChildIDs[0],
		WithdrawalConfirmedBy: &actorID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}
	require.NoError(t, repositories.NewFactory(db).CareWithdrawal.UpsertPending(ctx, completion))

	input := userService.CareExitInput{
		LastCareDay: timezone.NewDate(2026, 8, 24), Reason: userModels.CareExitReasonNoCareNeed,
	}
	svc := newCareLifecycleService(t, db)
	preview, err := svc.PreviewWithdrawalCareEnd(ctx, completion.ID, input)
	require.NoError(t, err)
	_, err = svc.ConfirmWithdrawalCareEnd(ctx, completion.ID, preview.Token, input, actorID)
	require.NoError(t, err)

	assert.Equal(t, []timezone.Date{firstGap, firstGap}, bookingEndDates(t, db, ctx, student.ID))
}

func bookingEndDates(t *testing.T, db *bun.DB, ctx context.Context, studentID int64) []timezone.Date {
	t.Helper()
	var rows []struct {
		ValidUntil timezone.Date `bun:"valid_until"`
	}
	err := db.NewSelect().TableExpr("enrollment.request_child_offerings AS rco").
		Column("rco.valid_until").
		Join("JOIN enrollment.request_children AS rc ON rc.id = rco.request_child_id").
		Where("rc.created_student_id = ?", studentID).Order("rco.id ASC").Scan(ctx, &rows)
	require.NoError(t, err)
	result := make([]timezone.Date, len(rows))
	for i, row := range rows {
		result[i] = row.ValidUntil
	}
	return result
}

func TestCareWithdrawalLifecycle_DeletesStudentAndRedactsCompletionAtomically(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Lina", "Loeschung", "2a")
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate())
	deletion := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	svc := newCareLifecycleServiceWithDeletion(t, db, deletion)

	preview, err := svc.PreviewWithdrawalDeletion(ctx, completion.ID)
	require.NoError(t, err)
	input := userService.StudentDeletionInput{
		ActorAccountID:      actorID,
		ExpectedFingerprint: "stale",
		ConfirmationName:    preview.ConfirmationName,
		Reason:              userService.StudentDeletionReasonPrivacyRequest,
		Acknowledged:        true,
	}
	_, err = svc.DeleteWithdrawal(ctx, completion.ID, input)
	require.ErrorIs(t, err, userService.ErrStudentDeletionPreviewChanged)
	stillPending, err := repos.CareWithdrawal.FindByID(ctx, completion.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStatePending, stillPending.State)

	input.ExpectedFingerprint = preview.Fingerprint
	result, err := svc.DeleteWithdrawal(ctx, completion.ID, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	redacted, err := repos.CareWithdrawal.FindByID(ctx, completion.ID)
	require.NoError(t, err)
	require.NotNil(t, redacted)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redacted.State)
	require.NotNil(t, redacted.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeDeleted, *redacted.Outcome)
	assert.Nil(t, redacted.StudentID)
	assert.Nil(t, redacted.SourceAdjustmentID)
	assert.Nil(t, redacted.SourceRequestChildID)
	assert.Empty(t, redacted.SourceOfferings)
}

func TestStudentDeletion_RedactsPendingWithdrawalOutsideCompletionFlow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Noah", "Direktloeschung", "3b")
	resolved := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate())
	resolvedAt := time.Now().Add(-2 * time.Hour)
	changed, err := repos.CareWithdrawal.MarkResolved(ctx, resolved.ID, actorID, resolvedAt)
	require.NoError(t, err)
	require.True(t, changed)

	obsolete := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate().AddDays(1))
	obsoleteAt := time.Now().Add(-time.Hour)
	changed, err = repos.CareWithdrawal.MarkObsoleteForRebooking(
		ctx, student.ID, timezone.TodayDate(), obsoleteAt,
	)
	require.NoError(t, err)
	require.True(t, changed)

	pending := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate().AddDays(2))
	deletion := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	userService.WireStudentDeletionCareWithdrawals(deletion, repos.CareWithdrawal)

	preview, err := deletion.Preview(ctx, student.ID)
	require.NoError(t, err)
	_, err = deletion.Delete(ctx, userService.StudentDeletionInput{
		StudentID: student.ID, ActorAccountID: actorID,
		ExpectedFingerprint: preview.Fingerprint, ConfirmationName: preview.ConfirmationName,
		Reason: userService.StudentDeletionReasonPrivacyRequest, Acknowledged: true,
	})
	require.NoError(t, err)
	redacted, err := repos.CareWithdrawal.FindByID(ctx, pending.ID)
	require.NoError(t, err)
	require.NotNil(t, redacted)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redacted.State)
	require.NotNil(t, redacted.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeDeleted, *redacted.Outcome)
	assert.Nil(t, redacted.StudentID)
	assert.Nil(t, redacted.SourceRequestChildID)
	assert.Empty(t, redacted.SourceOfferings)

	redactedResolved, err := repos.CareWithdrawal.FindByID(ctx, resolved.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redactedResolved.State)
	require.NotNil(t, redactedResolved.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeCareEnded, *redactedResolved.Outcome)
	assert.Equal(t, resolvedAt.Unix(), redactedResolved.ResolvedAt.Unix())
	assert.Nil(t, redactedResolved.StudentID)

	redactedObsolete, err := repos.CareWithdrawal.FindByID(ctx, obsolete.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateObsolete, redactedObsolete.State)
	assert.Nil(t, redactedObsolete.Outcome)
	require.NotNil(t, redactedObsolete.ObsoleteReason)
	assert.Equal(t, userModels.CareWithdrawalObsoleteRebooked, *redactedObsolete.ObsoleteReason)
	assert.Equal(t, obsoleteAt.Unix(), redactedObsolete.ResolvedAt.Unix())
	assert.Nil(t, redactedObsolete.StudentID)
}

func newCareLifecycleServiceWithDeletion(
	t *testing.T,
	db *bun.DB,
	deletion userService.StudentDeletionService,
) userService.CareLifecycleService {
	t.Helper()
	repos := repositories.NewFactory(db)
	repos.BindTimetable(timetabletest.New(t, db))
	return userService.NewCareLifecycleService(userService.CareLifecycleDependencies{
		StudentRepo: repos.Student, PersonRepo: repos.Person,
		CareExitRepo: repos.CareExit, CleanupRepo: repos.CareExitCleanup,
		WithdrawalRepo: repos.CareWithdrawal, TagReleaser: repos.GradeTransition,
		AuditService:    userService.NewStudentAuditService(repos.StudentFieldEdit, nil),
		StudentDeletion: deletion,
		DB:              db,
	})
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
			err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
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

func TestCareWithdrawalLifecycle_ResolvedCompletionIsAConflict(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Mila", "Erledigt", "3a")
	completion := createWithdrawalCompletion(t, db, student.ID, actorID, timezone.TodayDate())
	changed, err := repositories.NewFactory(db).CareWithdrawal.MarkResolved(ctx, completion.ID, actorID, time.Now())
	require.NoError(t, err)
	require.True(t, changed)

	_, err = svc.GetPendingWithdrawal(ctx, completion.ID)
	require.ErrorIs(t, err, userModels.ErrCareWithdrawalAlreadyResolved)
}

func TestCareWithdrawalLifecycle_CancellingPlannedExitRestoresTask(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	bookingGateCalls := 0
	today := timezone.NewDate(2026, 8, 24)
	svc := newCareLifecycleServiceWithLockAt(t, db, func(context.Context) error {
		bookingGateCalls++
		return nil
	}, func() timezone.Date { return today })
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Nele", "Storno", "2b")
	firstGap := today.AddDays(2)
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
