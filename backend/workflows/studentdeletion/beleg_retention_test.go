// The Belege of a Träger survive every operative deletion (#4720 slice 1,
// legal Löschkonzept section 4): deleting a child through the deletion
// workflow, deleting its guardian contact and erasing the unused account of
// that guardian leave every contract, invoice, correction, cancellation,
// notice, SEPA mandate and debit of the child unchanged. The Belege keep
// their own copy of the names they need, so they hold no reference that
// would keep the child profile alive.
package studentdeletion_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// belegTables are the tables of the Belege the fixture fills for one child.
var belegTables = []string{
	"care_contract.contracts", "care_contract.contract_versions", "care_contract.contract_parties",
	"parent_billing.sepa_mandates", "parent_billing.sepa_mandate_events", "parent_billing.sepa_debits",
	"parent_billing.care_runs", "parent_billing.care_invoices", "parent_billing.care_invoice_corrections",
	"parent_billing.care_invoice_cancellations", "parent_billing.care_notice_runs", "parent_billing.care_notices",
}

// seedBelege writes the Belege of one child and its guardian account as the
// releases write them: the contract with its version and party, a monthly
// invoice with a correction and a cancellation, a released Jahresbescheid,
// a SEPA mandate with its trail and the debit of the invoice.
func seedBelege(t *testing.T, ctx context.Context, db *bun.DB, organization, studentID, guardianAccountID int64, family string) {
	t.Helper()
	school := testpkg.Tenant(t)
	const creditor = "DE98ZZZ09999999999"
	var contract, version, mandate, run, invoice, noticeRun int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, student_id,
			status, sent_at, concluded_at, conclusion_method)
		VALUES (?, ?, ?, ?, 'aktiv', NOW(), NOW(), 'papier') RETURNING id`,
		organization, school, testpkg.UniqueTestTenantID(t), studentID).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, 'wirksam', DATE '2029-05-01', 'Lia', 'Beispiel', 'OGS', 'open_ended', 1, 'month_end',
			false, '', false, '', '', 'SJR Pforzheim') RETURNING id`, organization, contract).Scan(ctx, &version))
	_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_parties (organization_id, contract_id, first_name, last_name,
			email, account_id) VALUES (?, ?, 'Eva', 'Beispiel', ?, ?)`, organization, contract, family, guardianAccountID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.sepa_creditors (organization_id, creditor_id) VALUES (?, ?)`,
		organization, creditor)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
			signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, ?, ?, 'SJR-BELEG-1', DATE '2026-09-01', 'recurring', 'first', ?, '\x02'::bytea, '\x01'::bytea, 'DE•• 30 00', 'office')
		RETURNING id`, organization, family, contract, creditor).Scan(ctx, &mandate))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_events (organization_id, mandate_id, care_contract_id,
			action, source, actor_account_id) VALUES (?, ?, ?, 'create', 'parent', ?)`, organization, mandate, contract, guardianAccountID)
	require.NoError(t, err)

	// A correction is added while the run is calculated; the release then
	// fixes the invoice.
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, DATE '2029-06-01')
		RETURNING id`, organization).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			student_id, recipient_first_name, recipient_last_name, recipient_email, family_email, month_rule, total_cents)
		VALUES (?, ?, 'MB-BELEG-1', 2029, 1, DATE '2029-05-20', DATE '2029-06-01', ?, ?, 'OGS', 'Lia', 'Beispiel', ?,
			'Eva', 'Beispiel', ?, ?, 'pro_rata', 7750) RETURNING id`,
		organization, run, contract, school, studentID, family, family).Scan(ctx, &invoice))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_corrections (organization_id, invoice_id, position, kind,
			label, amount_cents, reason, account_id, amended_at, carried_from_number)
		VALUES (?, ?, 1, 'amount', 'Betreuung bis 14:00 Uhr', -500, 'Erste Woche krank, halber Beitrag', ?, NOW(), 'MB-ALT-1')`,
		organization, invoice, guardianAccountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE parent_billing.care_runs SET status = 'released', released_at = NOW(),
		issued_on = DATE '2029-05-20', document_count = 1, total_cents = 7750, month_rule = 'pro_rata' WHERE id = ?`, run)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id,
			number, number_year, number_sequence, issued_on, reason, total_cents, created_by_account_id)
		VALUES (?, ?, ?, 'MB-BELEG-2', 2029, 2, DATE '2029-05-25', 'Vertrag rückwirkend beendet', -7750, ?)`,
		organization, run, invoice, guardianAccountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind,
			run_id, care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
			mandate_signed_on, account_holder_sealed, iban_sealed)
		VALUES (?, ?, ?, 'care', ?, ?, 'MB-BELEG-1', 7750, DATE '2029-06-04', 'FRST', ?, 'SJR-BELEG-1', DATE '2026-09-01',
			'\x02'::bytea, '\x01'::bytea)`, organization, mandate, contract, run, invoice, creditor)
	require.NoError(t, err)

	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from,
			period_until, school_ids, status, released_at, issued_on, notice_count, total_cents)
		VALUES (?, 2029, DATE '2029-09-01', DATE '2029-12-31', ARRAY[?]::bigint[], 'released', NOW(), DATE '2029-09-24', 1, 3200)
		RETURNING id`, organization, school).Scan(ctx, &noticeRun))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_year, kind, care_contract_id, school_id, school_name, child_first_name,
			child_last_name, student_id, recipient_email, family_email, recipient, payment_method, period_from, period_until,
			lines, previous_cents, total_cents, delta_cents, legal_remedy)
		VALUES (?, ?, 'JB-BELEG-1', 2029, 1, DATE '2029-09-24', 2029, 'initial', ?, ?, 'OGS', 'Lia', 'Beispiel', ?, ?, ?,
			'{}'::jsonb, 'direct_debit', DATE '2029-09-01', DATE '2029-12-31', '[]'::jsonb, 0, 3200, 3200,
			'Widerspruch binnen eines Monats.')`,
		organization, noticeRun, contract, school, studentID, family, family)
	require.NoError(t, err)
}

// belegSnapshot reads every row of the Belege of the Träger, whole.
func belegSnapshot(t *testing.T, ctx context.Context, db *bun.DB, organization int64) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, table := range belegTables {
		var rows *string
		require.NoError(t, db.NewRaw(`SELECT jsonb_agg(row ORDER BY row.id)::text FROM `+table+` AS row
			WHERE row.organization_id = ?`, organization).Scan(ctx, &rows))
		require.NotNil(t, rows, "the fixture wrote %s", table)
		snapshot[table] = *rows
	}
	return snapshot
}

func TestBelegeSurviveTheDeletionOfChildGuardianAndAccount(t *testing.T) {
	t.Parallel()
	// The account erasure sweeps every unused account of the database.
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Belege bleiben", Slug: fmt.Sprintf("traeger-belege-bleiben-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)

	child := testpkg.CreateTestStudent(t, db, "Lia", "Beispiel", "1a")
	family := fmt.Sprintf("eva.beleg.%d@example.org", testpkg.UniqueTestTenantID(t))
	guardianAccount := testpkg.CreateTestAccount(t, db, family)
	// A parent account holds the guardian role of the school, nothing more.
	_, err := db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`,
		guardianAccount.ID, testpkg.GetOrCreateTestRole(t, db, "guardian").ID, testpkg.Tenant(t))
	require.NoError(t, err)
	guardian := testpkg.CreateTestGuardianProfile(t, db, family)
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, guardianAccount.ID, guardian.ID)
	require.NoError(t, err)
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "legal_guardian")
	seedBelege(t, ctx, db, organization, child.ID, guardianAccount.ID, family)
	issued := belegSnapshot(t, ctx, db, organization)

	// The child goes through the deletion workflow.
	workflow := f.workflow(t)
	preview, err := workflow.Preview(ctx, child.ID)
	require.NoError(t, err)
	_, err = workflow.Execute(ctx, child.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	require.Zero(t, rowCount(t, db, "users.student_profiles", child.ID))
	assert.Equal(t, issued, belegSnapshot(t, ctx, db, organization), "deleting the child keeps every Beleg unchanged")

	// Its guardian contact goes next.
	guardians, err := api.NewGuardianTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, guardians.PeopleDirectory.DeleteGuardian(ctx,
		peopledirectory.GuardianDelete{GuardianID: guardian.ID, ActorAccountID: f.actorID}))
	require.Zero(t, rowCount(t, db, "users.guardian_profiles", guardian.ID))
	assert.Equal(t, issued, belegSnapshot(t, ctx, db, organization), "deleting the guardian keeps every Beleg unchanged")

	// The account of the guardian is no longer used and is erased.
	_, err = db.ExecContext(ctx, `SELECT * FROM auth.erase_unused_accounts()`)
	require.NoError(t, err)
	var erased bool
	require.NoError(t, db.NewRaw(`SELECT personal_data_erased_at IS NOT NULL FROM auth.accounts WHERE id = ?`, guardianAccount.ID).
		Scan(ctx, &erased))
	require.True(t, erased, "the account erasure ran for the guardian's account")
	assert.Equal(t, issued, belegSnapshot(t, ctx, db, organization), "erasing the account keeps every Beleg unchanged")
}
