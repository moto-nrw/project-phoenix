package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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
	rows := []participationScheduleRow{
		{TenantID: school, StudentID: student.ID, EffectiveFrom: monday, Monday: true, Tuesday: true, ChangedBy: account.ID},
		{TenantID: school, StudentID: student.ID, EffectiveFrom: monday.AddDays(3), Thursday: true, ChangedBy: account.ID},
		{TenantID: otherSchool, StudentID: other.ID, EffectiveFrom: monday, Monday: true, ChangedBy: account.ID},
		{TenantID: school, StudentID: unrequested.ID, EffectiveFrom: monday, Monday: true, ChangedBy: account.ID},
	}
	_, err := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_participation_schedules`).Exec(ctx)
	require.NoError(t, err)
	overrides := []participationOverrideRow{
		{TenantID: school, StudentID: student.ID, Date: monday.AddDays(2), Participating: true, ChangedBy: account.ID},
		{TenantID: school, StudentID: student.ID, Date: monday.AddDays(4), Participating: false, ChangedBy: account.ID},
	}
	_, err = db.NewInsert().Model(&overrides).ModelTableExpr(`schedule.meal_participation_overrides`).Exec(ctx)
	require.NoError(t, err)
	tuesday := monday.AddDays(1)
	reported := tuesday.BerlinMidnight().Add(8 * time.Hour)
	cleared := reported.Add(2 * time.Hour)
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_sickness_status_history
  (tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
  VALUES (?, ?, ?, 'sick', ?, ?, NULL), (?, ?, ?, 'sick', ?, ?, ?)`,
		school, student.ID, tuesday, reported, reported,
		school, student.ID, tuesday, cleared, reported, cleared)
	require.NoError(t, err)
	store := New(func(context.Context) (bun.IDB, int64, error) { return db, school, nil })
	cutoffs := map[domain.Date]time.Time{}
	for offset := range 5 {
		date := monday.AddDays(offset)
		cutoffs[date] = date.BerlinMidnight().Add(9 * time.Hour)
	}
	ids := []int64{student.ID, other.ID}
	days, _, err := store.FindParticipationDays(ctx, ids, cutoffs)
	require.NoError(t, err)
	for date, cutoff := range cutoffs {
		daily, _, dailyErr := store.FindDailyParticipation(ctx, ids, date, cutoff)
		require.NoError(t, dailyErr)
		for _, id := range ids {
			assert.Equal(t, daily[id], days[date][id], "child %d on %s", id, date)
		}
		assert.NotContains(t, days[date], unrequested.ID)
		assert.Equal(t, domain.DailyParticipation{}, days[date][other.ID])
	}
	assert.True(t, days[monday][student.ID].Regular)
	assert.False(t, days[monday.AddDays(4)][student.ID].Regular)
	require.Len(t, days[tuesday][student.ID].Absences, 1, "Tuesday's later clearance is beyond its cutoff, even though Wednesday's cutoff is later")
	require.NotNil(t, days[monday.AddDays(2)][student.ID].Override)
	assert.True(t, *days[monday.AddDays(2)][student.ID].Override)
}
