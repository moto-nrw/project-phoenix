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

// The letters, sends and Storno of the monthly invoices keep a Träger to
// its rows also below row level security (#4684, ADR 0165): the Träger
// transaction drops its request role, so each assertion holds only because
// the adapter names the Träger in its own statement. Both Träger share the
// contract, the month and the address, so a missing filter shows the other
// Träger's row.

const (
	sharedLetterContract = int64(990_001)
	sharedLetterEmail    = "eva.letters@example.org"
)

// careLetterScope is a Träger with a released run, a cancelled invoice that
// was sent, both letters, one send and the invoice that replaced it.
type careLetterScope struct {
	careScope
	invoice      int64
	letter       int64
	stornoLetter int64
}

func seedCareLetterScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careLetterScope, name string) careLetterScope {
	t.Helper()
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at,
		issued_on, document_count, total_cents, month_rule) VALUES (?, DATE '2029-09-01', 'released', NOW(), DATE '2029-09-01', 1, 3200,
		'version_on_first') RETURNING id`, scope.organization).Scan(ctx, &scope.released))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
		number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
		recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, ?, 2029, 1, DATE '2029-09-01', DATE '2029-09-01', ?, ?, ?, 'Mia', 'Beispiel', 'Eva', 'Beispiel', ?,
		'version_on_first', 3200) RETURNING id`, scope.organization, scope.released, scope.number, sharedLetterContract, scope.school,
		name, sharedLetterEmail).Scan(ctx, &scope.invoice))
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_letters (organization_id, run_id, payment_due_on, notes)
		VALUES (?, ?, DATE '2029-09-15', ?)`, scope.organization, scope.released, name)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_letters (organization_id, run_id, invoice_id, kind, filename, pdf)
		VALUES (?, ?, ?, 'invoice', 'rechnung.pdf', '\x255044462d'::bytea) RETURNING id`,
		scope.organization, scope.released, scope.invoice).Scan(ctx, &scope.letter))
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_letter_mailings (organization_id, run_id, letter_id, delivery_id,
		recipient_email) VALUES (?, ?, ?, ?, ?)`, scope.organization, scope.released, scope.letter, scope.letter, sharedLetterEmail)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number,
		number_year, number_sequence, issued_on, reason, total_cents) VALUES (?, ?, ?, ?, 2029, 2, DATE '2029-09-10', 'Falscher Vertrag', -3200)`,
		scope.organization, scope.released, scope.invoice, scope.number+"-ST")
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_letters (organization_id, run_id, invoice_id, kind, filename, pdf)
		VALUES (?, ?, ?, 'cancellation', 'storno.pdf', '\x255044462d'::bytea) RETURNING id`,
		scope.organization, scope.released, scope.invoice).Scan(ctx, &scope.stornoLetter))
	return scope
}

func TestCareLettersFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := careLetterScope{careScope: newCareScope(t, db, "Träger Schreiben A")}
	other := careLetterScope{careScope: newCareScope(t, db, "Träger Schreiben B")}
	store := NewStore()
	september := calendar.NewDate(2029, 9, 1)
	rollback := errors.New("roll the monthly letters back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own = seedCareLetterScope(t, ctx, tx, own, "Träger Schreiben A")
		other = seedCareLetterScope(t, ctx, tx, other, "Träger Schreiben B")

		letters, err := store.CareRunLetters(ctx, own.organization, other.released)
		require.NoError(t, err)
		assert.Nil(t, letters, "CareRunLetters of another Träger's run")
		records, err := store.CareRunLetterRecords(ctx, own.organization, other.released, true)
		require.NoError(t, err)
		assert.Empty(t, records, "CareRunLetterRecords of another Träger's run")
		cancellations, err := store.CareRunCancellations(ctx, own.organization, other.released)
		require.NoError(t, err)
		assert.Empty(t, cancellations, "CareRunCancellations of another Träger's run")
		sends, err := store.CareRunMailings(ctx, own.organization, other.released)
		require.NoError(t, err)
		assert.Empty(t, sends, "CareRunMailings of another Träger's run")

		_, err = store.InsertCareCancellation(ctx, own.organization, other.released, 0, 2029, 9,
			parentbilling.CareCancellation{InvoiceID: other.invoice, Number: "ST-OWN-9", IssuedOn: september, Reason: "x", TotalCents: -1})
		assert.ErrorIs(t, err, parentbilling.ErrCareInvoiceNotFound, "InsertCareCancellation of another Träger's invoice")
		assert.NoError(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{other.number + "-ST"}, careNumbersTable),
			"another Träger's Storno number is free here")
		assert.ErrorIs(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{own.number + "-ST"}, holidayNumbersTable),
			parentbilling.ErrInvoiceNumberTaken, "the own Storno number is taken")

		replaced, err := cancelledInvoicesToReplace(ctx, tx, own.organization, september,
			[]parentbilling.CareInvoice{{ContractID: sharedLetterContract}})
		require.NoError(t, err)
		require.Contains(t, replaced, sharedLetterContract)
		assert.Equal(t, own.invoice, *replaced[sharedLetterContract], "cancelledInvoicesToReplace names the own invoice")

		invoiced, err := store.InvoicedContracts(ctx, own.organization, september)
		require.NoError(t, err)
		assert.Empty(t, invoiced, "a cancelled invoice bills its contract no more")

		inbox, err := store.RecipientDocuments(ctx, own.organization, sharedLetterEmail)
		require.NoError(t, err)
		require.Len(t, inbox, 1, "RecipientDocuments lists only the own sent letter")
		assert.Equal(t, own.letter, inbox[0].DocumentID)
		assert.True(t, inbox[0].CareRun)
		assert.Equal(t, parentbilling.ParentDocumentKindCareInvoiceLetter, inbox[0].Kind)
		assert.Equal(t, "Monatsbeitrag September 2029", inbox[0].PeriodName)
		_, _, _, err = store.RecipientDocumentLetter(ctx, own.organization, other.letter, sharedLetterEmail)
		assert.ErrorIs(t, err, parentbilling.ErrParentDocumentNotFound, "RecipientDocumentLetter of another Träger's letter")
		_, _, _, err = store.RecipientDocumentLetter(ctx, own.organization, own.stornoLetter, sharedLetterEmail)
		assert.ErrorIs(t, err, parentbilling.ErrParentDocumentNotFound, "a letter never sent is not in the inbox")
		runID, care, file, err := store.RecipientDocumentLetter(ctx, own.organization, own.letter, sharedLetterEmail)
		require.NoError(t, err)
		assert.Equal(t, own.released, runID)
		assert.True(t, care)
		assert.Equal(t, "rechnung.pdf", file.Filename)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A letter, a send and a Storno never change, for any role (1.15.809).
func TestCareLettersNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := careLetterScope{careScope: newCareScope(t, db, "Träger Schreiben C")}
	rollback := errors.New("roll back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedCareLetterScope(t, ctx, tx, own, "Träger Schreiben C")
		for _, statement := range []string{
			`UPDATE parent_billing.care_letters SET filename = 'neu.pdf' WHERE id = ?`,
			`UPDATE parent_billing.care_letter_mailings SET recipient_email = 'neu@example.org' WHERE letter_id = ?`,
			`UPDATE parent_billing.care_invoice_cancellations SET reason = 'neu' WHERE invoice_id = (SELECT invoice_id FROM parent_billing.care_letters WHERE id = ?)`,
			`UPDATE parent_billing.care_run_letters SET notes = 'neu' WHERE run_id = (SELECT run_id FROM parent_billing.care_letters WHERE id = ?)`,
		} {
			_, err := tx.ExecContext(ctx, "SAVEPOINT change")
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, statement, own.letter)
			assert.ErrorContains(t, err, "never changes", statement)
			_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT change")
			require.NoError(t, err)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
