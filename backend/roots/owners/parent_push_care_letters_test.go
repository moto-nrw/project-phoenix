package owners

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// careLetterPush turns the fixture's push into the push of a sent monthly
// letter (#4684, ADR 0165) of the chain's child at its school. studentID is
// the child the invoice copied.
func (f *billingPushFixture) careLetterPush(t *testing.T, chain testpkg.ParentChain, studentID int64) {
	t.Helper()
	var run, invoice int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at, issued_on,
		document_count, total_cents, month_rule) VALUES (?, DATE '2026-10-01', 'released', NOW(), DATE '2026-10-01', 1, 3200, 'version_on_first')
		RETURNING id`, f.organization).Scan(f.ctx, &run))
	require.NoError(t, f.db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence,
		issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
		recipient_last_name, recipient_email, month_rule, total_cents, student_id)
		VALUES (?, ?, 'MB-1', 2026, 1, DATE '2026-10-01', DATE '2026-10-01', ?, ?, 'OGS', 'Mia', 'Test', 'Test', 'Family', ?,
		'version_on_first', 3200, ?) RETURNING id`, f.organization, run, testpkg.UniqueTestTenantID(t), chain.TenantID, chain.Email, studentID).
		Scan(f.ctx, &invoice))
	_, err := f.db.ExecContext(f.ctx, `INSERT INTO parent_billing.care_run_letters (organization_id, run_id, payment_due_on)
		VALUES (?, ?, DATE '2026-10-15')`, f.organization, run)
	require.NoError(t, err)
	require.NoError(t, f.db.NewRaw(`INSERT INTO parent_billing.care_letters (organization_id, run_id, invoice_id, kind, filename, pdf)
		VALUES (?, ?, ?, 'invoice', 'rechnung.pdf', ?) RETURNING id`, f.organization, run, invoice, []byte("%PDF-test")).Scan(f.ctx, &f.document))
	_, err = f.db.ExecContext(f.ctx, `INSERT INTO parent_billing.care_letter_mailings (organization_id, run_id, letter_id, delivery_id,
		recipient_email) VALUES (?, ?, ?, ?, ?)`, f.organization, run, f.document, f.document+900000, chain.Email)
	require.NoError(t, err)
}

// A monthly letter pushes through the object check of #6138 like a holiday
// letter, and only while it is current: for the child of its invoice, whose
// relationship grants the account the inbox, to the address the invoice
// copied, while the invoice is not cancelled.
func TestCareLetterDevicePushRechecksTheInvoiceAndItsChild(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, scenario := range []string{"valid", "another child", "right revoked", "cancelled invoice", "recipient changed"} {
		t.Run(scenario, func(t *testing.T) {
			f, chain := newBillingPushFixture(t)
			student := chain.StudentID
			if scenario == "another child" {
				other := testpkg.CreateTestStudentForTenant(t, f.db, chain.TenantID, "Other", "Child", "2a")
				student = other.ID
			}
			f.careLetterPush(t, chain, student)
			f.fanout(t, chain, 2)
			switch scenario {
			case "right revoked":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.enrollments.view'
					WHERE tenant_id = ? AND relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?)`,
					chain.TenantID, chain.StudentID, chain.GuardianProfileID)
				require.NoError(t, err)
			case "cancelled invoice":
				_, err := f.db.ExecContext(f.ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id,
					number, number_year, number_sequence, issued_on, reason, total_cents)
					SELECT organization_id, run_id, id, 'MB-2', 2026, 2, issued_on, 'Test', -total_cents FROM parent_billing.care_invoices
					WHERE id = (SELECT invoice_id FROM parent_billing.care_letters WHERE id = ?)`, f.document)
				require.NoError(t, err)
			case "recipient changed":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.accounts SET email = ? WHERE id = ?`, fmt.Sprintf("changed-%d@example.org", chain.AccountID), chain.AccountID)
				require.NoError(t, err)
			}
			f.runWorker(t)
			if scenario == "valid" {
				assert.Equal(t, []string{"sent", "sent"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				return
			}
			assert.Equal(t, []string{"cancelled", "cancelled"}, f.states(t))
			assert.Zero(t, f.transport.calls)
		})
	}
}
