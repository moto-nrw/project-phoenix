package owners

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The deletion of a Beleg decides on what holds at the moment of the delete,
// not on the record of the day's evaluation (#4720, ADR 0184): a suspension
// or a Weisung recorded meanwhile, a cancellation or credit note issued
// meanwhile and the trigger of a mandate whose debits went.

// waitUntilBlockedOrDone reports whether the call behind done is waiting on
// a lock of the database; false when it finished first.
func waitUntilBlockedOrDone(t *testing.T, db *bun.DB, done <-chan error) (blocked bool, finished error) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return false, err
		case <-deadline:
			t.Fatal("the call neither finished nor waited on a lock")
		case <-tick.C:
			var waiting int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_stat_activity
				WHERE datname = current_database() AND wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(context.Background(), &waiting))
			if waiting > 0 {
				return true, nil
			}
		}
	}
}

func belegExists(t *testing.T, db *bun.DB, table string, id int64) bool {
	t.Helper()
	var found bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = ?)`, id).Scan(context.Background(), &found))
	return found
}

// TestBelegRetentionJobWaitsForASuspensionBeingRecorded: an operator records
// a suspension of the debits while the hourly job runs. The job must not
// decide on the suspensions it read before the operator committed; it waits
// for the Träger and keeps the suspended debit and the invoice it holds.
func TestBelegRetentionJobWaitsForASuspensionBeingRecorded(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit gleichzeitiger Aussetzung")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		return nil
	}))
	today = calendar.NewDate(2035, time.January, 2)
	job := NewBelegRetention(func() calendar.Date { return calendar.NewDate(2035, time.January, 2) })

	recorded, release := make(chan struct{}), make(chan struct{})
	operatorDone := make(chan error, 1)
	go func() {
		operatorDone <- tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			if _, err := retention.RecordSuspension(adminCtx, f.organization, 0, parentbilling.NewRetentionSuspension{
				BelegType: parentbilling.BelegSEPADebit, Reason: "Betriebsprüfung 2026", Scope: "alle Lastschriften",
				Responsible: "Kämmerei", ReviewOn: calendar.NewDate(2035, time.March, 1),
			}); err != nil {
				close(recorded)
				return err
			}
			close(recorded)
			<-release
			return nil
		})
	}()
	<-recorded

	jobDone := make(chan error, 1)
	go func() {
		jobDone <- tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			_, err := job.ApplyBelegRetention(adminCtx)
			return err
		})
	}()
	blocked, early := waitUntilBlockedOrDone(t, db, jobDone)
	close(release)
	require.NoError(t, <-operatorDone)
	if blocked {
		require.NoError(t, <-jobDone)
	} else {
		require.NoError(t, early)
	}
	assert.True(t, blocked, "the job waits for the operator's change of the Träger")
	assert.True(t, belegExists(t, db, "parent_billing.sepa_debits", f.debit), "the debit suspended meanwhile stays")
	assert.True(t, belegExists(t, db, "parent_billing.care_invoices", f.careInvoice), "and with it the invoice it holds")
}

// TestBelegDeletionRechecksWhatHoldsAtDeleteTime calls the deletion after
// the evaluation of the day before, with facts that changed in between.
func TestBelegDeletionRechecksWhatHoldsAtDeleteTime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	suspended := newBelegFixture(t, db, "Träger mit späterer Aussetzung")
	withdrawn := newBelegFixture(t, db, "Träger ohne aktuelle Weisung")
	cancelled := newBelegFixture(t, db, "Träger mit spätem Storno")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	admin := func(fn func(context.Context) error) {
		t.Helper()
		require.NoError(t, tenant.WithinAdmin(unit, fn))
	}
	admin(func(adminCtx context.Context) error {
		for _, f := range []belegFixture{suspended, withdrawn, cancelled} {
			instruct(t, retention, adminCtx, f.organization)
		}
		return nil
	})
	today = calendar.NewDate(2034, time.December, 31)
	admin(func(adminCtx context.Context) error { _, err := retention.ApplyBelegRetention(adminCtx); return err })

	// After the evaluation: a suspension of the debits of one Träger, no
	// current Weisung for invoices and debits of another, a cancellation
	// of the monthly invoice of the third, issued on the day of deletion.
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retention_suspensions
		(organization_id, beleg_type, reason, scope, responsible, review_on)
		VALUES (?, 'sepa_debit', 'Betriebsprüfung', 'alle Lastschriften', 'Kämmerei', '2035-03-01')`, suspended.organization)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE parent_billing.beleg_retention_instructions SET replaced_at = CURRENT_TIMESTAMP
		WHERE organization_id = ? AND beleg_type IN ('sepa_debit', 'care_invoice')`, withdrawn.organization)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number,
			number_year, number_sequence, issued_on, reason, total_cents)
		SELECT organization_id, run_id, id, 'MB-ST-1', 2035, 1, DATE '2035-01-01', 'Storno', -total_cents
		FROM parent_billing.care_invoices WHERE id = ?`, cancelled.careInvoice)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `SELECT * FROM parent_billing.delete_due_belege(DATE '2035-01-01')`)
	require.NoError(t, err)

	assert.True(t, belegExists(t, db, "parent_billing.sepa_debits", suspended.debit), "a suspension recorded after the evaluation holds the debit")
	assert.True(t, belegExists(t, db, "parent_billing.care_invoices", suspended.careInvoice))
	assert.True(t, belegExists(t, db, "parent_billing.sepa_debits", withdrawn.debit), "without a current Weisung nothing goes")
	assert.True(t, belegExists(t, db, "parent_billing.care_invoices", withdrawn.careInvoice))
	assert.True(t, belegExists(t, db, "parent_billing.care_invoices", cancelled.careInvoice),
		"a cancellation issued in 2035 keeps its invoice until 2044")
	var cancellations int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoice_cancellations WHERE invoice_id = ?`,
		cancelled.careInvoice).Scan(ctx, &cancellations))
	assert.Equal(t, 1, cancellations)
}

// TestBelegDeletionWaitsForACreditNoteBeingIssued: a credit note on an
// invoice is issued while the deletion runs. The deletion waits for it and
// keeps the invoice, whose cascade would take the new credit note along.
func TestBelegDeletionWaitsForACreditNoteBeingIssued(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit Gutschrift im Lauf")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		return nil
	}))
	today = calendar.NewDate(2035, time.December, 31)
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		_, err := retention.ApplyBelegRetention(adminCtx)
		return err
	}))
	require.True(t, belegExists(t, db, "parent_billing.documents", f.invoice))

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var credit int64
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
		VALUES (?, ?, 'holiday', 'FE-3', 0, ?, '2036-01-01', 1, 2, 'Eva', 'Muster', 'eva@example.org', '2026-07-06', '2026-07-10',
			-1000, 'credit', ?, 'Gutschrift') RETURNING id`,
		f.organization, f.holidayRun, testpkg.UniqueTestTenantID(t), f.invoice).Scan(ctx, &credit))

	deletion := make(chan error, 1)
	go func() {
		_, err := db.ExecContext(ctx, `SELECT * FROM parent_billing.delete_due_belege(DATE '2036-01-01')`)
		deletion <- err
	}()
	blocked, early := waitUntilBlockedOrDone(t, db, deletion)
	require.NoError(t, tx.Commit())
	if blocked {
		require.NoError(t, <-deletion)
	} else {
		require.NoError(t, early)
	}
	assert.True(t, belegExists(t, db, "parent_billing.documents", credit), "the credit note issued in 2036 stays")
	assert.True(t, belegExists(t, db, "parent_billing.documents", f.invoice), "and keeps the invoice it refers to")
}

// TestMandateKeepsTheSubmissionOfItsLastDebitAfterTheDebitWent: the period
// of an expired mandate counts from the submission of its last debit. Once
// the debit went, the record keeps that day instead of falling back to the
// later collection day on the mandate.
func TestMandateKeepsTheSubmissionOfItsLastDebitAfterTheDebitWent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit ausgesetztem Mandat")
	_, err := db.ExecContext(ctx, `UPDATE parent_billing.sepa_mandates SET last_collected_on = '2026-11-13', sequence = 'recurring' WHERE id = ?`, f.mandate)
	require.NoError(t, err)
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	admin := func(fn func(context.Context) error) {
		t.Helper()
		require.NoError(t, tenant.WithinAdmin(unit, fn))
	}
	admin(func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		_, err := retention.RecordSuspension(adminCtx, f.organization, 0, parentbilling.NewRetentionSuspension{
			BelegType: parentbilling.BelegSEPAMandate, Reason: "Rückfrage der Bank", Scope: "alle Mandate",
			Responsible: "Kämmerei", ReviewOn: calendar.NewDate(2036, time.March, 1),
		})
		return err
	})
	trigger := func() calendar.Date {
		t.Helper()
		var day calendar.Date
		require.NoError(t, db.NewRaw(`SELECT trigger_date FROM parent_billing.beleg_retentions
			WHERE organization_id = ? AND beleg_type = 'sepa_mandate' AND beleg_id = ?`, f.organization, f.mandate).Scan(ctx, &day))
		return day
	}
	for _, day := range []calendar.Date{calendar.NewDate(2035, time.January, 1), calendar.NewDate(2035, time.January, 2)} {
		today = day
		admin(func(adminCtx context.Context) error { _, err := retention.ApplyBelegRetention(adminCtx); return err })
	}
	require.False(t, belegExists(t, db, "parent_billing.sepa_debits", f.debit), "the debit of 2026 went on 1 January 2035")
	require.True(t, belegExists(t, db, "parent_billing.sepa_mandates", f.mandate), "the suspended mandate stays")
	assert.Equal(t, calendar.NewDate(2026, time.November, 10), trigger(), "the submission of the last debit, not its collection day")
}

// TestContractDeletionWaitsForAMandateBeingSigned: a mandate is recorded for
// an ended contract while the deletion runs. The deletion waits for it and
// keeps the contract, whose cascade would take the new mandate along.
func TestContractDeletionWaitsForAMandateBeingSigned(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit neuem Mandat im Lauf")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		return nil
	}))
	today = calendar.NewDate(2035, time.January, 1)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var mandate int64
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
			signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, 'zweiter.elternteil@example.org', ?, ?, '2034-12-20', 'recurring', 'first', 'DE98ZZZ09999999999', '\x01'::bytea, '\x02'::bytea, 'DE••', 'office')
		RETURNING id`, f.organization, f.contract, "M-NEU-"+time.Now().Format("150405.000000")).Scan(ctx, &mandate))

	deletion := make(chan error, 1)
	go func() {
		deletion <- tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			_, err := retention.ApplyBelegRetention(adminCtx)
			return err
		})
	}()
	blocked, early := waitUntilBlockedOrDone(t, db, deletion)
	require.NoError(t, tx.Commit())
	if blocked {
		require.NoError(t, <-deletion)
	} else {
		require.NoError(t, early)
	}
	assert.False(t, belegExists(t, db, "parent_billing.sepa_mandates", f.mandate), "the expired mandate of 2026 goes")
	assert.True(t, belegExists(t, db, "parent_billing.sepa_mandates", mandate), "the mandate signed meanwhile stays")
	assert.True(t, belegExists(t, db, "care_contract.contracts", f.contract), "and keeps its contract")
}

// TestInstructionTableRefusesAShortStatutoryPeriod: the database holds the
// same floor as the application, also for a row written past it.
func TestInstructionTableRefusesAShortStatutoryPeriod(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Untergrenze", Slug: "beleg-untergrenze-" + time.Now().Format("150405.000000"),
	}).ID
	insert := func(belegType, trigger string, period int, basis, legalBasis string, justification *string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retention_instructions (organization_id, beleg_type, trigger_event,
				period, basis, legal_basis, basis_justification, instructed_by, instructed_on, instruction_reference)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'Frau Beispiel, Kämmerei', '2026-11-09', 'Nachtrag 2026-11')`,
			organization, belegType, trigger, period, basis, legalBasis, justification)
		return err
	}
	reason := "Träger ist nicht buchführungspflichtig"
	for name, err := range map[string]error{
		"invoice for one year under § 147 AO": insert("care_invoice", "issued", 1, "booking_voucher", "§ 147 AO", nil),
		"invoice as business letter":          insert("care_invoice", "issued", 6, "business_letter", "§ 257 HGB", nil),
		"contract as booking voucher, 6":      insert("care_contract", "contract_ended", 6, "booking_voucher", "§ 147 AO", nil),
		"notice as business letter, 5":        insert("care_notice", "issued", 5, "business_letter", "§ 257 HGB", nil),
		"no statutes, no justification":       insert("sepa_debit", "issued", 1, "not_applicable", "Satzung", nil),
		"no statutes, citing § 14b UStG":      insert("holiday_document", "issued", 1, "not_applicable", "§ 14b UStG", &reason),
	} {
		var pgErr interface{ Field(byte) string }
		require.ErrorAs(t, err, &pgErr, name)
		assert.Equal(t, "23514", pgErr.Field('C'), name)
	}
	require.NoError(t, insert("care_invoice", "issued", 1, "not_applicable", "Satzung des Trägers § 12", &reason))
}

// TestInstructionTableRefusesEverySpellingOfTheStatutes: the database holds
// "Vorschriften nicht anwendbar" to the same normalized citation check as
// the application (statuteSpellings in beleg_retention_rules_test.go), also
// for a row written past it (#4720, third review).
func TestInstructionTableRefusesEverySpellingOfTheStatutes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Zitate", Slug: "beleg-zitate-" + time.Now().Format("150405.000000"),
	}).ID
	insert := func(legalBasis string) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.beleg_retention_instructions (organization_id, beleg_type, trigger_event,
					period, basis, legal_basis, basis_justification, instructed_by, instructed_on, instruction_reference)
				VALUES (?, 'care_invoice', 'issued', 1, 'not_applicable', ?, 'Träger ist nicht buchführungspflichtig',
					'Frau Beispiel, Kämmerei', '2026-11-09', 'Nachtrag 2026-11')`, organization, legalBasis)
			if err == nil {
				// Keep the active instruction free for the next spelling.
				return errRollbackSpelling
			}
			return err
		})
	}
	for _, spelling := range []string{
		"§147AO", "§ 147 Abs. 1 AO", "147ao", "HGB§257", "UStG14b", "Abgabenordnung", "ABGABENORDNUNG", "aBgAbEnOrDnUnG",
		"Handelsgesetzbuch", "Umsatzsteuergesetz", "§\u00a0147\u00a0AO", "§\u2009257\u2009HGB", "§\u202f14b\u202fUStG", "A.O.", "A-O",
		"§ 14 b UStG", "§ 257 Abs. 4 Nr. 1 HGB", "Paragraph 147 der Abgaben-Ordnung", "\uff21\uff2f", "A\u200bO", "nach §147",
	} {
		var pgErr interface{ Field(byte) string }
		err := insert(spelling)
		require.ErrorAs(t, err, &pgErr, "%q", spelling)
		assert.Equal(t, "23514", pgErr.Field('C'), "%q", spelling)
	}
	for _, basis := range []string{"Satzung des Trägers § 12", "Beschluss des Vorstands vom 01.02.2026", "Kommunalhaushaltsverordnung NRW § 58"} {
		assert.ErrorIs(t, insert(basis), errRollbackSpelling, "%q", basis)
	}
}

var errRollbackSpelling = errors.New("accepted, rolled back")

// billingHoldsWhileRetentionRuns plays a billing transaction against the
// hourly job (#4720, third review): first takes the lock billing takes
// first, then the job runs, then next takes the lock billing takes second.
// The billing transaction must complete and the job must neither fail nor
// make billing fail; a busy Beleg waits for a later run.
func billingHoldsWhileRetentionRuns(t *testing.T, db *bun.DB, job parentbilling.BelegRetention, first, next string, args ...any) {
	t.Helper()
	ctx := context.Background()
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, first, args...)
	require.NoError(t, err)

	jobDone := make(chan error, 1)
	go func() {
		jobDone <- tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
			_, err := job.ApplyBelegRetention(adminCtx)
			return err
		})
	}()
	blocked, early := waitUntilBlockedOrDone(t, db, jobDone)
	_, billingErr := tx.ExecContext(ctx, next, args...)
	if billingErr == nil {
		billingErr = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	jobErr := early
	if blocked {
		jobErr = <-jobDone
	}
	require.NoError(t, billingErr, "the billing transaction completes")
	require.NoError(t, jobErr, "the job skips what billing holds instead of failing")
	assert.False(t, blocked, "the job does not wait on billing")
}

// TestBelegRetentionSkipsAnInvoiceWhoseRunACancellationHolds mirrors the
// probe of a monthly cancellation: billing locks the run, then adds the
// cancellation to the invoice. The job used to lock the invoice and then
// wait on the run, so Postgres aborted one side with 40P01.
func TestBelegRetentionSkipsAnInvoiceWhoseRunACancellationHolds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit Storno im Löschlauf")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		return nil
	}))
	today = calendar.NewDate(2035, time.January, 2)
	billingHoldsWhileRetentionRuns(t, db, retention,
		`SELECT r.id FROM parent_billing.care_runs r JOIN parent_billing.care_invoices i ON i.run_id = r.id WHERE i.id = ?0 FOR UPDATE OF r`,
		`INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number, number_year, number_sequence,
			issued_on, reason, total_cents)
		SELECT organization_id, run_id, id, 'MB-ST-1', 2026, 1, DATE '2026-12-01', 'Storno', -total_cents
		FROM parent_billing.care_invoices WHERE id = ?0`, f.careInvoice)
	assert.True(t, belegExists(t, db, "parent_billing.care_invoices", f.careInvoice), "the invoice billing holds waits for a later run")

	today = calendar.NewDate(2035, time.January, 3)
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		_, err := retention.ApplyBelegRetention(adminCtx)
		return err
	}))
	assert.False(t, belegExists(t, db, "parent_billing.care_invoices", f.careInvoice), "the next run deletes it with its cancellation of 2026")
}

// TestBelegRetentionSkipsADebitWhoseMandateAConfirmationHolds mirrors the
// probe of a SEPA confirmation: billing locks the mandate, then the debit.
// The job used to lock the debit and then wait on the mandate, so Postgres
// aborted one side with 40P01.
func TestBelegRetentionSkipsADebitWhoseMandateAConfirmationHolds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	f := newBelegFixture(t, db, "Träger mit Bestätigung im Löschlauf")
	today := calendar.NewDate(2026, time.November, 9)
	retention := NewBelegRetention(func() calendar.Date { return today })
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		instruct(t, retention, adminCtx, f.organization)
		return nil
	}))
	today = calendar.NewDate(2035, time.January, 2)
	billingHoldsWhileRetentionRuns(t, db, retention,
		`SELECT id FROM parent_billing.sepa_mandates WHERE id = ?0 FOR UPDATE`,
		`SELECT id FROM parent_billing.sepa_debits WHERE mandate_id = ?0 ORDER BY id FOR UPDATE`, f.mandate)
	for table, id := range map[string]int64{"parent_billing.sepa_debits": f.debit, "parent_billing.care_invoices": f.careInvoice,
		"parent_billing.sepa_mandates": f.mandate} {
		assert.True(t, belegExists(t, db, table, id), "%s waits for a later run", table)
	}

	today = calendar.NewDate(2035, time.January, 3)
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		_, err := retention.ApplyBelegRetention(adminCtx)
		return err
	}))
	for table, id := range map[string]int64{"parent_billing.sepa_debits": f.debit, "parent_billing.care_invoices": f.careInvoice,
		"parent_billing.sepa_mandates": f.mandate} {
		assert.False(t, belegExists(t, db, table, id), "the next run deletes %s", table)
	}
}
