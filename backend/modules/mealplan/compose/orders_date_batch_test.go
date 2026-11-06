package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrdersDateBatchKeepsDailyRosterAndPendingSchedules(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "meal-date-roster")
	first := testpkg.CreateTestStudent(t, db, "First", "Child", "2a")
	entrant := testpkg.CreateTestStudent(t, db, "New", "Child", "3b")
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET enrolled_from = '2026-09-09' WHERE tenant_id = ? AND student_profile_id = ?`, testpkg.Tenant(t), entrant.ID)
	require.NoError(t, err)
	register := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
	allDays := []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday, mealplan.Thursday, mealplan.Friday}
	for _, studentID := range []int64{first.ID, entrant.ID} {
		effective := mealplan.Date("2026-09-07")
		if studentID == entrant.ID {
			effective = "2026-09-09"
		}
		_, err := register.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: studentID, StaffAccountID: account.ID, Weekdays: allDays, EffectiveFrom: effective})
		require.NoError(t, err)
	}
	_, err = register.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: first.ID, StaffAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Friday}, EffectiveFrom: "2026-09-10"})
	require.NoError(t, err)
	reader := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	school, err := reader.Orders(ctx, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	caterer, err := reader.CatererOrders(ctx, time.Date(2026, 9, 1, 8, 0, 0, 0, testBerlin), "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	for _, week := range []mealplan.OrderWeek{school, caterer} {
		require.Len(t, week.Days, 5)
		for index, total := range []int{1, 1, 2, 1, 2} {
			assert.Equal(t, total, week.Days[index].Total, "day %d", index)
		}
	}
}
