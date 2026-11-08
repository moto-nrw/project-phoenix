package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The price forms of #6196 below row level security: the heads of the price
// lists, the scale steps and the billed scale steps stay with their Träger
// only because the adapter names it in each statement.

func scalePriceList(school int64, from calendar.Date) parentbilling.CarePriceList {
	sibling := int64(4500)
	return parentbilling.CarePriceList{SchoolID: school, ValidFrom: from, FreeMonths: []int{8}, Prices: []parentbilling.CarePrice{
		{CareOfferingID: 704, ModuleName: "Modul 4 mit Mittagessen", DaysPerWeek: new(3), MonthlyCents: 6000, SiblingCents: &sibling, MealExtra: true},
		{CareOfferingID: 701, ModuleName: "Modul 1", Weekday: "mon", MonthlyCents: 650},
	}}
}

func TestCarePriceListsFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Preisformen A")
	other := newCareScope(t, db, "Träger Preisformen B")
	store := NewStore()
	account := testpkg.CreateTestAccount(t, db, "preisformen@example.org").ID
	january := calendar.NewDate(2030, 1, 1)
	rollback := errors.New("roll the price lists back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		require.NoError(t, store.ReplaceCarePrices(ctx, own.organization, account, scalePriceList(own.school, january)))
		// The other Träger's list, written past the adapter's guard.
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_price_lists (organization_id, school_id, valid_from, free_months)
			VALUES (?, ?, ?, '{1}')`, other.organization, other.school, january)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_tariffs (organization_id, school_id, care_offering_id, module_name,
			days_per_week, valid_from, monthly_cents) VALUES (?, ?, 704, 'Modul 4', 3, ?, 6000)`, other.organization, other.school, january)
		require.NoError(t, err)

		heads, err := store.CarePriceListHeads(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, heads, 1, "CarePriceListHeads")
		assert.Equal(t, parentbilling.CarePriceListHead{SchoolID: own.school, ValidFrom: january, FreeMonths: []int{8}}, heads[0])

		tariffs, err := store.CareTariffs(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, tariffs, 2, "CareTariffs")
		byOffering := map[int64]parentbilling.CareTariff{}
		for _, tariff := range tariffs {
			byOffering[tariff.CareOfferingID] = tariff
		}
		scale := byOffering[704]
		require.NotNil(t, scale.DaysPerWeek)
		assert.Equal(t, 3, *scale.DaysPerWeek)
		assert.Empty(t, scale.Weekday)
		assert.Equal(t, int64(4500), *scale.SiblingCents)
		assert.True(t, scale.MealExtra)
		assert.Equal(t, "mon", byOffering[701].Weekday)
		assert.Nil(t, byOffering[701].SiblingCents)

		// A delete names the Träger: another Träger's school keeps its list.
		deleted, err := store.DeleteCarePrices(ctx, own.organization, other.school, january)
		require.NoError(t, err)
		assert.Zero(t, deleted, "DeleteCarePrices of another Träger's school")
		var otherRows int
		require.NoError(t, tx.NewRaw(`SELECT (SELECT count(*) FROM parent_billing.care_price_lists WHERE organization_id = ?)
			+ (SELECT count(*) FROM parent_billing.care_tariffs WHERE organization_id = ?)`, other.organization, other.organization).
			Scan(ctx, &otherRows))
		assert.Equal(t, 2, otherRows, "the other Träger keeps its head and price")
		deleted, err = store.DeleteCarePrices(ctx, own.organization, own.school, january)
		require.NoError(t, err)
		assert.Equal(t, int64(len(scalePriceList(own.school, january).Prices)), deleted)
		heads, err = store.CarePriceListHeads(ctx, own.organization)
		require.NoError(t, err)
		assert.Empty(t, heads, "the head goes with its prices")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A billed position of a scale names its step, not its weekdays, so a
// later price list keeps exactly that step.
func TestCareBilledPricesNameTheScaleStep(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Staffel abgerechnet")
	store := NewStore()
	rollback := errors.New("roll the invoice back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var run, invoice int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at,
			issued_on, document_count, total_cents, month_rule) VALUES (?, DATE '2030-01-01', 'released', NOW(), DATE '2030-01-01', 1, 6000,
			'version_on_first') RETURNING id`, own.organization).Scan(ctx, &run))
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
			VALUES (?, ?, ?, 2030, 1, DATE '2030-01-01', DATE '2030-01-01', ?, ?, 'Birkenfeld', 'Klara', 'Beispiel', 'Eva', 'Beispiel', '',
			'version_on_first', 6000) RETURNING id`, own.organization, run, own.number, own.contract, own.school).Scan(ctx, &invoice))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_positions (organization_id, invoice_id, position,
			care_offering_id, module_name, contract_version_number, service_from, service_until, weekdays, amount_cents, net_cents,
			days_per_week, tier_monthly_cents, meal_extra)
			VALUES (?, ?, 1, 704, 'Modul 4', 1, DATE '2030-01-01', DATE '2030-01-31',
			'[{"weekday":"mon","monthly_cents":2000,"amount_cents":2000},{"weekday":"tue","monthly_cents":2000,"amount_cents":2000},{"weekday":"wed","monthly_cents":2000,"amount_cents":2000}]'::jsonb,
			6000, 6000, 3, 6000, true)`, own.organization, invoice)
		require.NoError(t, err)

		billed, err := store.CareBilledPrices(ctx, own.organization, own.school)
		require.NoError(t, err)
		require.Len(t, billed, 1, "one step, not three weekdays")
		require.NotNil(t, billed[0].DaysPerWeek)
		assert.Equal(t, 3, *billed[0].DaysPerWeek)
		assert.Empty(t, billed[0].Weekday)

		invoices, err := store.CareRunInvoices(ctx, own.organization, run)
		require.NoError(t, err)
		require.Len(t, invoices, 1)
		position := invoices[0].Positions[0]
		require.NotNil(t, position.DaysPerWeek)
		assert.Equal(t, 3, *position.DaysPerWeek)
		assert.Equal(t, int64(6000), *position.TierMonthlyCents)
		assert.True(t, position.MealExtra)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The one statement the price list and the monthly run read (#6196) joins
// the heads with the prices. Each side names the Träger in its own filter:
// below row level security another Träger's head and price, at the same
// month, never join in. Dropping either filter turns this red.
func TestCarePriceListsJoinKeepsTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Preisbuch A")
	other := newCareScope(t, db, "Träger Preisbuch B")
	store := NewStore()
	january := calendar.NewDate(2030, 1, 1)
	rollback := errors.New("roll the price lists back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		for _, scope := range []careScope{own, other} {
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_price_lists (organization_id, school_id, valid_from, free_months)
				VALUES (?, ?, ?, '{8}')`, scope.organization, scope.school, january)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_tariffs (organization_id, school_id, care_offering_id, module_name,
				days_per_week, valid_from, monthly_cents, sibling_cents) VALUES (?, ?, 704, 'Modul 4', 3, ?, 6000, 4500)`,
				scope.organization, scope.school, january)
			require.NoError(t, err)
		}
		// A price of the other Träger without a head of its own would join as
		// a price without a head if the price side lost its filter.
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_tariffs (organization_id, school_id, care_offering_id, module_name,
			weekday, valid_from, monthly_cents) VALUES (?, ?, 701, 'Modul 1', 'mon', DATE '2030-02-01', 650)`,
			other.organization, other.school)
		require.NoError(t, err)

		tariffs, heads, err := store.CarePriceLists(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, tariffs, 1, "CarePriceLists prices")
		assert.Equal(t, own.school, tariffs[0].SchoolID)
		assert.Equal(t, int64(4500), *tariffs[0].SiblingCents)
		require.Len(t, heads, 1, "CarePriceLists heads")
		assert.Equal(t, parentbilling.CarePriceListHead{SchoolID: own.school, ValidFrom: january, FreeMonths: []int{8}}, heads[0])
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
