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

// The debits of the SEPA file keep a Träger to its rows also below row
// level security (#4697): the Träger transaction drops its request role, so
// every assertion holds only because the adapter names the Träger in its
// own statement. Removing one filter turns its assertion red.

type debitScope struct {
	mandateScope
	run   int64
	debit int64
}

// seedDebitScope writes, as the release would, a real chain: the creditor,
// a contract of a school, the family's mandate, a released monthly run with
// one invoice and its pending FRST debit. The rows are real because an
// update rechecks the foreign keys of rows its own transaction inserted.
func seedDebitScope(t *testing.T, ctx context.Context, tx bun.IDB, scope mandateScope, school int64, creditorID, reference string) debitScope {
	t.Helper()
	seeded := debitScope{mandateScope: scope}
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_creditors (organization_id, creditor_id) VALUES (?, ?)`,
		scope.organization, creditorID)
	require.NoError(t, err)
	var contract int64
	require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
		VALUES (?, ?, ?, 'versendet', NOW()) RETURNING id`, scope.organization, school, testpkg.UniqueTestTenantID(t)).Scan(ctx, &contract))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
		signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, ?, ?, ?, DATE '2026-09-01', 'recurring', 'first', ?, '\x02'::bytea, '\x01'::bytea, 'DE•• 30 00', 'office')
		RETURNING id`, scope.organization, scope.family, contract, reference, creditorID).Scan(ctx, &seeded.mandate))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, DATE '2029-06-01')
		RETURNING id`, scope.organization).Scan(ctx, &seeded.run))
	var invoice int64
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence,
		issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
		recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, ?, 2029, 1, DATE '2029-05-20', DATE '2029-06-01', ?, ?, 'OGS', 'Lia', 'Beispiel', 'Eva', 'Beispiel',
			'eva@example.org', 'pro_rata', 7750) RETURNING id`,
		scope.organization, seeded.run, "RE-"+reference, contract, school).Scan(ctx, &invoice))
	_, err = tx.ExecContext(ctx, `UPDATE parent_billing.care_runs SET status = 'released', released_at = NOW(),
		issued_on = DATE '2029-05-20', document_count = 1, total_cents = 7750, month_rule = 'pro_rata' WHERE id = ?`, seeded.run)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id,
		care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference, mandate_signed_on,
		account_holder_sealed, iban_sealed)
		VALUES (?, ?, ?, 'care', ?, ?, ?, 7750, DATE '2029-06-04', 'FRST', ?, ?, DATE '2026-09-01', '\x02'::bytea, '\x01'::bytea)
		RETURNING id`, scope.organization, seeded.mandate, contract, seeded.run, invoice, "RE-"+reference, creditorID,
		reference).Scan(ctx, &seeded.debit))
	return seeded
}

func TestSEPADebitFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownScope := newMandateScope(t, db, "Träger Datei A")
	otherScope := newMandateScope(t, db, "Träger Datei B")
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, ownScope.organization, "OGS Datei A")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, otherScope.organization, "OGS Datei B")
	store := NewStore()
	rollback := errors.New("roll the debits back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownScope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownScope.organization)
		require.NoError(t, err)
		own := seedDebitScope(t, ctx, tx, ownScope, ownSchool, "DE98ZZZ09999999999", "SJR-DA-1")
		other := seedDebitScope(t, ctx, tx, otherScope, otherSchool, "DE98ABC09999999999", "SJR-DB-1")

		debits, err := store.SEPADebitsOfRun(ctx, own.organization, parentbilling.DebitRunCare, own.run, true, false)
		require.NoError(t, err)
		require.Len(t, debits, 1, "SEPADebitsOfRun")
		assert.Equal(t, own.debit, debits[0].Debit.ID)
		assert.Equal(t, "DE98ZZZ09999999999", debits[0].CreditorNow, "the creditor now is the Träger's own")
		assert.Equal(t, []byte{1}, debits[0].IBAN)

		_, err = store.SEPADebitsOfRun(ctx, own.organization, parentbilling.DebitRunCare, other.run, true, false)
		require.ErrorIs(t, err, parentbilling.ErrSEPADebitRunNotFound, "the run of another Träger")
		history, err := store.SEPADebitsOfMandate(ctx, own.organization, other.mandate)
		require.NoError(t, err)
		assert.Empty(t, history, "SEPADebitsOfMandate of another Träger's mandate")

		err = store.SubmitSEPADebits(ctx, own.organization, 0, "MOTO-M1-X", []int64{other.debit},
			map[int64]calendar.Date{other.mandate: calendar.NewDate(2029, 6, 4)})
		require.ErrorIs(t, err, parentbilling.ErrSEPADebitsChanged, "a debit of another Träger is no pending debit here")
		require.NoError(t, store.SubmitSEPADebits(ctx, own.organization, 0, "MOTO-M1-A", []int64{own.debit},
			map[int64]calendar.Date{own.mandate: calendar.NewDate(2029, 6, 4), other.mandate: calendar.NewDate(2029, 6, 4)}))

		var state string
		const debitState = `SELECT debit.status || '|' || debit.sequence_type || '|' || mandate.sequence || '|'
			|| COALESCE(mandate.last_collected_on::text, '-')
			FROM parent_billing.sepa_debits AS debit JOIN parent_billing.sepa_mandates AS mandate ON mandate.id = debit.mandate_id
			WHERE debit.id = ?`
		require.NoError(t, tx.NewRaw(debitState, own.debit).Scan(ctx, &state))
		assert.Equal(t, "submitted|FRST|recurring|2029-06-04", state, "the own debit is collected, its mandate turns to RCUR")
		require.NoError(t, tx.NewRaw(debitState, other.debit).Scan(ctx, &state))
		assert.Equal(t, "pending|FRST|first|-", state, "the other Träger's debit and mandate stay untouched")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A debit keeps what the release wrote; only a pending FRST may turn to
// RCUR and a pending debit become submitted; a submitted one never changes.
func TestSEPADebitKeepsWhatTheReleaseWrote(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Datei Historie")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Datei Historie")
	store := NewStore()
	rollback := errors.New("roll the debit back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		seeded := seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-DH-1")
		refused := func(statement, want string) {
			t.Helper()
			_, err := tx.ExecContext(ctx, `SAVEPOINT debit`)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, statement, seeded.debit)
			require.ErrorContains(t, err, want, statement)
			_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT debit`)
			require.NoError(t, err)
		}
		refused(`UPDATE parent_billing.sepa_debits SET amount_cents = 1 WHERE id = ?`, "keeps its invoice, amount and account")
		refused(`UPDATE parent_billing.sepa_debits SET iban_sealed = '\x09'::bytea WHERE id = ?`, "keeps its invoice, amount and account")
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.sepa_debits SET sequence_type = 'RCUR' WHERE id = ?`, seeded.debit)
		require.NoError(t, err, "a pending FRST turns to RCUR")
		refused(`UPDATE parent_billing.sepa_debits SET sequence_type = 'FRST' WHERE id = ?`, "only turns from FRST to RCUR")
		require.NoError(t, store.SubmitSEPADebits(ctx, scope.organization, 0, "MOTO-M1-H", []int64{seeded.debit},
			map[int64]calendar.Date{seeded.mandate: calendar.NewDate(2029, 6, 4)}))
		refused(`UPDATE parent_billing.sepa_debits SET status = 'pending', message_id = NULL, submitted_at = NULL WHERE id = ?`,
			"a submitted SEPA debit never changes")
		err = store.SubmitSEPADebits(ctx, scope.organization, 0, "MOTO-M1-H2", []int64{seeded.debit},
			map[int64]calendar.Date{seeded.mandate: calendar.NewDate(2029, 7, 4)})
		require.ErrorIs(t, err, parentbilling.ErrSEPADebitsChanged, "a debit is submitted once")
		var last string
		require.NoError(t, tx.NewRaw(`SELECT last_collected_on::text FROM parent_billing.sepa_mandates WHERE id = ?`, seeded.mandate).Scan(ctx, &last))
		assert.Equal(t, "2029-06-04", last, "the refused second confirmation collects nothing")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
