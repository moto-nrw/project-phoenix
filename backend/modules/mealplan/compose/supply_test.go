package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// createTestCaterer adds a caterer business. Caterers belong to no school,
// so the test removes it, and its supplies with it.
func createTestCaterer(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES (?) RETURNING id`, fmt.Sprintf("%s %d", name, time.Now().UnixNano())).Scan(context.Background(), &id))
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.caterers WHERE id = ?`, id).Exec(context.Background())
		require.NoError(t, err)
	})
	return id
}

// ADR 0077: a school has one caterer at a time; starting the running supply
// again changes nothing, and ending it frees the school for another.
func TestSupplyHasOneCatererAtATime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestAccount(t, db, "meal-supply-staff")
	first, second := createTestCaterer(t, db, "Küche Eins"), createTestCaterer(t, db, "Küche Zwei")
	module := buildModule(t, db, true, nil)

	_, active, err := module.CurrentSupply(ctx)
	require.NoError(t, err)
	assert.False(t, active)
	require.NoError(t, module.StartSupply(ctx, first, " Schulstraße 1 ", staff.ID))
	require.NoError(t, module.StartSupply(ctx, first, "Schulstraße 1", staff.ID), "starting again changes nothing")
	require.ErrorIs(t, module.StartSupply(ctx, second, "", staff.ID), mealplan.ErrSupplyActive)

	supply, active, err := module.CurrentSupply(ctx)
	require.NoError(t, err)
	require.True(t, active)
	assert.Equal(t, first, supply.CatererID)
	assert.Equal(t, "Schulstraße 1", supply.DeliveryAddress)

	ended, err := module.EndSupply(ctx, staff.ID)
	require.NoError(t, err)
	assert.True(t, ended)
	ended, err = module.EndSupply(ctx, staff.ID)
	require.NoError(t, err)
	assert.False(t, ended, "nothing left to end")
	require.NoError(t, module.StartSupply(ctx, second, "", staff.ID))
}

// ADR 0077: the caterer reads the orders only while it supplies the school,
// sees names only while the school shows them, and confirms a fixed order
// once.
func TestCatererReadsAndConfirmsOrdersOnlyWhileItSupplies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Lea", "Koch", "4b")
	parent := testpkg.CreateTestAccount(t, db, "meal-caterer-parent")
	caterer := testpkg.CreateTestAccount(t, db, "meal-caterer-account")
	supplier, other := createTestCaterer(t, db, "Küche am Markt"), createTestCaterer(t, db, "Andere Küche")
	require.NoError(t, buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-07", Participating: true}))

	settings := singleDeadlineSettings("09:00")
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	_, err := module.CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
	require.ErrorIs(t, err, mealplan.ErrNotSupplied, "no supply, no orders")
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).StartSupply(ctx, supplier, "", 0))

	week, err := module.CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, week.Days, 1)
	day := week.Days[0]
	assert.Equal(t, mealplan.OrderOrdered, day.Status)
	assert.Equal(t, 1, day.Total)
	assert.Equal(t, []mealplan.ClassCount{{SchoolClass: "4b", Count: 1}}, day.Classes)
	assert.Empty(t, day.Children, "numbers only by default")

	settings.CatererNames = true
	named, err := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)).CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, named.Days[0].Children, 1)
	assert.Equal(t, mealplan.OrderChild{FirstName: "Lea", LastName: "Koch", SchoolClass: "4b"}, named.Days[0].Children[0], "names without the child's ID")

	require.ErrorIs(t, module.ConfirmOrder(ctx, other, day.OrderID, caterer.ID), mealplan.ErrNotSupplied)
	require.ErrorIs(t, module.ConfirmOrder(ctx, supplier, day.OrderID+1_000_000, caterer.ID), mealplan.ErrOrderNotFound)
	require.NoError(t, module.ConfirmOrder(ctx, supplier, day.OrderID, caterer.ID))
	require.NoError(t, module.ConfirmOrder(ctx, supplier, day.OrderID, caterer.ID), "confirming twice keeps the first")
	week, err = module.CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.Equal(t, mealplan.OrderConfirmed, week.Days[0].Status)

	_, err = module.EndSupply(ctx, 0)
	require.NoError(t, err)
	_, err = module.CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
	require.ErrorIs(t, err, mealplan.ErrNotSupplied, "the end takes the school from the caterer at once")
}

// ADR 0077: with orders by total the caterer reads no class.
func TestCatererReadsNoClassesWhenTheSchoolOrdersTotals(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Mia", "Summe", "1a")
	parent := testpkg.CreateTestAccount(t, db, "meal-total-parent")
	supplier := createTestCaterer(t, db, "Summenküche")
	require.NoError(t, buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-07", Participating: true}))
	settings := singleDeadlineSettings("09:00")
	settings.OrderDetail = "total"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).StartSupply(ctx, supplier, "", 0))

	week, err := module.CatererOrders(ctx, supplier, "2026-09-07", "2026-09-07")
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
	former, successor := createTestCaterer(t, db, "Alte Küche"), createTestCaterer(t, db, "Neue Küche")
	register := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 4, 8, 0, 0, 0, testBerlin))
	for _, date := range []mealplan.Date{"2026-09-07", "2026-09-08"} {
		require.NoError(t, register.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: date, Participating: true}))
	}
	monday := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	require.NoError(t, buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).StartSupply(ctx, former, "", 0))
	week, err := monday.CatererOrders(ctx, former, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	mondayOrder := week.Days[0].OrderID
	_, err = monday.EndSupply(ctx, 0)
	require.NoError(t, err)
	// A takeover later on the same day must not expose its frozen order.
	takeover := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin))
	require.NoError(t, takeover.StartSupply(ctx, successor, "", 0))
	previous, err := takeover.CatererOrders(ctx, successor, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.Empty(t, previous.Days)
	require.ErrorIs(t, takeover.ConfirmOrder(ctx, successor, mondayOrder, 0), mealplan.ErrOrderNotFound)

	tuesday := buildModuleWithRules(t, db, singleDeadlineSettings("09:00"), time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	require.NoError(t, tuesday.StartSupply(ctx, successor, "", 0))
	week, err = tuesday.CatererOrders(ctx, successor, "2026-09-07", "2026-09-08")
	require.NoError(t, err)
	require.Len(t, week.Days, 1, "Monday was ordered before the supply started")
	assert.Equal(t, mealplan.Date("2026-09-08"), week.Days[0].Date)
	require.ErrorIs(t, tuesday.ConfirmOrder(ctx, successor, mondayOrder, 0), mealplan.ErrOrderNotFound)
	require.NoError(t, tuesday.ConfirmOrder(ctx, successor, week.Days[0].OrderID, 0))
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
			former, successor := createTestCaterer(t, db, "Former"), createTestCaterer(t, db, "Successor")
			settings := singleDeadlineSettings("09:00")
			before := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
			require.NoError(t, before.StartSupply(ctx, former, "", 0))
			takeover := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
			_, err := takeover.EndSupply(ctx, 0)
			require.NoError(t, err)
			require.NoError(t, takeover.StartSupply(ctx, successor, "", 0))
			settled := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin))
			week, err := settled.Orders(ctx, "2026-09-07", "2026-09-07")
			require.NoError(t, err)
			require.Len(t, week.Days, 1)
			orderID := week.Days[0].OrderID
			require.NotZero(t, orderID)
			if confirmed {
				_, err = db.NewRaw(`UPDATE schedule.meal_orders SET confirmed_at = ? WHERE tenant_id = ? AND id = ?`, time.Date(2026, 9, 7, 11, 0, 0, 0, testBerlin), testpkg.TenantIDFromContext(ctx), orderID).Exec(ctx)
				require.NoError(t, err)
			}
			visible, err := settled.CatererOrders(ctx, successor, "2026-09-07", "2026-09-07")
			require.NoError(t, err)
			assert.Empty(t, visible.Days, "the deadline preceded the new supply")
			require.ErrorIs(t, settled.ConfirmOrder(ctx, successor, orderID, 0), mealplan.ErrOrderNotFound)
		})
	}
}
