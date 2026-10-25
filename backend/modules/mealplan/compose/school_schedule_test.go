package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSchoolScheduleRequiresCareOnEffectiveDate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	effective := calendar.NewDate(2026, 9, 10)
	before, after := effective.AddDays(-1), effective.AddDays(1)
	for _, tc := range []struct {
		name        string
		from, until *calendar.Date
		status      string
		allowed     bool
	}{
		{name: "care already ended", until: &before, status: "active"},
		{name: "care not started yet", from: &after, status: "active"},
		{name: "inactive membership", status: "inactive"},
		{name: "first day of care", from: &effective, status: "active", allowed: true},
		{name: "last day of care", until: &effective, status: "active", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testpkg.OwnCtx(t)
			student := testpkg.CreateTestStudent(t, db, "Sina", "Schedule", "1a")
			staff := testpkg.CreateTestAccount(t, db, "meal-schedule-staff")
			_, err := db.NewRaw(`UPDATE users.student_school_memberships SET enrolled_from = ?, enrolled_until = ?, status = ? WHERE tenant_id = ? AND student_profile_id = ?`, tc.from, tc.until, tc.status, student.TenantID, student.ID).Exec(ctx)
			require.NoError(t, err)
			module := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 9, 8, 0, 0, 0, testBerlin))
			from, err := module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{
				StudentID: student.ID, StaffAccountID: staff.ID,
				Weekdays: []mealplan.Weekday{mealplan.Thursday}, EffectiveFrom: mealplan.Date(effective),
			})
			var wantWrites int64
			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, mealplan.Date(effective), from)
				wantWrites = 1
			} else {
				require.ErrorIs(t, err, mealplan.ErrChildNotEnrolled)
			}
			for _, table := range []string{"schedule.meal_participation_schedules", "schedule.meal_registration_changes"} {
				count, countErr := db.NewSelect().TableExpr(table).Where("tenant_id = ? AND student_id = ?", student.TenantID, student.ID).Count(ctx)
				require.NoError(t, countErr)
				assert.Equal(t, wantWrites, count, table)
			}
		})
	}
}
