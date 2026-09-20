package users_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestCareWithdrawalCompletionRepository_OnePendingTaskPerChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Mira", "Kurz", "2a")
	actor := testpkg.CreateTestAccount(t, db, "withdrawal-actor")
	studentID := student.ID
	firstGap := timezone.NewDate(2026, 8, 24).AddDays(3)

	first := &userModels.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy:   &actor.ID,
		WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt:   time.Now(),
	}
	require.NoError(t, repo.UpsertPending(ctx, first))
	firstID := first.ID

	second := *first
	second.ID = 0
	second.FirstBookinglessDay = firstGap.AddDays(2)
	require.NoError(t, repo.UpsertPending(ctx, &second))
	assert.Equal(t, firstID, second.ID)

	rows, total, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, second.FirstBookinglessDay, rows[0].FirstBookinglessDay,
		"reconciliation must move the one open task when the real gap moves")
	assert.Equal(t, "Mira", rows[0].FirstName)
}

func TestCareWithdrawalCompletionRepository_RebookingOnlyObsoletesWithoutGap(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Sam", "Kurz", "3a")
	actor := testpkg.CreateTestAccount(t, db, "rebooking-actor")
	studentID := student.ID
	firstGap := timezone.TodayDate().AddDays(2)

	create := func() {
		t.Helper()
		require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: firstGap,
			Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
			WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
		}))
	}

	create()
	changed, err := repo.MarkObsoleteForRebooking(ctx, student.ID, firstGap.AddDays(1), time.Now())
	require.NoError(t, err)
	assert.False(t, changed, "a booking after a real gap must keep the completion open")

	changed, err = repo.MarkObsoleteForRebooking(ctx, student.ID, firstGap, time.Now())
	require.NoError(t, err)
	assert.True(t, changed)
	rows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: student.ID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCareWithdrawalCompletionRepository_UpsertUsesIncomingBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Echte", "Lücke", "3b")
	studentID := student.ID
	firstGap := timezone.NewDate(2026, 8, 24).AddDays(-2)
	completion := &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger: userModels.CareWithdrawalTriggerBookingExpired, WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
	}
	require.NoError(t, repo.UpsertPending(ctx, completion))
	completion.ID = 0
	completion.FirstBookinglessDay = timezone.NewDate(2026, 8, 24).AddDays(5)
	require.NoError(t, repo.UpsertPending(ctx, completion))
	rows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, timezone.NewDate(2026, 8, 24).AddDays(5), rows[0].FirstBookinglessDay)
}

// TestCareWithdrawalCompletionRepository_UpsertPreservesSchoolConfirmation pins
// the conflict-update CASE: a task the school confirmed directly keeps its
// confirmation when a booking expiry lands on the same child, while every other
// combination takes the incoming values.
func TestCareWithdrawalCompletionRepository_UpsertPreservesSchoolConfirmation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	lifecycle, err := repositories.NewCareLifecycleTestRepositories(db, nil)
	require.NoError(t, err)
	repo := lifecycle.CareWithdrawal
	actor := testpkg.CreateTestAccount(t, db, "withdrawal-confirmer")
	gap := timezone.NewDate(2026, 8, 24)

	t.Run("booking expiry does not overwrite a school confirmation", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Behalten", "Bestätigt", "1a")
		studentID := student.ID
		confirmed := &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: gap,
			Trigger: userModels.CareWithdrawalTriggerDirectSchool, WithdrawalConfirmedBy: &actor.ID,
			WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
		}
		require.NoError(t, repo.UpsertPending(ctx, confirmed))

		require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: gap.AddDays(3),
			Trigger: userModels.CareWithdrawalTriggerBookingExpired, WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
		}))

		stored, err := repo.FindByID(ctx, confirmed.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		assert.Equal(t, userModels.CareWithdrawalTriggerDirectSchool, stored.Trigger)
		assert.Equal(t, "admin", stored.WithdrawalConfirmedRole)
		require.NotNil(t, stored.WithdrawalConfirmedBy)
		assert.Equal(t, actor.ID, *stored.WithdrawalConfirmedBy)
		assert.Equal(t, gap.AddDays(3), stored.FirstBookinglessDay, "the gap itself always follows the incoming row")
	})

	t.Run("school confirmation replaces an expired booking", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Ersetzt", "Abgelaufen", "1a")
		studentID := student.ID
		expired := &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: gap,
			Trigger: userModels.CareWithdrawalTriggerBookingExpired, WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
		}
		require.NoError(t, repo.UpsertPending(ctx, expired))

		require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: gap,
			Trigger: userModels.CareWithdrawalTriggerDirectSchool, WithdrawalConfirmedBy: &actor.ID,
			WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
		}))

		stored, err := repo.FindByID(ctx, expired.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		assert.Equal(t, userModels.CareWithdrawalTriggerDirectSchool, stored.Trigger)
		assert.Equal(t, "admin", stored.WithdrawalConfirmedRole)
		require.NotNil(t, stored.WithdrawalConfirmedBy)
		assert.Equal(t, actor.ID, *stored.WithdrawalConfirmedBy)
	})
}

func TestCareWithdrawalCompletionRepository_ParticipationBoundaryUsesPendingCompletionWhenEnrollmentIsOpen(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	repo := factory.CareWithdrawal
	// Callers pass the tenant rows they read through FindByIDs (#3221).
	studentsByID := func(ids ...int64) map[int64]*userModels.Student {
		t.Helper()
		students, err := factory.Student.FindByIDs(ctx, ids)
		require.NoError(t, err)
		return students
	}
	student := testpkg.CreateTestStudent(t, db, "Offen", "Grenze", "3b")
	studentID := student.ID
	firstGap := timezone.NewDate(2026, 8, 24).AddDays(4)

	require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger: userModels.CareWithdrawalTriggerDirectSchool, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}))

	boundaries, err := repo.ListParticipationBoundaries(ctx, studentsByID(student.ID), false)
	require.NoError(t, err)
	assert.Equal(t, firstGap, boundaries[student.ID])

	// The boundary merge moved out of SQL with #3221: the earlier of the day
	// after enrolled_until and the pending completion wins, booking-expiry
	// tasks count only on request, and a child with neither has no boundary.
	day := timezone.NewDate(2026, 9, 7)
	setEnrolledUntil := func(studentID int64, until timezone.Date) {
		t.Helper()
		_, err := db.NewUpdate().TableExpr("users.student_school_memberships").Set("enrolled_until = ?", until).Where("student_profile_id = ? AND deleted_at IS NULL", studentID).Exec(ctx)
		require.NoError(t, err)
	}
	upsert := func(studentID int64, gap timezone.Date, trigger string) {
		t.Helper()
		require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: gap,
			Trigger: trigger, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
		}))
	}

	enrollmentFirst := testpkg.CreateTestStudent(t, db, "Früh", "Ende", "4a")
	setEnrolledUntil(enrollmentFirst.ID, day)
	upsert(enrollmentFirst.ID, day.AddDays(5), userModels.CareWithdrawalTriggerDirectSchool)

	completionFirst := testpkg.CreateTestStudent(t, db, "Früh", "Abmeldung", "4a")
	setEnrolledUntil(completionFirst.ID, day.AddDays(10))
	upsert(completionFirst.ID, day.AddDays(3), userModels.CareWithdrawalTriggerDirectSchool)

	bookingOnly := testpkg.CreateTestStudent(t, db, "Nur", "Buchung", "4a")
	upsert(bookingOnly.ID, day.AddDays(2), userModels.CareWithdrawalTriggerBookingExpired)

	open := testpkg.CreateTestStudent(t, db, "Ohne", "Ende", "4a")
	students := studentsByID(enrollmentFirst.ID, completionFirst.ID, bookingOnly.ID, open.ID)
	require.Len(t, students, 4)

	boundaries, err = repo.ListParticipationBoundaries(ctx, students, false)
	require.NoError(t, err)
	assert.Equal(t, map[int64]timezone.Date{
		enrollmentFirst.ID: day.AddDays(1),
		completionFirst.ID: day.AddDays(3),
	}, boundaries)

	boundaries, err = repo.ListParticipationBoundaries(ctx, students, true)
	require.NoError(t, err)
	assert.Equal(t, map[int64]timezone.Date{
		enrollmentFirst.ID: day.AddDays(1),
		completionFirst.ID: day.AddDays(3),
		bookingOnly.ID:     day.AddDays(2),
	}, boundaries)
}

func TestCareWithdrawalCompletionRepository_WeeklyPlansObsoletePending(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Mia", "Wochenplan", "2a")
	actor := testpkg.CreateTestAccount(t, db, "weekly-plan-actor")
	studentID := student.ID
	require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: timezone.TodayDate(),
		Trigger:               userModels.CareWithdrawalTriggerBookingExpired,
		WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}))
	direct := testpkg.CreateTestStudent(t, db, "Noah", "Direkt", "2b")
	directID := direct.ID
	require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID: &directID, FirstBookinglessDay: timezone.TodayDate(),
		Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}))

	changed, err := repo.MarkPendingObsoleteForWeeklyPlans(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, changed)
	rows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.Empty(t, rows)
	directRows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: directID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	require.Len(t, directRows, 1)
	assert.Equal(t, userModels.CareWithdrawalTriggerDirectSchool, directRows[0].Trigger)
}

func TestCareWithdrawalCompletionRepository_CancelCreatesNewPendingEvent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Lia", "Storno", "1a")
	actor := testpkg.CreateTestAccount(t, db, "withdrawal-cancel-actor")
	studentID := student.ID
	original := &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: timezone.TodayDate().AddDays(4),
		Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}
	require.NoError(t, repo.UpsertPending(ctx, original))
	resolved, err := repo.MarkResolved(ctx, original.ID, actor.ID, time.Now())
	require.NoError(t, err)
	require.True(t, resolved)

	reopened, err := repo.ReopenAfterCancelledExit(ctx, original.ID+1, student.ID, time.Now())
	require.NoError(t, err)
	assert.False(t, reopened, "an unrelated exit must not resurrect the latest resolved withdrawal")

	reopened, err = repo.ReopenAfterCancelledExit(ctx, original.ID, student.ID, time.Now())
	require.NoError(t, err)
	require.True(t, reopened)
	rows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: student.ID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	pending := rows[0]
	assert.NotEqual(t, original.ID, pending.ID, "cancellation is a new durable event, not a rewrite of the resolved outcome")
	assert.Equal(t, original.FirstBookinglessDay, pending.FirstBookinglessDay)

	var resolvedCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.care_withdrawal_completions WHERE id = ? AND state = 'resolved'`, original.ID).Scan(ctx, &resolvedCount))
	assert.Equal(t, 1, resolvedCount)
}
