// "Betreuung beenden" (#2487): the regular exit of a child from the OGS.
//
// The invariants these tests hold: the last care day is INCLUSIVE, the
// confirmation is all-or-nothing and only valid for the exact state the
// preview described, another tenant's child is invisible, and everything that
// happened before the exit stays untouched.
package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// careActor is the acting account. It has to be a real one: the exit record
// carries a foreign key to auth.accounts, exactly so a reason can always be
// traced back to a person.
func careActor(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	return testpkg.CreateTestAccount(t, db, "care-actor").ID
}

func newCareLifecycleService(t *testing.T, db *bun.DB) careplan.CareLifecycle {
	return newCareLifecycleServiceWithLockAt(t, db, nil, nil)
}

func newCareLifecycleServiceAt(t *testing.T, db *bun.DB, today calendar.Date) careplan.CareLifecycle {
	return newCareLifecycleServiceWithLockAt(t, db, nil, func() calendar.Date { return today })
}

func newCareLifecycleServiceWithLockAt(
	t *testing.T,
	db *bun.DB,
	lockCareBookingWrites func(context.Context) error,
	today func() calendar.Date,
) careplan.CareLifecycle {
	t.Helper()
	// RFID tag release runs through the People Directory composition (#2661).
	repos, err := api.NewCareLifecycleTestRepositories(testpkg.NewDatabase(t, db), nil)
	require.NoError(t, err)
	svc, err := repos.NewCareLifecycle(api.CareLifecycleTestConfig{
		Audit:                 api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(testpkg.NewDatabase(t, db))),
		LockCareBookingWrites: lockCareBookingWrites,
		BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil },
		Today:                 today,
	})
	require.NoError(t, err)
	return svc
}

// carePlanRecords is the Care Plan record capability the suites stage and
// read exit reasons through.
func carePlanRecords(db *bun.DB) careplan.Capability {
	return api.NewRepositoryTestFactory(testpkg.MustDatabase(db)).CarePlan()
}

// endCare runs the full product path — preview, then confirm the token it
// handed out — because that is the only way the action is reachable.
func endCare(
	t *testing.T,
	ctx context.Context,
	svc careplan.CareLifecycle,
	actorID int64,
	input careplan.CareExitInput,
) *careplan.CareExitResult {
	t.Helper()
	preview, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.Preview, input)
	require.NoError(t, err)
	require.False(t, preview.Blocked, "preview must not be blocked: %+v", preview.Students)
	result, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), svc.Confirm, preview.Token, input, actorID)
	require.NoError(t, err)
	return result
}

func TestCareLifecycle_ConfirmAuditsOnlyCareEnd(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Audit", "CareEnd", "2a")

	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("status = ?", string(peopledirectory.StudentStatusActive)).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().
		TableExpr("users.student_profiles").
		Set("extra_info = ?", "Nicht ändern").
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().
		TableExpr("users.student_care_profiles").
		Set("supervisor_notes = ?", "Nicht ändern").
		Set("health_info = ?", "Nicht ändern").
		Set("pickup_status = ?", "pickup").
		Where("membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: calendar.TodayDate(),
		Reason:      careplan.CareExitReasonMovedAway,
	})

	edits, err := testpkg.TenantQuery1(t, ctx, db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentFieldEdit.GetByStudentID, student.ID)
	require.NoError(t, err)
	require.Len(t, edits, 1)
	assert.Equal(t, peopledirectory.StudentFieldCareEnd, edits[0].FieldName)
}

func loadStudent(t *testing.T, db *bun.DB, ctx context.Context, id int64) *peopledirectory.StudentState {
	t.Helper()
	student, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student.FindByID, any(id))
	require.NoError(t, err)
	return student
}

func TestCareLifecycle_LastCareDayIsInclusive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2026, 8, 24)
	svc := newCareLifecycleServiceAt(t, db, today)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Lina", "Bergmann", "2a")
	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      careplan.CareExitReasonMovedAway,
	})

	stored := loadStudent(t, db, ctx, student.ID)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, today, *stored.EnrolledUntil,
		"the last care day is stored verbatim as the interval's upper bound")

	assert.False(t, stored.CareEndedOn(today),
		"the child is still in care ON their last care day")
	assert.True(t, stored.CareEndedOn(today.AddDays(1)),
		"the child is out from the day after")
}

func TestCareLifecycle_RefusesRetroactiveExit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)

	student := testpkg.CreateTestStudent(t, db, "Jonas", "Bergmann", "2a")

	_, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: calendar.TodayDate().AddDays(-1),
		Reason:      careplan.CareExitReasonNoCareNeed,
	})
	require.ErrorIs(t, err, careplan.ErrCareExitDayInPast)
}

func TestCareLifecycle_ReasonNotePairing(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)

	student := testpkg.CreateTestStudent(t, db, "Mila", "Bergmann", "2a")
	today := calendar.TodayDate()

	t.Run("other requires a note", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      careplan.CareExitReasonOther,
		})
		require.ErrorIs(t, err, careplan.ErrCareExitNoteRequired)
	})

	t.Run("a categorised reason refuses a note", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      careplan.CareExitReasonMovedAway,
			ReasonNote:  "steht nicht zur Wahl",
		})
		require.ErrorIs(t, err, careplan.ErrCareExitNoteNotAllowed)
	})

	t.Run("an unknown reason is refused", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      "weggezogen",
		})
		require.ErrorIs(t, err, careplan.ErrCareExitInvalidReason)
	})
}

func TestCareLifecycle_ConfirmRefusesStalePreview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	first := testpkg.CreateTestStudent(t, db, "Ida", "Kramer", "3a")
	second := testpkg.CreateTestStudent(t, db, "Tom", "Kramer", "3a")
	today := calendar.TodayDate()
	input := careplan.CareExitInput{
		StudentIDs:  []int64{first.ID, second.ID},
		LastCareDay: today,
		Reason:      careplan.CareExitReasonNoCareNeed,
	}

	preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, input)
	require.NoError(t, err)

	// Somebody edits one of the children between preview and confirmation.
	_, err = db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("school_class = ?", "3b").
		Set("updated_at = ?", time.Now().Add(time.Second)).
		Where("student_profile_id = ? AND deleted_at IS NULL", second.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = testpkg.TenantQuery3(t, ctx, db, svc.Confirm, preview.Token, input, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitPreviewChanged)

	// Nothing at all was written — not even for the untouched child.
	for _, id := range []int64{first.ID, second.ID} {
		stored := loadStudent(t, db, ctx, id)
		assert.Nil(t, stored.EnrolledUntil,
			"a refused confirmation must leave every child unchanged")
	}
}

func TestCareLifecycle_BlockedChildStopsTheWholeAction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	fine := testpkg.CreateTestStudent(t, db, "Nele", "Wirth", "1b")
	graduated := testpkg.CreateTestStudent(t, db, "Ben", "Wirth", "4b")
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("status = ?", string(peopledirectory.StudentStatusAlumnus)).
		Where("student_profile_id = ? AND deleted_at IS NULL", graduated.ID).
		Exec(context.Background())
	require.NoError(t, err)

	input := careplan.CareExitInput{
		StudentIDs:  []int64{fine.ID, graduated.ID},
		LastCareDay: calendar.TodayDate(),
		Reason:      careplan.CareExitReasonMovedAway,
	}

	preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, input)
	require.NoError(t, err)
	assert.True(t, preview.Blocked)

	blocked := map[int64]string{}
	for _, impact := range preview.Students {
		blocked[impact.StudentID] = impact.Blocker
	}
	assert.Empty(t, blocked[fine.ID], "the healthy child carries no blocker")
	assert.NotEmpty(t, blocked[graduated.ID], "the graduate is named with a reason")

	_, err = testpkg.TenantQuery3(t, ctx, db, svc.Confirm, preview.Token, input, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitBlocked)
	assert.Nil(t, loadStudent(t, db, ctx, fine.ID).EnrolledUntil,
		"one blocked child stops the whole action")
}

func TestCareLifecycle_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)

	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Fremd", "Kind", "1a")

	preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
		StudentIDs:  []int64{foreign.ID},
		LastCareDay: calendar.TodayDate(),
		Reason:      careplan.CareExitReasonMovedAway,
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	assert.NotEmpty(t, preview.Students[0].Blocker,
		"a child of another school is unknown here, not endable")

	stored, err := testpkg.TenantQuery1(t, tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID), db, api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student.FindByID, any(foreign.ID))
	require.NoError(t, err)
	assert.Nil(t, stored.EnrolledUntil)
}

func TestCareLifecycle_EndsBookingsAtTheLastCareDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	timetableRepos := testTimetableRepositories(db)

	student := testpkg.CreateTestStudent(t, db, "Ella", "Vogt", "2b")
	group := testpkg.CreateTestActivityGroup(t, db, "Fußball")
	today := calendar.TodayDate()

	booking := &timetable.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       string(today.AddDays(-30)),
	}
	booking.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).Create(ctx, booking))

	preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      careplan.CareExitReasonNoCareNeed,
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	assert.Equal(t, 1, preview.Students[0].ActivityBookings,
		"the preview names the booking before it ends it")

	result, err := testpkg.TenantQuery3(t, ctx, db, svc.Confirm, preview.Token, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      careplan.CareExitReasonNoCareNeed,
	}, actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.BookingsEnded)

	// valid_until is an EXCLUSIVE bound, so the booking still counts on the
	// last care day and stops the day after.
	stillToday, err := timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).FindActiveByStudentIDs(ctx, []int64{student.ID}, string(today))
	require.NoError(t, err)
	assert.Len(t, stillToday, 1, "the booking still counts on the last care day")

	tomorrow, err := timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).FindActiveByStudentIDs(ctx, []int64{student.ID}, string(today.AddDays(1)))
	require.NoError(t, err)
	assert.Empty(t, tomorrow, "the booking has ended the day after")
}

func TestCareLifecycle_CancelOnlyBeforeItTakesEffect(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Ruben", "Hesse", "3c")
	future := calendar.TodayDate().AddDays(14)

	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: future,
		Reason:      careplan.CareExitReasonOther,
		ReasonNote:  "Wechsel in den Hort",
	})

	cancelled, err := testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, cancelled)
	assert.Nil(t, loadStudent(t, db, ctx, student.ID).EnrolledUntil,
		"cancelling removes the planned end entirely")

	// The reason row goes with it — a cancelled exit has no reason to keep.
	reasons, err := carePlanRecords(db).FindCareExits(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, reasons)

	// A second cancel has nothing to withdraw.
	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitNotPlanned)
}

func TestCareLifecycle_CancelRefusesOrdinaryEnrollmentEnd(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Nora", "Hesse", "3c")
	ordinaryEnd := calendar.NewDate(2026, 8, 24).AddDays(14)
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", ordinaryEnd).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitNotPlanned)
	stored := loadStudent(t, db, ctx, student.ID)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, ordinaryEnd, *stored.EnrolledUntil)
}

func TestCareLifecycle_CancelRestoresPreviousEnrollmentEnd(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2026, 8, 24)
	svc := newCareLifecycleServiceAt(t, db, today)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Mara", "Hesse", "3c")
	previousEnd := today.AddDays(40)
	_, err := db.NewUpdate().TableExpr("users.student_school_memberships").Set("enrolled_until = ?", previousEnd).Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(ctx)
	require.NoError(t, err)
	endCare(t, ctx, svc, actorID, careplan.CareExitInput{StudentIDs: []int64{student.ID}, LastCareDay: today.AddDays(10), Reason: careplan.CareExitReasonMovedAway})
	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.NoError(t, err)
	stored := loadStudent(t, db, ctx, student.ID)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, previousEnd, *stored.EnrolledUntil)
}

func TestCareLifecycle_CancelRefusedAfterItTookEffect(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Fine", "Hesse", "3c")
	// Written directly: the product refuses a retroactive exit, and this test
	// is about the state the day after a legitimate one.
	yesterday := calendar.TodayDate().AddDays(-1)
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", yesterday).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(context.Background())
	require.NoError(t, err)
	require.NoError(t, carePlanRecords(db).UpsertCareExit(ctx, careplan.CareExit{
		StudentID:  student.ID,
		Reason:     careplan.CareExitReasonMovedAway,
		RecordedBy: &actorID,
	}))

	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitAlreadyEffective)
}

func TestCareLifecycle_Resume(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2026, 8, 24)
	svc := newCareLifecycleServiceAt(t, db, today)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Yara", "Lorenz", "1c")
	yesterday := today.AddDays(-1)
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", yesterday).
		Set("status = ?", string(peopledirectory.StudentStatusInactive)).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(context.Background())
	require.NoError(t, err)
	require.NoError(t, carePlanRecords(db).UpsertCareExit(ctx, careplan.CareExit{
		StudentID:  student.ID,
		Reason:     careplan.CareExitReasonMovedAway,
		RecordedBy: &actorID,
	}))

	t.Run("refuses without the explicit review", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       calendar.NewDate(2026, 8, 24),
			ActorAccountID: actorID,
			Checked:        false,
		})
		require.ErrorIs(t, err, careplan.ErrCareResumeNotChecked)
	})

	t.Run("refuses a start in the past", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       yesterday,
			ActorAccountID: actorID,
			Checked:        true,
		})
		require.ErrorIs(t, err, careplan.ErrCareResumeStartInPast)
	})

	t.Run("reopens the care from the new start", func(t *testing.T) {
		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       calendar.NewDate(2026, 8, 24),
			ActorAccountID: actorID,
			Checked:        true,
		}))

		stored := loadStudent(t, db, ctx, student.ID)
		assert.Nil(t, stored.EnrolledUntil, "the end of care is gone")
		require.NotNil(t, stored.EnrolledFrom)
		assert.Equal(t, calendar.NewDate(2026, 8, 24), *stored.EnrolledFrom)
		assert.Equal(t, peopledirectory.StudentStatusActive, stored.Status,
			"a child resumed for today is active right away")
	})

	t.Run("refuses a child whose care is running", func(t *testing.T) {
		running := testpkg.CreateTestStudent(t, db, "Noah", "Lorenz", "1c")
		err := testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
			StudentID:      running.ID,
			NewStart:       calendar.NewDate(2026, 8, 24),
			ActorAccountID: actorID,
			Checked:        true,
		})
		require.ErrorIs(t, err, careplan.ErrCareResumeNotEnded)
	})

	t.Run("refuses an enrollment that ended without a recorded care exit", func(t *testing.T) {
		naturalEnd := testpkg.CreateTestStudent(t, db, "Lina", "Lorenz", "1c")
		_, err := db.NewUpdate().
			TableExpr("users.student_school_memberships").
			Set("enrolled_until = ?", yesterday).
			Set("status = ?", string(peopledirectory.StudentStatusInactive)).
			Where("student_profile_id = ? AND deleted_at IS NULL", naturalEnd.ID).
			Exec(context.Background())
		require.NoError(t, err)

		err = testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
			StudentID:      naturalEnd.ID,
			NewStart:       calendar.NewDate(2026, 8, 24),
			ActorAccountID: actorID,
			Checked:        true,
		})
		require.ErrorIs(t, err, careplan.ErrCareResumeMissing)
	})
}

func TestCareLifecycle_ResumeForAFutureStartWaitsForTheScheduler(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Emil", "Roth", "2c")
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", calendar.TodayDate().AddDays(-3)).
		Set("status = ?", string(peopledirectory.StudentStatusInactive)).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(context.Background())
	require.NoError(t, err)
	require.NoError(t, carePlanRecords(db).UpsertCareExit(ctx, careplan.CareExit{
		StudentID:  student.ID,
		Reason:     careplan.CareExitReasonMovedAway,
		RecordedBy: &actorID,
	}))

	start := calendar.TodayDate().AddDays(10)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
		StudentID:      student.ID,
		NewStart:       start,
		ActorAccountID: actorID,
		Checked:        true,
	}))

	stored := loadStudent(t, db, ctx, student.ID)
	assert.Equal(t, peopledirectory.StudentStatusPending, stored.Status,
		"a future start waits for the activate-students tick, like any other")
}

func TestCareLifecycle_ArchiveHoldsEveryRegularlyEndedCare(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	// One ended by hand (with a recorded reason), one whose enrollment phase
	// simply ran out (no reason row at all), one still in care.
	manual := testpkg.CreateTestStudent(t, db, "Hanna", "Peters", "4a")
	phase := testpkg.CreateTestStudent(t, db, "Karl", "Peters", "4a")
	running := testpkg.CreateTestStudent(t, db, "Mia", "Peters", "4a")

	yesterday := calendar.TodayDate().AddDays(-1)
	for _, id := range []int64{manual.ID, phase.ID} {
		_, err := db.NewUpdate().
			TableExpr("users.student_school_memberships").
			Set("enrolled_until = ?", yesterday).
			Where("student_profile_id = ? AND deleted_at IS NULL", id).
			Exec(context.Background())
		require.NoError(t, err)
	}
	require.NoError(t, carePlanRecords(db).UpsertCareExit(ctx, careplan.CareExit{
		StudentID:  manual.ID,
		Reason:     careplan.CareExitReasonMovedAway,
		RecordedBy: ptrInt64(actorID),
	}))

	rows, total, err := testpkg.TenantQueryPair1(t, ctx, db, svc.ListEnded, careplan.EndedCareFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	byID := map[int64]careplan.EndedCare{}
	for _, row := range rows {
		byID[row.StudentID] = row
	}
	require.Contains(t, byID, manual.ID)
	require.Contains(t, byID, phase.ID)
	assert.NotContains(t, byID, running.ID, "a child still in care is not in the archive")

	require.NotNil(t, byID[manual.ID].Reason)
	assert.Equal(t, careplan.CareExitReasonMovedAway, *byID[manual.ID].Reason)
	assert.Nil(t, byID[phase.ID].Reason,
		"a care that ran out on its own has no recorded reason, and says so")
}

func ptrInt64(value int64) *int64 { return &value }

// A planned exit that has not taken effect can be CANCELLED — and a
// cancellation that leaves the child active with an emptied timetable and
// ended offerings is not one. The plan has to come back with the child
// (#2487).
func TestCareLifecycle_CancelPutsThePlanBack(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2030, 8, 26)
	svc := newCareLifecycleServiceAt(t, db, today)
	actorID := careActor(t, db)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	timetableRepos := testTimetableRepositories(db)

	student := testpkg.CreateTestStudent(t, db, "Jara", "Ohlsen", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Plan", "Verantwortlich")
	room := testpkg.CreateTestRoom(t, db, "Atelier")
	group := testpkg.CreateTestActivityGroup(t, db, "Theater")

	// A block after the planned exit, carrying a status somebody set by hand —
	// the case a rebuild-from-enrollments restore would silently flatten.
	manualAt := time.Now()
	instance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(30), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
	roster := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID,
		studentpresence.SessionAttendanceAbsent,
		testpkg.InstanceStudentOpts{ManualStatusAt: &manualAt})
	testpkg.CreateTestPickupSchedule(t, db, student.ID, 1, staff.ID, "15:30")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, 1, staff.ID, "08:00")
	pickupException := testpkg.CreateTestPickupException(
		t, db, student.ID, today.AddDays(30), staff.ID, "14:45", "Arzt",
	)
	testpkg.CreateTestArrivalException(
		t, db, student.ID, today.AddDays(30), staff.ID, "09:00", "Termin",
	)
	testpkg.UpdateSessionAttendance(t, ctx, db, roster.ID, map[string]any{"pickup_exception_id": pickupException.ID})
	roster.PickupExceptionID = &pickupException.ID

	// One open-ended booking (gets capped) and one starting after the exit
	// (gets deleted outright).
	openEnded := &timetable.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       string(today.AddDays(-30)),
	}
	openEnded.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).Create(ctx, openEnded))

	futureOnly := &timetable.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       string(today.AddDays(40)),
		Weekday:         testpkg.IntPtr(3),
	}
	futureOnly.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).Create(ctx, futureOnly))
	futureOnlyID := futureOnly.ID

	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today.AddDays(14),
		Reason:      careplan.CareExitReasonMovedAway,
	})

	// Sanity: the exit really did take the plan apart.
	rows, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

		instance.ID)

	require.NoError(t, err)
	require.Empty(t, rows, "the exit removed the roster row")
	pickupSchedules, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.Len(t, pickupSchedules, 1, "the weekly pickup plan remains until the exit takes effect")
	arrivalSchedules, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.Len(t, arrivalSchedules, 1, "the weekly arrival plan remains until the exit takes effect")

	cancelled, err := testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.NoError(t, err)
	require.Equal(t, 1, cancelled)

	t.Run("the roster row is back, with the hand-set status intact", func(t *testing.T) {
		restored, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

			instance.ID)

		require.NoError(t, err)
		require.Len(t, restored, 1)
		assert.Equal(t, student.ID, restored[0].StudentID)
		assert.Equal(t, studentpresence.SessionAttendanceAbsent, restored[0].Status,
			"a restore is the inverse of the deletion, not a rebuild from enrollments")
		assert.NotNil(t, restored[0].ManualStatusAt)
		require.NotNil(t, restored[0].PickupExceptionID)
		assert.Equal(t, pickupException.ID, *restored[0].PickupExceptionID,
			"the restored roster keeps its restored pickup exception")
	})

	t.Run("the weekly arrival and pickup plans are back", func(t *testing.T) {
		pickupSchedules, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, pickupSchedules, 1)
		arrivalSchedules, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, arrivalSchedules, 1)
		pickupExceptions, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupException.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, pickupExceptions, 1)
		arrivalExceptions, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentArrivalException.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, arrivalExceptions, 1)
	})

	t.Run("the capped booking is open-ended again", func(t *testing.T) {
		active, err := timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).FindActiveByStudentIDs(
			ctx, []int64{student.ID}, string(today.AddDays(200)))
		require.NoError(t, err)
		require.NotEmpty(t, active, "the offering runs on as if nothing happened")
	})

	t.Run("the deleted booking is back under its own id", func(t *testing.T) {
		restored, err := timetableCompose.NativeEnrollmentRecords(timetableRepos.StudentEnrollment).FindByID(ctx, futureOnlyID)
		require.NoError(t, err)
		require.NotNil(t, restored)
		assert.Equal(t, string(today.AddDays(40)), restored.ValidFrom)
		assert.Nil(t, restored.ValidUntil)
	})
}

// Moving a planned exit to a LATER day must apply to the untouched plan. The
// first run removed everything after June; if the second run only worked on
// what was left, July would stay empty and the change would quietly be a
// second cut rather than a correction (#2487).
func TestCareLifecycle_ChangingTheDayReplansFromTheBaseline(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	timetableRepos := testTimetableRepositories(db)

	student := testpkg.CreateTestStudent(t, db, "Tomke", "Ahrend", "1b")
	room := testpkg.CreateTestRoom(t, db, "Musikraum")
	group := testpkg.CreateTestActivityGroup(t, db, "Chor")
	today := calendar.TodayDate()

	early := testpkg.CreateTestActivityInstance(t, db, today.AddDays(10), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
	late := testpkg.CreateTestActivityInstance(t, db, today.AddDays(40), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
	testpkg.CreateTestInstanceStudent(t, db, early.ID, student.ID, studentpresence.SessionAttendanceExpected)
	testpkg.CreateTestInstanceStudent(t, db, late.ID, student.ID, studentpresence.SessionAttendanceExpected)

	// First decision: the child leaves in five days. Both blocks go.
	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today.AddDays(5),
		Reason:      careplan.CareExitReasonNoCareNeed,
	})
	rows, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

		early.ID)

	require.NoError(t, err)
	require.Empty(t, rows)

	// Correction: they stay until day 20 after all.
	newLastDay := today.AddDays(20)
	preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: newLastDay,
		Reason:      careplan.CareExitReasonNoCareNeed,
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	assert.Equal(t, 1, preview.Students[0].PlannedRosterRows,
		"the preview counts the baseline: only the block after the NEW day is lost")
	require.NotNil(t, preview.Students[0].PlannedEndsOn)

	_, err = testpkg.TenantQuery3(t, ctx, db, svc.Confirm, preview.Token, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: newLastDay,
		Reason:      careplan.CareExitReasonNoCareNeed,
	}, actorID)
	require.NoError(t, err)

	t.Run("the block inside the extended window is back", func(t *testing.T) {
		restored, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

			early.ID)

		require.NoError(t, err)
		require.Len(t, restored, 1)
		assert.Equal(t, student.ID, restored[0].StudentID)
	})

	t.Run("the block after the new day stays removed", func(t *testing.T) {
		gone, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

			late.ID)

		require.NoError(t, err)
		assert.Empty(t, gone)
	})
}

// A resume is not an undo: the criteria have the school check group,
// offerings, weekly plan and times themselves, so nothing the exit removed may
// switch itself back on (#2487).
func TestCareLifecycle_ResumeDoesNotBringThePlanBack(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	timetableRepos := testTimetableRepositories(db)

	student := testpkg.CreateTestStudent(t, db, "Malte", "Ruhnau", "2c")
	room := testpkg.CreateTestRoom(t, db, "Turnhalle")
	group := testpkg.CreateTestActivityGroup(t, db, "Turnen")
	today := calendar.TodayDate()

	instance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(30), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, studentpresence.SessionAttendanceExpected)

	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      careplan.CareExitReasonMovedAway,
	})

	// The exit takes effect, then the family comes back.
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", today.AddDays(-1)).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.Resume, careplan.CareResumeInput{
		StudentID:      student.ID,
		NewStart:       today,
		ActorAccountID: actorID,
		Checked:        true,
	}))

	rows, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

		instance.ID)

	require.NoError(t, err)
	assert.Empty(t, rows, "the old plan stays off — the school sets it up again")

	// And the ledger is gone, so a later exit cannot resurrect it either.
	endCare(t, ctx, svc, actorID, careplan.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today.AddDays(3),
		Reason:      careplan.CareExitReasonNoCareNeed,
	})
	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.NoError(t, err)

	after, err := testpkg.TenantQuery1(t, ctx, db, timetableRepos.InstanceStudent.FindByInstanceID,

		instance.ID)

	require.NoError(t, err)
	assert.Empty(t, after, "a discarded ledger stays discarded")
}
