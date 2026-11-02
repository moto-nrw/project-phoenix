package compose

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// TestOrders_QueryBudget pins the statements of reading a week of orders
// (#3873, #2940) in its most expensive shape: Monday is due and the worker
// has not stored it yet, so the read lays it out as of its deadline beside
// the current registration of every day. Each day is read in bulk, so the
// count must not grow with the number of children, and the read writes
// nothing.
func TestOrders_QueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "meal-order-budget")
	// Registered before Monday's 09:00 deadline, so no write stores an order.
	register := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
	addChildren := func(count int) {
		for range count {
			student := testpkg.CreateTestStudent(t, db, "Budget", "Kind", "2a")
			_, err := register.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday}})
			require.NoError(t, err)
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	reader := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	read := func(children int) int {
		counter.Reset()
		week, err := reader.Orders(ctx, "2026-09-07", "2026-09-11")
		require.NoError(t, err)
		monday := orderDay(t, week, "2026-09-07")
		assert.Equal(t, mealplan.OrderOrdered, monday.Status)
		assert.Zero(t, monday.OrderID, "Monday is laid out, not stored")
		assert.Equal(t, children, monday.Total)
		return counter.Total()
	}

	addChildren(3)
	small := read(3)
	addChildren(5)
	large := read(8)
	assert.Equal(t, small, large, "query count must not grow with the children")
	assert.Empty(t, append(append(counter.Operation("INSERT"), counter.Operation("UPDATE")...), counter.Operation("DELETE")...), "a read writes nothing")
	assert.Empty(t, counter.Matching(func(sql string) bool { return strings.Contains(sql, "pg_advisory") }), "a read takes no lock")
	querybudgets.AssertQueryBudget(t, "modules.mealplan.orders.read_week", counter.Queries())
}
