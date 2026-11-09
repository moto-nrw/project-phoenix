package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// An invoice the release issued with the reason mandate_expired (#4697)
// does not stop the rollback: Down maps it to the older reason and restores
// the older constraint, and Up applies again on top.
func TestSEPADebitsMigrationRollsBackAnExpiredMandateInvoice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-lastschrift-down-%d", testpkg.UniqueTestTenantID(t))
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Lastschrift Down", Slug: slug}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Lastschrift Down OGS")
	var run, invoice int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, DATE '2029-10-01')
		RETURNING id`, own).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
		number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
		recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents, payment_method, due_on, due_source,
		debit_gap)
		VALUES (?, ?, 'MB-DOWN-1', 2029, 1, DATE '2029-10-30', DATE '2029-10-01', ?, ?, 'OGS', 'Mia', 'Beispiel', 'Eva', 'Beispiel', '',
		'version_on_first', 3700, 'transfer', DATE '2029-11-13', 'run', 'mandate_expired') RETURNING id`,
		own, run, testpkg.UniqueTestTenantID(t), school).Scan(ctx, &invoice))

	require.NoError(t, parentBillingSEPADebitsDown(ctx, db), "an invoice with mandate_expired must not block the rollback")
	var gap string
	require.NoError(t, db.NewRaw(`SELECT debit_gap FROM parent_billing.care_invoices WHERE id = ?`, invoice).Scan(ctx, &gap))
	assert.Equal(t, "mandate_missing", gap)
	_, err := db.ExecContext(ctx, `UPDATE parent_billing.care_invoices SET total_cents = 1 WHERE id = ?`, invoice)
	require.Error(t, err, "the guard of the issued invoices is on again")
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('parent_billing.sepa_debits') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists)

	require.NoError(t, parentBillingSEPADebitsUp(ctx, db))
	require.NoError(t, db.NewRaw(`SELECT to_regclass('parent_billing.sepa_debits') IS NOT NULL`).Scan(ctx, &exists))
	assert.True(t, exists)
}
