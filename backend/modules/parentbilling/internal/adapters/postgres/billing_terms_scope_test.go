package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The terms, deviations and due dates keep a Träger to its rows also below
// row level security (#4689): the Träger transaction drops its request role,
// so every assertion holds only because the adapter names the Träger in its
// own statement. Removing one filter turns its assertion red.

type termsScope struct {
	organization int64
	family       string
	familyID     int64
	openRun      int64
	releasedRun  int64
}

func newTermsScope(t *testing.T, db *bun.DB, name string) termsScope {
	t.Helper()
	unique := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: fmt.Sprintf("terms-scope-%d", unique)}).ID
	return termsScope{organization: organization, family: fmt.Sprintf("familie.%d@example.org", unique)}
}

// seedTermsScope writes the Träger's terms, a deviation, a run in the
// preview with its due dates and a released run.
func seedTermsScope(t *testing.T, ctx context.Context, tx bun.IDB, scope termsScope, method string, due calendar.Date) termsScope {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.service_terms (organization_id, service, payment_method, due_days)
		VALUES (?, 'care', ?, 7)`, scope.organization, method)
	require.NoError(t, err)
	// The deviation hangs on a contract; its key plays no part here.
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.family_billing (organization_id, family_email, care_contract_id,
		recipient_kind, payment_method) VALUES (?, ?, 1, 'applicant', ?) RETURNING id`, scope.organization, scope.family, method).
		Scan(ctx, &scope.familyID))
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month)
		VALUES (?, DATE '2029-10-01') RETURNING id`, scope.organization).Scan(ctx, &scope.openRun))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at,
		issued_on, document_count, total_cents, month_rule) VALUES (?, DATE '2029-09-01', 'released', NOW(), DATE '2029-09-01', 1, 0,
		'version_on_first') RETURNING id`, scope.organization).Scan(ctx, &scope.releasedRun))
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.run_due_dates (organization_id, care_run_id, payment_method, due_on)
		VALUES (?, ?, ?, ?)`, scope.organization, scope.openRun, method, due)
	require.NoError(t, err)
	return scope
}

func TestBillingTermsFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newTermsScope(t, db, "Träger A")
	other := newTermsScope(t, db, "Träger B")
	store := NewStore()
	ownDue, otherDue := calendar.NewDate(2029, 11, 4), calendar.NewDate(2029, 11, 2)
	rollback := errors.New("roll the terms back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own = seedTermsScope(t, ctx, tx, own, parentbilling.PaymentDirectDebit, ownDue)
		other = seedTermsScope(t, ctx, tx, other, parentbilling.PaymentTransfer, otherDue)

		terms, err := store.ServiceTerms(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, terms, 1, "ServiceTerms")
		assert.Equal(t, parentbilling.PaymentDirectDebit, terms[0].PaymentMethod)

		families, err := store.FamilyBillings(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, families, 1, "FamilyBillings")
		assert.Equal(t, own.family, families[0].FamilyEmail)

		combined, combinedFamilies, dates, err := store.InvoicePaymentTerms(ctx, own.organization, own.openRun, 0)
		require.NoError(t, err)
		require.Len(t, combined, 1, "InvoicePaymentTerms: terms")
		assert.Equal(t, parentbilling.PaymentDirectDebit, combined[0].PaymentMethod)
		require.Len(t, combinedFamilies, 1, "InvoicePaymentTerms: families")
		assert.Equal(t, own.family, combinedFamilies[0].FamilyEmail)
		assert.Equal(t, parentbilling.RunDueDates{parentbilling.PaymentDirectDebit: ownDue}, dates)
		_, _, dates, err = store.InvoicePaymentTerms(ctx, own.organization, other.openRun, 0)
		require.NoError(t, err)
		assert.Empty(t, dates, "InvoicePaymentTerms: the due dates of another Träger's run")

		removed, err := store.DeleteFamilyBilling(ctx, own.organization, other.familyID, "")
		require.NoError(t, err)
		assert.Nil(t, removed, "DeleteFamilyBilling of another Träger's deviation by ID")
		removed, err = store.DeleteFamilyBilling(ctx, own.organization, 0, other.family)
		require.NoError(t, err)
		assert.Nil(t, removed, "DeleteFamilyBilling of another Träger's family")

		require.NoError(t, store.ReplaceRunDueDates(ctx, own.organization, 0, other.openRun, 0, nil),
			"ReplaceRunDueDates names the Träger")
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.run_due_dates WHERE care_run_id = ?`, other.openRun).Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its due date")
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its deviation")

		// A released run never gets a due date, also below row level
		// security.
		_, err = tx.ExecContext(ctx, `SAVEPOINT released`)
		require.NoError(t, err)
		err = store.ReplaceRunDueDates(ctx, own.organization, 0, own.releasedRun, 0,
			parentbilling.RunDueDates{parentbilling.PaymentTransfer: ownDue})
		require.ErrorContains(t, err, "never change", "the due dates of a released run")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT released`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
