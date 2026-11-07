package compose

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The worker fixes due orders under the order lock (#5839). The children and
// absences of all due days are one date-set read after that lock, so a week
// order costs what an order of two days costs. Each day still counts only the
// absences reported by the order's deadline.
func TestSettleReadsTheDueDaysOnce(t *testing.T) {
	t.Parallel()
	weekly := splitSettings()
	weekly.OrderRhythm = "weekly"

	// settle registers children for Monday to Friday in a tenant of the
	// subtest and lets the worker run at now. It returns the worker's
	// statements, its registration reads before it stores the first order,
	// and the order days it stored; the registrations already fixed the
	// days due when they were written.
	settle := func(t *testing.T, children int, now time.Time) (int, int, int) {
		t.Helper()
		testpkg.OwnTenant(t)
		db := testpkg.SetupTestDB(t)
		counter := testpkg.CaptureQueriesForContext(t, db)
		ctx := testpkg.Ctx(t)
		account := testpkg.CreateTestAccount(t, db, "meal-settle-statements")
		early := buildModuleWithRules(t, db, weekly, time.Date(2026, 9, 2, 10, 0, 0, 0, testBerlin))
		var tenantID int64
		for range children {
			student := testpkg.CreateTestStudent(t, db, "Wenn", "Woche", "4a")
			tenantID = student.TenantID
			_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID,
				Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday, mealplan.Thursday, mealplan.Friday}})
			require.NoError(t, err)
		}
		worker := buildModuleWithRules(t, db, weekly, now)
		before := orderCountOf(t, db, tenantID)
		require.NoError(t, worker.SettleOrders(counter.Context(ctx)))
		registrationReads := 0
		for _, sql := range counter.Queries() {
			if strings.HasPrefix(sql, "INSERT INTO schedule.meal_orders") {
				break
			}
			for _, table := range []string{`"meal_participation_schedule"`, `"meal_participation_override"`, `"sickness_history"`} {
				if strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, table) {
					registrationReads++
				}
			}
		}
		return counter.Total(), registrationReads, orderCountOf(t, db, tenantID) - before
	}

	type result struct{ total, registrationReads int }
	results := map[string]result{}
	for _, tc := range []struct {
		name     string
		children int
		now      time.Time
		days     int
	}{
		// Thursday 11:00: Thursday and Friday of the running week are open;
		// the next week's registration closes only at noon.
		{name: "two due days", children: 3, now: time.Date(2026, 9, 10, 11, 0, 0, 0, testBerlin), days: 2},
		// Friday 10:00: Friday of the running week and the whole next week.
		{name: "six due days", children: 3, now: time.Date(2026, 9, 11, 10, 0, 0, 0, testBerlin), days: 6},
		{name: "six due days, eight children", children: 8, now: time.Date(2026, 9, 11, 10, 0, 0, 0, testBerlin), days: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total, reads, stored := settle(t, tc.children, tc.now)
			assert.Equal(t, tc.days, stored)
			results[tc.name] = result{total: total, registrationReads: reads}
		})
	}
	t.Logf("settle statements: %+v", results)
	for name, measured := range results {
		assert.Equal(t, 3, measured.registrationReads, "%s: plans, overrides and absences of all due days are one read each", name)
	}
}

// An absence reported before a week order's deadline cancels that day's
// meal; one reported later does not: each day keeps its own cutoff when the
// days are read together.
func TestSettleKeepsEachDaysDeadlineWhenReadTogether(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	weekly := splitSettings()
	weekly.OrderRhythm = "weekly"
	account := testpkg.CreateTestAccount(t, db, "meal-settle-cutoffs")
	early := buildModuleWithRules(t, db, weekly, time.Date(2026, 9, 2, 10, 0, 0, 0, testBerlin))
	sickEarly := testpkg.CreateTestStudent(t, db, "Frida", "Früh", "4a")
	sickLate := testpkg.CreateTestStudent(t, db, "Simon", "Spät", "4a")
	healthy := testpkg.CreateTestStudent(t, db, "Greta", "Gesund", "4a")
	for _, id := range []int64{sickEarly.ID, sickLate.ID, healthy.ID} {
		_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: id, GuardianAccountID: account.ID,
			Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Wednesday, mealplan.Friday}})
		require.NoError(t, err)
	}
	// The order of the week of 2026-09-14 is due at Thursday 2026-09-10 noon.
	createSickStatusDay(t, db, sickEarly.TenantID, sickEarly.ID, "2026-09-16", time.Date(2026, 9, 10, 11, 0, 0, 0, testBerlin))
	createSickStatusDay(t, db, sickLate.TenantID, sickLate.ID, "2026-09-16", time.Date(2026, 9, 10, 13, 0, 0, 0, testBerlin))

	worker := buildModuleWithRules(t, db, weekly, time.Date(2026, 9, 11, 10, 0, 0, 0, testBerlin))
	require.NoError(t, worker.SettleOrders(ctx))
	week, err := worker.Orders(ctx, "2026-09-14", "2026-09-18")
	require.NoError(t, err)
	monday := orderDay(t, week, "2026-09-14")
	wednesday := orderDay(t, week, "2026-09-16")
	assert.NotZero(t, wednesday.OrderID)
	assert.Equal(t, monday.OrderID, wednesday.OrderID, "one order for the week")
	assert.ElementsMatch(t, []int64{sickEarly.ID, sickLate.ID, healthy.ID}, orderedChildren(t, db, monday.OrderID, "2026-09-14"))
	assert.ElementsMatch(t, []int64{sickLate.ID, healthy.ID}, orderedChildren(t, db, wednesday.OrderID, "2026-09-16"),
		"only the absence reported by the deadline leaves the order")
	assert.Empty(t, orderedChildren(t, db, monday.OrderID, "2026-09-15"), "nobody eats on Tuesday")
}

// orderedChildren reads the children the stored order holds for a day.
func orderedChildren(t *testing.T, db *bun.DB, orderID int64, date string) []int64 {
	t.Helper()
	ids := []int64{}
	require.NoError(t, db.NewRaw(`SELECT student_id FROM schedule.meal_order_lines WHERE order_id = ? AND date = ? ORDER BY student_id`, orderID, date).
		Scan(testpkg.Ctx(t), &ids))
	return ids
}
