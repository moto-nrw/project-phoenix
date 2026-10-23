package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOrders_QueryBudget pins the statements of reading a week of orders
// (#3873, #2940): fixing the due orders and recording the changes read per
// day in bulk, so the count must not grow with the number of children.
func TestOrders_QueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "meal-order-budget")
	addChildren := func(count int, now time.Time) {
		register := buildModuleAt(t, db, true, nil, now)
		for range count {
			student := testpkg.CreateTestStudent(t, db, "Budget", "Kind", "2a")
			_, err := register.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday}})
			require.NoError(t, err)
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	read := func(now time.Time) int {
		counter.Reset()
		_, err := buildModuleAt(t, db, true, nil, now).Orders(ctx, "2026-09-07", "2026-09-11")
		require.NoError(t, err)
		return counter.Total()
	}

	addChildren(3, time.Date(2026, 9, 4, 8, 0, 0, 0, testBerlin))
	read(time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)) // fixes Monday's order
	small := read(time.Date(2026, 9, 7, 10, 30, 0, 0, testBerlin))
	// Registered after Monday's deadline: from Tuesday on, so no change.
	addChildren(5, time.Date(2026, 9, 7, 10, 45, 0, 0, testBerlin))
	large := read(time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin))
	assert.Equal(t, small, large, "query count must not grow with the children")
	testpkg.AssertQueryBudget(t, "modules.mealplan.orders.read_week", counter.Queries())
}
