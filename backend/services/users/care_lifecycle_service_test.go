// "Betreuung beenden" (#2487): the regular exit of a child from the OGS.
//
// The invariants these tests hold: the last care day is INCLUSIVE, the
// confirmation is all-or-nothing and only valid for the exact state the
// preview described, another tenant's child is invisible, and everything that
// happened before the exit stays untouched.
package users_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
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

func newCareLifecycleService(t *testing.T, db *bun.DB) userService.CareLifecycleService {
	t.Helper()
	repos := repositories.NewFactory(db)
	return userService.NewCareLifecycleService(userService.CareLifecycleDependencies{
		StudentRepo:  repos.Student,
		PersonRepo:   repos.Person,
		CareExitRepo: repos.CareExit,
		CleanupRepo:  repos.CareExitCleanup,
		TagReleaser:  repos.GradeTransition,
		AuditService: userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
		DB:           db,
		Logger:       slog.Default(),
	})
}

// endCare runs the full product path — preview, then confirm the token it
// handed out — because that is the only way the action is reachable.
func endCare(
	t *testing.T,
	ctx context.Context,
	svc userService.CareLifecycleService,
	actorID int64,
	input userService.CareExitInput,
) *userService.CareExitResult {
	t.Helper()
	preview, err := svc.Preview(ctx, input)
	require.NoError(t, err)
	require.False(t, preview.Blocked, "preview must not be blocked: %+v", preview.Students)
	result, err := svc.Confirm(ctx, preview.Token, input, actorID)
	require.NoError(t, err)
	return result
}

func loadStudent(t *testing.T, db *bun.DB, ctx context.Context, id int64) *userModels.Student {
	t.Helper()
	student, err := repositories.NewFactory(db).Student.FindByID(ctx, id)
	require.NoError(t, err)
	return student
}

func TestCareLifecycle_LastCareDayIsInclusive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Lina", "Bergmann", "2a")
	today := timezone.TodayDate()

	endCare(t, ctx, svc, actorID, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonMovedAway,
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

	_, err := svc.Preview(ctx, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: timezone.TodayDate().AddDays(-1),
		Reason:      userModels.CareExitReasonNoCareNeed,
	})
	require.ErrorIs(t, err, userService.ErrCareExitDayInPast)
}

func TestCareLifecycle_ReasonNotePairing(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)

	student := testpkg.CreateTestStudent(t, db, "Mila", "Bergmann", "2a")
	today := timezone.TodayDate()

	t.Run("other requires a note", func(t *testing.T) {
		_, err := svc.Preview(ctx, userService.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      userModels.CareExitReasonOther,
		})
		require.ErrorIs(t, err, userModels.ErrCareExitNoteRequired)
	})

	t.Run("a categorised reason refuses a note", func(t *testing.T) {
		_, err := svc.Preview(ctx, userService.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      userModels.CareExitReasonMovedAway,
			ReasonNote:  "steht nicht zur Wahl",
		})
		require.ErrorIs(t, err, userModels.ErrCareExitNoteNotAllowed)
	})

	t.Run("an unknown reason is refused", func(t *testing.T) {
		_, err := svc.Preview(ctx, userService.CareExitInput{
			StudentIDs:  []int64{student.ID},
			LastCareDay: today,
			Reason:      "weggezogen",
		})
		require.ErrorIs(t, err, userModels.ErrCareExitInvalidReason)
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
	today := timezone.TodayDate()
	input := userService.CareExitInput{
		StudentIDs:  []int64{first.ID, second.ID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonNoCareNeed,
	}

	preview, err := svc.Preview(ctx, input)
	require.NoError(t, err)

	// Somebody edits one of the children between preview and confirmation.
	_, err = db.NewUpdate().
		TableExpr("users.students").
		Set("school_class = ?", "3b").
		Set("updated_at = ?", time.Now().Add(time.Second)).
		Where("id = ?", second.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = svc.Confirm(ctx, preview.Token, input, actorID)
	require.ErrorIs(t, err, userService.ErrCareExitPreviewChanged)

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
		TableExpr("users.students").
		Set("status = ?", string(userModels.StudentStatusAlumnus)).
		Where("id = ?", graduated.ID).
		Exec(context.Background())
	require.NoError(t, err)

	input := userService.CareExitInput{
		StudentIDs:  []int64{fine.ID, graduated.ID},
		LastCareDay: timezone.TodayDate(),
		Reason:      userModels.CareExitReasonMovedAway,
	}

	preview, err := svc.Preview(ctx, input)
	require.NoError(t, err)
	assert.True(t, preview.Blocked)

	blocked := map[int64]string{}
	for _, impact := range preview.Students {
		blocked[impact.StudentID] = impact.Blocker
	}
	assert.Empty(t, blocked[fine.ID], "the healthy child carries no blocker")
	assert.NotEmpty(t, blocked[graduated.ID], "the graduate is named with a reason")

	_, err = svc.Confirm(ctx, preview.Token, input, actorID)
	require.ErrorIs(t, err, userService.ErrCareExitBlocked)
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

	preview, err := svc.Preview(ctx, userService.CareExitInput{
		StudentIDs:  []int64{foreign.ID},
		LastCareDay: timezone.TodayDate(),
		Reason:      userModels.CareExitReasonMovedAway,
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	assert.NotEmpty(t, preview.Students[0].Blocker,
		"a child of another school is unknown here, not endable")

	stored, err := repositories.NewFactory(db).Student.FindByID(
		tenant.WithTenantID(context.Background(), otherTenantID), foreign.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.EnrolledUntil)
}

func TestCareLifecycle_EndsBookingsAtTheLastCareDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)
	repos := repositories.NewFactory(db)

	student := testpkg.CreateTestStudent(t, db, "Ella", "Vogt", "2b")
	group := testpkg.CreateTestActivityGroup(t, db, "Fußball")
	today := timezone.TodayDate()

	booking := &activityModels.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       today.AddDays(-30),
	}
	booking.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentEnrollment.Create(ctx, booking))

	preview, err := svc.Preview(ctx, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonNoCareNeed,
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	assert.Equal(t, 1, preview.Students[0].ActivityBookings,
		"the preview names the booking before it ends it")

	result, err := svc.Confirm(ctx, preview.Token, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonNoCareNeed,
	}, actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.BookingsEnded)

	// valid_until is an EXCLUSIVE bound, so the booking still counts on the
	// last care day and stops the day after.
	stillToday, err := repos.StudentEnrollment.FindActiveByStudentIDs(ctx, []int64{student.ID}, today)
	require.NoError(t, err)
	assert.Len(t, stillToday, 1, "the booking still counts on the last care day")

	tomorrow, err := repos.StudentEnrollment.FindActiveByStudentIDs(ctx, []int64{student.ID}, today.AddDays(1))
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
	future := timezone.TodayDate().AddDays(14)

	endCare(t, ctx, svc, actorID, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: future,
		Reason:      userModels.CareExitReasonOther,
		ReasonNote:  "Wechsel in den Hort",
	})

	cancelled, err := svc.Cancel(ctx, []int64{student.ID}, actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, cancelled)
	assert.Nil(t, loadStudent(t, db, ctx, student.ID).EnrolledUntil,
		"cancelling removes the planned end entirely")

	// The reason row goes with it — a cancelled exit has no reason to keep.
	reasons, err := repositories.NewFactory(db).CareExit.FindByStudentIDs(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, reasons)

	// A second cancel has nothing to withdraw.
	_, err = svc.Cancel(ctx, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, userService.ErrCareExitNotPlanned)
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
	yesterday := timezone.TodayDate().AddDays(-1)
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", yesterday).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, userService.ErrCareExitAlreadyEffective)
}

func TestCareLifecycle_Resume(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	student := testpkg.CreateTestStudent(t, db, "Yara", "Lorenz", "1c")
	yesterday := timezone.TodayDate().AddDays(-1)
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", yesterday).
		Set("status = ?", string(userModels.StudentStatusInactive)).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	t.Run("refuses without the explicit review", func(t *testing.T) {
		err := svc.Resume(ctx, userService.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       timezone.TodayDate(),
			ActorAccountID: actorID,
			Checked:        false,
		})
		require.ErrorIs(t, err, userService.ErrCareResumeNotChecked)
	})

	t.Run("refuses a start in the past", func(t *testing.T) {
		err := svc.Resume(ctx, userService.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       yesterday,
			ActorAccountID: actorID,
			Checked:        true,
		})
		require.ErrorIs(t, err, userService.ErrCareResumeStartInPast)
	})

	t.Run("reopens the care from the new start", func(t *testing.T) {
		require.NoError(t, svc.Resume(ctx, userService.CareResumeInput{
			StudentID:      student.ID,
			NewStart:       timezone.TodayDate(),
			ActorAccountID: actorID,
			Checked:        true,
		}))

		stored := loadStudent(t, db, ctx, student.ID)
		assert.Nil(t, stored.EnrolledUntil, "the end of care is gone")
		require.NotNil(t, stored.EnrolledFrom)
		assert.Equal(t, timezone.TodayDate(), *stored.EnrolledFrom)
		assert.Equal(t, userModels.StudentStatusActive, stored.Status,
			"a child resumed for today is active right away")
	})

	t.Run("refuses a child whose care is running", func(t *testing.T) {
		running := testpkg.CreateTestStudent(t, db, "Noah", "Lorenz", "1c")
		err := svc.Resume(ctx, userService.CareResumeInput{
			StudentID:      running.ID,
			NewStart:       timezone.TodayDate(),
			ActorAccountID: actorID,
			Checked:        true,
		})
		require.ErrorIs(t, err, userService.ErrCareResumeNotEnded)
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
		TableExpr("users.students").
		Set("enrolled_until = ?", timezone.TodayDate().AddDays(-3)).
		Set("status = ?", string(userModels.StudentStatusInactive)).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	start := timezone.TodayDate().AddDays(10)
	require.NoError(t, svc.Resume(ctx, userService.CareResumeInput{
		StudentID:      student.ID,
		NewStart:       start,
		ActorAccountID: actorID,
		Checked:        true,
	}))

	stored := loadStudent(t, db, ctx, student.ID)
	assert.Equal(t, userModels.StudentStatusPending, stored.Status,
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

	yesterday := timezone.TodayDate().AddDays(-1)
	for _, id := range []int64{manual.ID, phase.ID} {
		_, err := db.NewUpdate().
			TableExpr("users.students").
			Set("enrolled_until = ?", yesterday).
			Where("id = ?", id).
			Exec(context.Background())
		require.NoError(t, err)
	}
	require.NoError(t, repositories.NewFactory(db).CareExit.Upsert(ctx, &userModels.CareExit{
		StudentID:  manual.ID,
		Reason:     userModels.CareExitReasonMovedAway,
		RecordedBy: ptrInt64(actorID),
	}))

	rows, total, err := svc.ListEnded(ctx, userModels.CareExitListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	byID := map[int64]*userModels.EndedCare{}
	for _, row := range rows {
		byID[row.StudentID] = row
	}
	require.Contains(t, byID, manual.ID)
	require.Contains(t, byID, phase.ID)
	assert.NotContains(t, byID, running.ID, "a child still in care is not in the archive")

	require.NotNil(t, byID[manual.ID].Reason)
	assert.Equal(t, userModels.CareExitReasonMovedAway, *byID[manual.ID].Reason)
	assert.Nil(t, byID[phase.ID].Reason,
		"a care that ran out on its own has no recorded reason, and says so")
}

func ptrInt64(value int64) *int64 { return &value }
