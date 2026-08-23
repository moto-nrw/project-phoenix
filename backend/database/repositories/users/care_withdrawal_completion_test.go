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
	repo := repositories.NewFactory(db).CareWithdrawal
	student := testpkg.CreateTestStudent(t, db, "Mira", "Kurz", "2a")
	actor := testpkg.CreateTestAccount(t, db, "withdrawal-actor")
	studentID := student.ID
	firstGap := timezone.TodayDate().AddDays(3)

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
	assert.Equal(t, second.FirstBookinglessDay, rows[0].FirstBookinglessDay)
	assert.Equal(t, "Mira", rows[0].FirstName)
}

func TestCareWithdrawalCompletionRepository_RebookingOnlyObsoletesWithoutGap(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).CareWithdrawal
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

func TestCareWithdrawalCompletionRepository_CancelCreatesNewPendingEvent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).CareWithdrawal
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
