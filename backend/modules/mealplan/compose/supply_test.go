package compose

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ADR 0077: the caterer sees names only while the school shows them, and
// confirms a fixed order once. Who supplies the school is Identity & Access
// data; the Meal Plan gets when the supply started.
func TestCatererReadsAndConfirmsOrdersOfItsSupply(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Lea", "Koch", "4b")
	parent := testpkg.CreateTestAccount(t, db, "meal-caterer-parent")
	caterer := testpkg.CreateTestAccount(t, db, "meal-caterer-account")
	require.NoError(t, buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-07", Participating: true}))

	settings := singleDeadlineSettings("09:00")
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	since := time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)

	week, err := module.CatererOrders(ctx, since, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, week.Days, 1)
	provisional := week.Days[0]
	assert.Equal(t, mealplan.OrderOrdered, provisional.Status, "due at 09:00, shown before the worker stores it")
	assert.Zero(t, provisional.OrderID, "an order not stored yet has no ID")
	assert.Zero(t, orderCountOf(t, db, student.TenantID), "the caterer's read writes nothing")
	require.ErrorIs(t, module.ConfirmOrder(ctx, provisional.OrderID, since, caterer.ID), mealplan.ErrOrderNotFound, "confirming waits for the stored order")

	require.NoError(t, module.SettleOrders(ctx))
	week, err = module.CatererOrders(ctx, since, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, week.Days, 1)
	day := week.Days[0]
	assert.NotZero(t, day.OrderID)
	assert.Equal(t, mealplan.OrderOrdered, day.Status)
	assert.Equal(t, provisional.Total, day.Total, "the stored order has the numbers the caterer saw")
	assert.Equal(t, 1, day.Total)
	assert.Equal(t, []mealplan.ClassCount{{SchoolClass: "4b", Count: 1}}, day.Classes)
	assert.Empty(t, day.Children, "numbers only by default")

	settings.CatererNames = true
	named, err := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)).CatererOrders(ctx, since, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, named.Days[0].Children, 1)
	assert.Equal(t, mealplan.OrderChild{FirstName: "Lea", LastName: "Koch", SchoolClass: "4b"}, named.Days[0].Children[0], "names without the child's ID")

	require.ErrorIs(t, module.ConfirmOrder(ctx, day.OrderID+1_000_000, since, caterer.ID), mealplan.ErrOrderNotFound)
	require.NoError(t, module.ConfirmOrder(ctx, day.OrderID, since, caterer.ID))
	require.NoError(t, module.ConfirmOrder(ctx, day.OrderID, since, caterer.ID), "confirming twice keeps the first")
	week, err = module.CatererOrders(ctx, since, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.Equal(t, mealplan.OrderConfirmed, week.Days[0].Status)
}

// ADR 0077: with orders by total the caterer reads no class.
func TestCatererReadsNoClassesWhenTheSchoolOrdersTotals(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Mia", "Summe", "1a")
	parent := testpkg.CreateTestAccount(t, db, "meal-total-parent")
	require.NoError(t, buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-07", Participating: true}))
	settings := singleDeadlineSettings("09:00")
	settings.OrderDetail = "total"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))

	week, err := module.CatererOrders(ctx, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin), "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.Equal(t, 1, week.Days[0].Total)
	assert.Empty(t, week.Days[0].Classes)
}

// ADR 0077: a caterer reads and confirms nothing the school ordered before
// its supply started, so a successor never sees its predecessor's orders.
func TestNewCatererSeesNothingBeforeItsSupply(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Ben", "Vorher", "2c")
	parent := testpkg.CreateTestAccount(t, db, "meal-successor-parent")
	register := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 4, 8, 0, 0, 0, testBerlin))
	for _, date := range []mealplan.Date{"2026-09-07", "2026-09-08"} {
		require.NoError(t, register.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: date, Participating: true}))
	}
	monday := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	require.NoError(t, monday.SettleOrders(ctx))
	week, err := monday.CatererOrders(ctx, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin), "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	mondayOrder := week.Days[0].OrderID
	// A takeover later on the same day must not expose its frozen order.
	takeover := time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin)
	previous, err := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), takeover).CatererOrders(ctx, takeover, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.Empty(t, previous.Days)
	require.ErrorIs(t, monday.ConfirmOrder(ctx, mondayOrder, takeover, 0), mealplan.ErrOrderNotFound)

	tuesday := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	require.NoError(t, tuesday.SettleOrders(ctx))
	week, err = tuesday.CatererOrders(ctx, takeover, "2026-09-07", "2026-09-08")
	require.NoError(t, err)
	require.Len(t, week.Days, 1, "Monday was ordered before the supply started")
	assert.Equal(t, mealplan.Date("2026-09-08"), week.Days[0].Date)
	require.ErrorIs(t, tuesday.ConfirmOrder(ctx, mondayOrder, takeover, 0), mealplan.ErrOrderNotFound)
	require.NoError(t, tuesday.ConfirmOrder(ctx, week.Days[0].OrderID, takeover, 0))
}

// Delayed settlement must not transfer an order to the successor, including
// the idempotent confirmation path for an already confirmed order.
func TestNewCatererCannotAccessPredecessorOrderSettledAfterTakeover(t *testing.T) {
	t.Parallel()
	for _, confirmed := range []bool{false, true} {
		t.Run(fmt.Sprintf("confirmed=%t", confirmed), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			settings := singleDeadlineSettings("09:00")
			takeover := time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)
			settled := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin))
			require.NoError(t, settled.SettleOrders(ctx))
			week, err := settled.Orders(ctx, "2026-09-07", "2026-09-07")
			require.NoError(t, err)
			require.Len(t, week.Days, 1)
			orderID := week.Days[0].OrderID
			require.NotZero(t, orderID)
			if confirmed {
				_, err = db.NewRaw(`UPDATE schedule.meal_orders SET confirmed_at = ? WHERE tenant_id = ? AND id = ?`, time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin), testpkg.TenantIDFromContext(ctx), orderID).Exec(ctx)
				require.NoError(t, err)
			}
			visible, err := settled.CatererOrders(ctx, takeover, "2026-09-07", "2026-09-07")
			require.NoError(t, err)
			assert.Empty(t, visible.Days, "the deadline preceded the new supply")
			require.ErrorIs(t, settled.ConfirmOrder(ctx, orderID, takeover, 0), mealplan.ErrOrderNotFound)
		})
	}
}
