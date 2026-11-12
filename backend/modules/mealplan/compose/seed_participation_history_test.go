package compose_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	mealcompose "github.com/moto-ogs/project-phoenix/modules/mealplan/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedParticipationHistoryPreservesDecisionsAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	student := testpkg.CreateTestStudent(t, db, "Past", "Meal", "2a")
	other := testpkg.CreateTestStudentForTenant(t, db, otherSchool, "Other", "Meal", "3a")
	account := testpkg.CreateTestAccount(t, db, "seed-meal-history")
	day := calendar.NewDate(2026, 9, 7)
	future := calendar.NewDate(2026, 10, 1)
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_participation_schedules
 (tenant_id,student_id,effective_from,monday,changed_by_account_id) VALUES (?,?,?,true,?),(?,?,?,true,?)`, school, student.ID, future, account.ID, otherSchool, other.ID, future, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_participation_overrides
 (tenant_id,student_id,date,participating,changed_by_account_id) VALUES (?,?,?,false,?)`, school, student.ID, day.AddDays(1), account.ID)
	require.NoError(t, err)
	history := mealcompose.NewParticipationHistory()
	days := []mealplan.PastMealDay{{StudentID: student.ID, Date: day}, {StudentID: student.ID, Date: day.AddDays(1)}, {StudentID: other.ID, Date: day}}
	for range 2 {
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
			return history.RecordParticipationHistory(testpkg.ContextForTenant(adminCtx, school), days)
		}))
	}
	var rows []struct {
		StudentID     int64
		Date          calendar.Date
		Participating bool
	}
	require.NoError(t, db.NewRaw(`SELECT student_id,date,participating FROM schedule.meal_participation_overrides WHERE tenant_id=? ORDER BY date`, school).Scan(ctx, &rows))
	require.Len(t, rows, 2)
	assert.Equal(t, student.ID, rows[0].StudentID)
	assert.True(t, rows[0].Participating)
	assert.False(t, rows[1].Participating, "keep the existing cancellation on a repeated seed")
	count, err := db.NewSelect().TableExpr("schedule.meal_participation_schedules").Where("tenant_id=? AND student_id=? AND effective_from=?", school, student.ID, future).Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count, "preserve the current schedule")
}

func TestSeedParticipationHistoryRejectsInvalidDaysAtomically(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Invalid", "Meal", "1a")
	history := mealcompose.NewParticipationHistory()
	require.Error(t, history.RecordParticipationHistory(ctx, []mealplan.PastMealDay{{StudentID: student.ID, Date: calendar.NewDate(2026, 9, 7)}, {StudentID: student.ID, Date: calendar.NewDate(2099, 9, 7)}}))
	count, err := db.NewSelect().TableExpr("schedule.meal_participation_overrides").Where("tenant_id=?", testpkg.Tenant(t)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
}
