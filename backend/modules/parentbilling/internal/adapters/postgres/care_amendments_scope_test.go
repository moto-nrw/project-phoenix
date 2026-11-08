package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The changes of the review keep a Träger to its rows below row level
// security (#6197): each read names the Träger in its own statement, so an
// entry, a stored correction or a carried proposal of another Träger never
// reaches this one. Removing one filter turns its assertion red.

// seedCareAmendmentScope gives the open run of the scope a change of its
// contract and the issued invoice of the released run a correction and a
// Storno, so the next run of the month carries the correction.
func seedCareAmendmentScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope, label string) int64 {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_amendments (organization_id, run_id, care_contract_id, service,
		kind, label, amount_cents, reason) VALUES (?, ?, ?, 'care', 'add_position', ?, 500, 'Ausflug')`,
		scope.organization, scope.open, scope.contract, label)
	require.NoError(t, err)
	var invoice int64
	require.NoError(t, tx.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE organization_id = ? AND number = ?`,
		scope.organization, scope.number).Scan(ctx, &invoice))
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_corrections (organization_id, invoice_id, position, kind,
		label, amount_cents, reason, amended_at, amendment_id) VALUES (?, ?, 1, 'add_position', ?, 500, 'Ausflug', NOW(), 1)`,
		scope.organization, invoice, label)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number,
		number_year, number_sequence, issued_on, reason, total_cents) VALUES (?, ?, ?, ?, 2029, 2, DATE '2029-09-02', 'Falsch', -3700)`,
		scope.organization, scope.released, invoice, scope.number+"-S")
	require.NoError(t, err)
	return invoice
}

func TestCareAmendmentFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Änderung A")
	other := newCareScope(t, db, "Träger Änderung B")
	store := NewStore()
	september := calendar.NewDate(2029, 9, 1)
	rollback := errors.New("roll the review rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedCareScope(t, ctx, tx, own, "Träger Änderung A")
		other = seedCareScope(t, ctx, tx, other, "Träger Änderung B")
		ownInvoice := seedCareAmendmentScope(t, ctx, tx, own, "Ausflug A")
		otherInvoice := seedCareAmendmentScope(t, ctx, tx, other, "Ausflug B")

		entries, err := store.CareRunAmendments(ctx, own.organization, other.open)
		require.NoError(t, err)
		assert.Empty(t, entries, "CareRunAmendments of another Träger's run")
		entries, err = store.CareRunAmendments(ctx, own.organization, own.open)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "Ausflug A", entries[0].Label)

		carried, err := store.CarriedCareCorrections(ctx, own.organization, september)
		require.NoError(t, err)
		require.Len(t, carried, 1, "CarriedCareCorrections carries the own Storno only")
		proposal := carried[parentbilling.CareInvoiceKey{ContractID: own.contract, Service: parentbilling.ServiceCare}]
		require.Len(t, proposal, 1)
		assert.Equal(t, own.number, proposal[0].CarriedFromNumber)

		corrections, err := careCorrectionsOf(ctx, tx, own.organization, []int64{ownInvoice, otherInvoice})
		require.NoError(t, err)
		assert.Len(t, corrections, 1, "careCorrectionsOf reads the own invoice only")
		assert.Len(t, corrections[ownInvoice], 1)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
