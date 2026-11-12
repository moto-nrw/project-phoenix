package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The Mensa's own filters keep a Träger to its rows below row level
// security (#4686), as care_months_scope_test.go does for the monthly rows:
// the request role is dropped, so every assertion holds only because the
// adapter names the Träger in its statement.

// seedMealScope writes a Mensa price and a Mensa position on the Träger's
// issued invoice of September 2029.
func seedMealScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_tariffs (organization_id, school_id, valid_from, mode, amount_cents)
		VALUES (?, ?, DATE '2029-08-01', 'per_meal', 480)`, scope.organization, scope.school)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id,
		student_id, meal_month, mode, quantity, unit_cents, amount_cents, net_cents)
		SELECT organization_id, id, 1, school_id, 9501, DATE '2029-08-01', 'per_meal', 2, 480, 960, 960
		FROM parent_billing.care_invoices WHERE organization_id = ? AND run_id = ?`, scope.organization, scope.released)
	require.NoError(t, err)
}

func TestCareMealFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger A")
	other := newCareScope(t, db, "Träger B")
	store := NewStore()
	august := calendar.NewDate(2029, 8, 1)
	rollback := errors.New("roll the Mensa rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own = seedCareScope(t, ctx, tx, own, "Träger A")
		other = seedCareScope(t, ctx, tx, other, "Träger B")
		seedMealScope(t, ctx, tx, own)
		seedMealScope(t, ctx, tx, other)

		tariffs, err := store.CareMealTariffs(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, tariffs, 1, "CareMealTariffs")
		assert.Equal(t, own.school, tariffs[0].SchoolID)

		billed, err := store.BilledMealMonths(ctx, own.organization, august, august)
		require.NoError(t, err)
		require.Len(t, billed, 1, "BilledMealMonths")
		assert.Equal(t, parentbilling.CareBilledMeal{SchoolID: own.school, StudentID: 9501, Month: august}, billed[0].CareBilledMeal, "BilledMealMonths")
		assert.Equal(t, int64(960), billed[0].AmountCents)
		assert.Equal(t, 1, billed[0].ReleaseVersion)

		until, err := store.MealBilledUntil(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, map[int64]calendar.Date{own.school: august}, until, "MealBilledUntil")

		invoices, err := store.CareRunInvoices(ctx, own.organization, own.released)
		require.NoError(t, err)
		require.Len(t, invoices, 1)
		_, history, _, err := store.InvoiceHistory(ctx, own.organization, parentbilling.HistoryCareInvoice, invoices[0].ID)
		require.NoError(t, err)
		require.NotEmpty(t, history)
		assert.Equal(t, len(invoices[0].Positions)+len(invoices[0].MealPositions), history[0].Positions, "combined invoices count care and Mensa positions")
		assert.Equal(t, []parentbilling.CareMealPosition{{SchoolID: own.school, StudentID: 9501, MealMonth: august,
			Mode: parentbilling.MealModePerMeal, Quantity: 2, UnitCents: 480, AmountCents: 960, ReleaseVersion: 1}}, invoices[0].MealPositions,
			"CareRunInvoices reads the invoice's own Mensa months")

		// A Mensa-only invoice bills its contract: a later run of the month
		// leaves the contract out instead of a second invoice.
		var mensaOnly int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
			VALUES (?, ?, ?, 2029, 2, DATE '2029-09-01', DATE '2029-09-01', ?, ?, 'Schule', 'Ben', 'Beispiel', 'Eva', 'Beispiel', '',
			'version_on_first', 960) RETURNING id`, own.organization, own.released, own.number+"-M", own.contract+7, own.school).Scan(ctx, &mensaOnly))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id,
			student_id, meal_month, mode, quantity, unit_cents, amount_cents, net_cents)
			VALUES (?, ?, 1, ?, 9502, DATE '2029-08-01', 'per_meal', 2, 480, 960, 960)`, own.organization, mensaOnly, own.school)
		require.NoError(t, err)
		invoiced, err := store.InvoicedContracts(ctx, own.organization, calendar.NewDate(2029, 9, 1))
		require.NoError(t, err)
		require.Contains(t, invoiced, own.contract+7, "a Mensa-only invoice bills its contract")
		assert.Empty(t, invoiced[own.contract+7].Positions)
		assert.Equal(t, int64(960), invoiced[own.contract+7].TotalCents)

		deleted, err := store.DeleteCareMealTariff(ctx, own.organization, other.school, august)
		require.NoError(t, err)
		assert.Zero(t, deleted, "DeleteCareMealTariff of another Träger's school")
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.care_meal_tariffs WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its Mensa price")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
