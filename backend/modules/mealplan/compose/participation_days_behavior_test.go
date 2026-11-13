package compose_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	mealcompose "github.com/moto-ogs/project-phoenix/modules/mealplan/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParticipationDaysKeepsEffectiveSchedulesCutoffsAndTenantScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	student := testpkg.CreateTestStudent(t, db, "Meal", "Dates", "2a")
	other := testpkg.CreateTestStudentForTenant(t, db, otherSchool, "Other", "School", "3b")
	unrequested := testpkg.CreateTestStudent(t, db, "Other", "Child", "2a")
	account := testpkg.CreateTestAccount(t, db, "meal-dates")
	monday := calendar.NewDate(2026, 9, 7)
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_participation_schedules
  (tenant_id, student_id, effective_from, monday, tuesday, wednesday, thursday, friday, changed_by_account_id)
  VALUES (?, ?, ?, TRUE, TRUE, FALSE, FALSE, FALSE, ?),
         (?, ?, ?, FALSE, FALSE, FALSE, TRUE, FALSE, ?),
         (?, ?, ?, TRUE, FALSE, FALSE, FALSE, FALSE, ?),
         (?, ?, ?, TRUE, FALSE, FALSE, FALSE, FALSE, ?)`,
		school, student.ID, monday, account.ID,
		school, student.ID, monday.AddDays(3), account.ID,
		otherSchool, other.ID, monday, account.ID,
		school, unrequested.ID, monday, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_participation_overrides
  (tenant_id, student_id, date, participating, changed_by_account_id)
  VALUES (?, ?, ?, TRUE, ?), (?, ?, ?, FALSE, ?)`,
		school, student.ID, monday.AddDays(2), account.ID,
		school, student.ID, monday.AddDays(4), account.ID)
	require.NoError(t, err)
	reported := monday.BerlinMidnight().Add(8 * time.Hour)
	cleared := reported.Add(2 * time.Hour)
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_sickness_status_history
  (tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
  VALUES (?, ?, ?, 'sick', ?, ?, NULL), (?, ?, ?, 'sick', ?, ?, ?)`,
		school, student.ID, monday, reported, reported,
		school, student.ID, monday, cleared, reported, cleared)
	require.NoError(t, err)
	// The third child's schedule must not leak into the current roster.
	_, err = db.ExecContext(ctx, `UPDATE users.student_school_memberships
  SET enrolled_from = ? WHERE tenant_id = ? AND student_profile_id = ?`, monday.AddDays(7), school, unrequested.ID)
	require.NoError(t, err)
	module := mealcompose.BuildOrderDateModuleForTest(t, db, true, nil, monday.BerlinMidnight().Add(11*time.Hour))
	from, to := mealplan.Date(monday.String()), mealplan.Date(monday.AddDays(4).String())
	week, err := module.Orders(ctx, from, to)
	require.NoError(t, err)
	require.Len(t, week.Days, 5)
	for index, day := range week.Days {
		single, dailyErr := module.Orders(ctx, day.Date, day.Date)
		require.NoError(t, dailyErr)
		require.Len(t, single.Days, 1)
		assert.Equal(t, single.Days[0], day, "batch and scalar public orders on %s", day.Date)
		assert.Equal(t, []int{0, 1, 1, 1, 0}[index], day.Total)
		for _, child := range day.Children {
			assert.Equal(t, student.ID, child.StudentID)
			assert.NotEqual(t, other.ID, child.StudentID)
			assert.NotEqual(t, unrequested.ID, child.StudentID)
		}
	}
	foreign, err := module.Participation(ctx, other.ID, from, to)
	require.NoError(t, err)
	assert.Empty(t, foreign.Weekdays)
	for _, day := range foreign.Days {
		assert.False(t, day.Participating)
		assert.False(t, day.Regular)
	}
}
