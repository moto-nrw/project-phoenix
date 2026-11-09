package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCareAmendmentsErasureExtendsTheErasureOf811 keeps the erasure of
// 1.15.821 the erasure of 1.15.811 with the deletion of the changes of the
// review, before the contracts they name go.
func TestCareAmendmentsErasureExtendsTheErasureOf811(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor,
		careAmendmentsErasureStatement+careAmendmentsErasureAnchor, 1), demoVisitorErasureCareAmendmentsSQL)
}

// TestCareInvoiceCorrectionsCloseWithTheRelease pins ADR 0119 point 7 on the
// corrections of a monthly invoice (#6197): the request role may append a
// correction only while the run of its invoice is calculated, as the
// release does; once the run is released, no correction is appended,
// changed or deleted, also not by the migration role. Clearing the account
// and the cascade from the invoice stay possible.
func TestCareInvoiceCorrectionsCloseWithTheRelease(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-korrektur-%d", testpkg.UniqueTestTenantID(t))
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Korrektur", Slug: slug}).ID
	testpkg.OwnTraegerCareContracts(t, db, own)
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Korrektur OGS")
	invoiceOf := func(status string, sequence int) int64 {
		var run, invoice int64
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, DATE '2029-10-01')
			RETURNING id`, own).Scan(ctx, &run))
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
			VALUES (?, ?, ?, 2029, ?, DATE '2029-10-30', DATE '2029-10-01', ?, ?, 'OGS', 'Mia', 'Beispiel', 'Eva', 'Beispiel', '',
			'version_on_first', 3700) RETURNING id`, own, run, fmt.Sprintf("MB-%d-%d", own, sequence), sequence,
			testpkg.UniqueTestTenantID(t), school).Scan(ctx, &invoice))
		if status == "released" {
			_, err := db.ExecContext(ctx, `UPDATE parent_billing.care_runs SET status = 'released', released_at = NOW(),
				issued_on = DATE '2029-10-30', document_count = 1, total_cents = 3700, month_rule = 'version_on_first' WHERE id = ?`, run)
			require.NoError(t, err)
		}
		return invoice
	}
	asRequestRole := func(statement string, args ...any) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(own)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, statement, args...)
			return err
		})
	}
	const insert = `INSERT INTO parent_billing.care_invoice_corrections (organization_id, invoice_id, position, kind, label,
		amount_cents, reason, amended_at, amendment_id) VALUES (?, ?, ?, 'add_position', 'Ausflug', 500, 'Elternbrief', NOW(), 1)`

	open := invoiceOf("calculated", 1)
	require.NoError(t, asRequestRole(insert, own, open, 1), "the release appends a correction while its run is calculated")
	issued := invoiceOf("released", 2)
	err := asRequestRole(insert, own, issued, 1)
	require.Error(t, err, "the request role appends no correction to an issued invoice")
	assert.ErrorContains(t, err, "never changes")
	_, err = db.ExecContext(ctx, insert, own, issued, 1)
	assert.ErrorContains(t, err, "never changes", "no role appends a correction to an issued invoice")

	_, err = db.ExecContext(ctx, `UPDATE parent_billing.care_runs SET status = 'released', released_at = NOW(),
		issued_on = DATE '2029-10-30', document_count = 1, total_cents = 3700, month_rule = 'version_on_first'
		WHERE id = (SELECT run_id FROM parent_billing.care_invoices WHERE id = ?)`, open)
	require.NoError(t, err)
	for _, statement := range []string{
		`UPDATE parent_billing.care_invoice_corrections SET amount_cents = 900 WHERE invoice_id = ?`,
		`DELETE FROM parent_billing.care_invoice_corrections WHERE invoice_id = ?`,
	} {
		_, err = db.ExecContext(ctx, statement, open)
		assert.ErrorContains(t, err, "never changes", "an issued correction never changes: %s", statement)
	}
	_, err = db.ExecContext(ctx, `DELETE FROM parent_billing.care_invoices WHERE id = ?`, open)
	require.NoError(t, err, "the correction goes with its invoice")
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoice_corrections WHERE invoice_id = ?`, open).Scan(ctx, &left))
	assert.Zero(t, left)
}
