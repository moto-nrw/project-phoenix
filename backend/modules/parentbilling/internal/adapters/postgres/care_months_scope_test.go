package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The monthly contribution's own filters keep a Träger to its rows also
// below row level security (#4684): the Träger transaction drops its
// request role (testpkg.WithoutRequestRole), so every assertion here holds
// only because the adapter names the Träger in its own statement. Removing
// one filter turns its assertion red.

// careScope is one Träger with a school and every kind of monthly row: a
// price, a released run with an invoice, a position and a held child, a run
// in the preview and the number of its invoice.
type careScope struct {
	organization int64
	school       int64
	released     int64
	open         int64
	contract     int64
	number       string
	holiday      string
}

// newCareScope makes the Träger and its school; their fixtures remove them.
func newCareScope(t *testing.T, db *bun.DB, name string) careScope {
	t.Helper()
	unique := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: fmt.Sprintf("care-scope-%d", unique)}).ID
	return careScope{
		organization: organization, school: testpkg.CreateTestSchoolInOrganization(t, db, organization, name),
		contract: unique, number: fmt.Sprintf("MB-%d", unique), holiday: fmt.Sprintf("FB-%d", unique),
	}
}

// seedCareScope writes the monthly rows of the Träger in tx, which the test
// rolls back.
func seedCareScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope, name string) careScope {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_tariffs (organization_id, school_id, care_offering_id, module_name,
		weekday, valid_from, monthly_cents) VALUES (?, ?, 101, 'Betreuung bis 14:00 Uhr', 'mon', DATE '2029-09-01', 3200)`,
		scope.organization, scope.school)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at,
		issued_on, document_count, total_cents, month_rule) VALUES (?, DATE '2029-09-01', 'released', NOW(), DATE '2029-09-01', 1, 3200,
		'version_on_first') RETURNING id`, scope.organization).Scan(ctx, &scope.released))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month)
		VALUES (?, DATE '2029-10-01') RETURNING id`, scope.organization).Scan(ctx, &scope.open))
	var invoice int64
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
		number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
		recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, ?, 2029, 1, DATE '2029-09-01', DATE '2029-09-01', ?, ?, ?, 'Mia', 'Beispiel', 'Eva', 'Beispiel', '',
		'version_on_first', 3200) RETURNING id`, scope.organization, scope.released, scope.number, scope.contract, scope.school, name).
		Scan(ctx, &invoice))
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_positions (organization_id, invoice_id, position,
		care_offering_id, module_name, contract_version_number, service_from, service_until, weekdays, amount_cents, net_cents)
		VALUES (?, ?, 1, 101, 'Betreuung bis 14:00 Uhr', 1, DATE '2029-09-01', DATE '2029-09-30',
		'[{"weekday":"mon","monthly_cents":3200,"amount_cents":3200}]'::jsonb, 3200, 3200)`, scope.organization, invoice)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_holds (organization_id, run_id, position, reason, school_ids,
		child_first_name, child_last_name, contracts, slots)
		VALUES (?, ?, 1, 'duplicate_contract', ARRAY[?]::bigint[], 'Noah', 'Beispiel',
			jsonb_build_array(jsonb_build_object('contract_id', ?::bigint, 'school_id', ?::bigint, 'school_name', ?::text)),
			'[{"module_name":"Betreuung bis 14:00 Uhr","weekday":"mon"}]'::jsonb)`,
		scope.organization, scope.released, scope.school, scope.contract+1, scope.school, name)
	require.NoError(t, err)
	// A holiday document with the Träger's holiday number. Its run, period
	// and registration play no part in the number check; replica mode skips
	// their keys for this one row.
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
		number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
		recipient_email, service_from, service_until, total_cents)
		VALUES (?, 1, 'holiday', ?, 2029, 1, DATE '2029-09-01', 1, 1, 'Eva', 'Beispiel', '', DATE '2029-09-01', DATE '2029-09-05', 1000)`,
		scope.organization, scope.holiday)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
	require.NoError(t, err)
	return scope
}

func TestCareMonthsFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger A")
	other := newCareScope(t, db, "Träger B")
	store := NewStore()
	september := calendar.NewDate(2029, 9, 1)
	rollback := errors.New("roll the monthly rows back")

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

		tariffs, err := store.CareTariffs(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, tariffs, 1, "CareTariffs")
		assert.Equal(t, own.school, tariffs[0].SchoolID)

		billed, err := store.CareBilledUntil(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, map[int64]calendar.Date{own.school: september}, billed, "CareBilledUntil")

		prices, err := store.CareBilledPrices(ctx, own.organization, other.school)
		require.NoError(t, err)
		assert.Empty(t, prices, "CareBilledPrices of another Träger's school")

		runs, err := store.ListCareRuns(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, runs, 2, "ListCareRuns")
		assert.ElementsMatch(t, []int64{own.released, own.open}, []int64{runs[0].ID, runs[1].ID})

		_, err = store.CareRun(ctx, own.organization, other.open, false)
		assert.ErrorIs(t, err, parentbilling.ErrCareRunNotFound, "CareRun of another Träger")
		_, err = store.CareRun(ctx, own.organization, other.open, true)
		assert.ErrorIs(t, err, parentbilling.ErrCareRunNotFound, "CareRun for update of another Träger")

		invoiced, err := store.InvoicedContracts(ctx, own.organization, september)
		require.NoError(t, err)
		assert.Equal(t, map[int64]bool{own.contract: true}, invoiced, "InvoicedContracts")

		_, err = store.MarkCareRunReleased(ctx, own.organization, other.open, september, 1, 3200, parentbilling.MonthRuleVersionOnFirst)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "MarkCareRunReleased of another Träger's run")

		invoices, err := store.CareRunInvoices(ctx, own.organization, other.released)
		require.NoError(t, err)
		assert.Empty(t, invoices, "CareRunInvoices of another Träger's run")

		holds, err := store.CareRunHolds(ctx, own.organization, other.released)
		require.NoError(t, err)
		assert.Empty(t, holds, "CareRunHolds of another Träger's run")

		deleted, err := store.DeleteCarePrices(ctx, own.organization, other.school, september)
		require.NoError(t, err)
		assert.Zero(t, deleted, "DeleteCarePrices of another Träger's school")

		assert.NoError(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{other.number}, careNumbersTable),
			"another Träger's monthly number is free here")
		assert.NoError(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{other.holiday}, holidayNumbersTable),
			"another Träger's holiday number is free here")
		assert.True(t, errors.Is(claimInvoiceNumbers(ctx, tx, own.organization, []string{own.number}, careNumbersTable),
			parentbilling.ErrInvoiceNumberTaken), "the own monthly number is taken")
		assert.True(t, errors.Is(claimInvoiceNumbers(ctx, tx, own.organization, []string{own.holiday}, holidayNumbersTable),
			parentbilling.ErrInvoiceNumberTaken), "the own holiday number is taken")

		var status string
		require.NoError(t, tx.NewRaw(`SELECT status FROM parent_billing.care_runs WHERE id = ?`, other.open).Scan(ctx, &status))
		assert.Equal(t, parentbilling.RunCalculated, status, "the other Träger's run stays in the preview")
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.care_tariffs WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its price")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
