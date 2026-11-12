package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The export protocol keeps a Träger to its rows also below row level
// security (#4693): the Träger transaction drops its request role, so each
// assertion holds only because the adapter names the Träger in its own
// statement. Removing one filter turns its assertion red.
func TestExportFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownScope := newMandateScope(t, db, "Träger Export A")
	otherScope := newMandateScope(t, db, "Träger Export B")
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, ownScope.organization, "OGS Export A")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, otherScope.organization, "OGS Export B")
	store := NewStore()
	rollback := errors.New("roll the exports back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownScope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownScope.organization)
		require.NoError(t, err)
		own := seedDebitScope(t, ctx, tx, ownScope, ownSchool, "DE98ZZZ09999999999", "SJR-EA-1")
		other := seedDebitScope(t, ctx, tx, otherScope, otherSchool, "DE98ABC09999999999", "SJR-EB-1")
		var ownInvoice, otherInvoice int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, own.debit).Scan(ctx, &ownInvoice))
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, other.debit).Scan(ctx, &otherInvoice))

		// The other Träger has a counter, a file of a run with the same kind
		// and a link to this Träger's invoice, as only a broken write could
		// leave it (foreign keys off).
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.number_range_counters (organization_id, service, year, last_number)
			VALUES (?, 'export', 0, 500)`, other.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		var foreignFile int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
			file_name, content_sha256, invoice_count, total_cents, reference)
			VALUES (?, 7, 'pain008', 'care', ?, 'fremd.xml', repeat('a', 64), 1, 7750, 'MOTO-SAME') RETURNING id`,
			other.organization, own.run).Scan(ctx, &foreignFile))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_invoice_id)
			VALUES (?, ?, ?)`, other.organization, foreignFile, ownInvoice)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.family_billing (organization_id, family_email, care_contract_id,
			recipient_kind, debtor_number) VALUES (?, ?, 1, 'applicant', '99 999')`, other.organization, own.family)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		numbers, err := store.ExportFileNumbers(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, parentbilling.ExportFileNumbers{Next: 1}, numbers, "ExportFileNumbers reads only the own counter")
		number, err := store.ReserveExportFileNumber(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, numbers.Next, number, "the own counter starts at 1, not after the other Träger's 500")

		entry, err := store.InsertExportFile(ctx, own.organization, 0, parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatPain008,
			RunKind: parentbilling.DebitRunCare, RunID: own.run, FileNumber: number, FileName: "lastschrift-1.xml",
			Content: []byte("b"), TotalCents: 7750, Reference: "MOTO-SAME",
			SEPADebitIDs: []int64{own.debit, other.debit}})
		require.NoError(t, err)
		assert.Equal(t, 2, entry.InvoiceCount)
		var held []int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.export_file_invoices WHERE export_file_id = ?`, entry.ID).
			Scan(ctx, &held))
		assert.Equal(t, []int64{ownInvoice}, held, "InsertExportFile links only the own Träger's debits")

		files, err := store.ExportFiles(ctx, own.organization, "", 0, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		require.Len(t, files.Files, 1, "ExportFiles")
		assert.Equal(t, 1, files.Total, "ExportFiles counts only the own files")
		assert.Equal(t, entry.ID, files.Files[0].ID)
		files, err = store.ExportFiles(ctx, own.organization, parentbilling.DebitRunCare, own.run, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		require.Len(t, files.Files, 1, "ExportFiles of a run")

		runs, err := store.ExportRuns(ctx, own.organization, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		require.Len(t, runs.Runs, 1, "ExportRuns")
		assert.Equal(t, 1, runs.Total, "ExportRuns counts only the own runs")
		assert.Equal(t, own.run, runs.Runs[0].RunID)
		assert.Equal(t, 1, runs.Runs[0].Exports, "the other Träger's file of the same run ID is not counted")

		content, err := store.ExportRunContent(ctx, own.organization, parentbilling.DebitRunCare, own.run)
		require.NoError(t, err)
		require.Len(t, content.Invoices, 1, "ExportRunContent")
		assert.Equal(t, ownInvoice, content.Invoices[0].CareInvoiceID)
		_, err = store.ExportRunContent(ctx, own.organization, parentbilling.DebitRunCare, other.run)
		require.ErrorIs(t, err, parentbilling.ErrExportRunNotFound, "the run of another Träger")

		_, steps, exports, err := store.InvoiceHistory(ctx, own.organization, parentbilling.HistoryCareInvoice, ownInvoice)
		require.NoError(t, err)
		require.Len(t, exports, 1, "InvoiceHistory leaves the other Träger's file out")
		assert.Equal(t, entry.ID, exports[0].ID)
		require.NotEmpty(t, steps)
		assert.Equal(t, parentbilling.HistoryIssued, steps[0].Kind)
		_, _, _, err = store.InvoiceHistory(ctx, own.organization, parentbilling.HistoryCareInvoice, otherInvoice)
		require.ErrorIs(t, err, parentbilling.ErrExportInvoiceNotFound, "the invoice of another Träger")

		require.NoError(t, store.MarkExportFilesSubmitted(ctx, own.organization, 0, parentbilling.DebitRunCare, own.run, "MOTO-SAME"))
		var submitted int
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.export_files WHERE submitted_at IS NOT NULL AND organization_id = ?`,
			other.organization).Scan(ctx, &submitted))
		assert.Zero(t, submitted, "MarkExportFilesSubmitted leaves the other Träger's file of the same message ID")

		debtors, err := store.FamilyDebtorNumbers(ctx, own.organization)
		require.NoError(t, err)
		assert.Empty(t, debtors, "FamilyDebtorNumbers reads only the own families")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The file numbers of a Träger have no gap and name one file each, also
// when several people download at once (#4693, E-d): every download takes
// the counter row with one atomic upsert that holds its lock until the
// transaction ends. Reading the last number and writing the next one apart
// would give two downloads the same number; this test then fails on
// uq_export_files_number or on the sequence.
func TestExportFileNumbersStayUniqueAndGaplessUnderConcurrentDownloads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	scope := newMandateScope(t, db, "Träger Dateinummern")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Dateinummern")
	store := NewStore()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	var seeded debitScope
	require.NoError(t, testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		seeded = seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-NR-1")
		return store.StartExportFileNumbers(ctx, scope.organization, 110)
	}))

	download := func(rollback bool) (int64, error) {
		var number int64
		err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
			var err error
			if number, err = store.ReserveExportFileNumber(ctx, scope.organization); err != nil {
				return err
			}
			tx, err := traegerDB(ctx, scope.organization)
			if err != nil {
				return err
			}
			// Hold the transaction open while the others start theirs.
			if _, err := tx.ExecContext(ctx, `SELECT pg_sleep(0.05)`); err != nil {
				return err
			}
			if _, err := store.InsertExportFile(ctx, scope.organization, 0, parentbilling.ExportFileRecord{
				Format: parentbilling.ExportFormatSollstellung, RunKind: parentbilling.DebitRunCare, RunID: seeded.run, FileNumber: number,
				FileName: "s.csv", Content: []byte("c")}); err != nil {
				return err
			}
			if rollback {
				return errors.New("the download fails after its number")
			}
			return nil
		})
		return number, err
	}

	// A download that fails returns its number.
	_, err := download(true)
	require.Error(t, err)

	const downloads = 6
	numbers := make([]int64, downloads)
	errs := make([]error, downloads)
	var wg sync.WaitGroup
	for i := range downloads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			numbers[i], errs[i] = download(false)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	sort.Slice(numbers, func(a, b int) bool { return numbers[a] < numbers[b] })
	assert.Equal(t, []int64{110, 111, 112, 113, 114, 115}, numbers, "one number per file, from the start value, without a gap")

	err = testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.ErrorIs(t, store.StartExportFileNumbers(ctx, scope.organization, 1), parentbilling.ErrExportFileNumberStartLocked,
			"the start is set once")
		next, err := store.ExportFileNumbers(ctx, scope.organization)
		require.NoError(t, err)
		assert.Equal(t, parentbilling.ExportFileNumbers{Next: 116, Started: true}, next)
		return nil
	})
	require.NoError(t, err)
}

// A protocol entry never changes; the database refuses a second file with
// the same number of the Träger and every rewrite except the one
// confirmation of a PAIN.008 file.
func TestExportFileKeepsWhatTheDownloadWrote(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Protokoll")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Protokoll")
	store := NewStore()
	rollback := errors.New("roll the protocol back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		seeded := seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-PR-1")
		record := parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatPain008, RunKind: parentbilling.DebitRunCare,
			RunID: seeded.run, FileNumber: 3, FileName: "l-3.xml", Reference: "MOTO-P",
			Content: []byte("d"), SEPADebitIDs: []int64{seeded.debit}}
		entry, err := store.InsertExportFile(ctx, scope.organization, 0, record)
		require.NoError(t, err)
		refused := func(statement, want string, args ...any) {
			t.Helper()
			_, err := tx.ExecContext(ctx, `SAVEPOINT protocol`)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, statement, args...)
			require.ErrorContains(t, err, want, statement)
			_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT protocol`)
			require.NoError(t, err)
		}
		refused(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
			content_sha256, invoice_count, total_cents) VALUES (?, 3, 'sollstellung', 'care', ?, 's.csv', repeat('e', 64), 0, 0)`,
			"uq_export_files_number", scope.organization, seeded.run)
		refused(`UPDATE parent_billing.export_files SET file_name = 'anders.xml' WHERE id = ?`, "never changes", entry.ID)
		refused(`UPDATE parent_billing.export_files SET total_cents = 1 WHERE id = ?`, "never changes", entry.ID)
		refused(`UPDATE parent_billing.export_file_invoices SET care_invoice_id = NULL WHERE export_file_id = ?`,
			"chk_export_file_invoices_one", entry.ID)
		require.NoError(t, store.MarkExportFilesSubmitted(ctx, scope.organization, 0, parentbilling.DebitRunCare, seeded.run, "MOTO-P"))
		refused(`UPDATE parent_billing.export_files SET submitted_at = submitted_at + INTERVAL '1 day' WHERE id = ?`,
			"stays submitted", entry.ID)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// Collection identity is stable across creation timestamps; the byte hash remains exact.
func TestDebitExportRepeatsAndMealHistory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Wiederholung")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Wiederholung")
	store := NewStore()
	rollback := errors.New("roll the export history back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		seeded := seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-REPEAT-1")
		var invoice int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, seeded.debit).Scan(ctx, &invoice))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id,
   student_id, meal_month, mode, quantity, unit_cents, amount_cents, net_cents)
   VALUES (?, ?, 1, ?, 9501, DATE '2029-06-01', 'per_meal', 2, 480, 960, 960)`, scope.organization, invoice, school)
		require.NoError(t, err)
		var firstHash string
		var firstNumber int64
		for i, message := range []string{"MOTO-SAME", "MOTO-SAME", "MOTO-CHANGED"} {
			entry, err := store.InsertExportFile(ctx, scope.organization, 0, parentbilling.ExportFileRecord{
				Format: parentbilling.ExportFormatPain008, RunKind: parentbilling.DebitRunCare, RunID: seeded.run,
				FileNumber: int64(i + 1), FileName: "lastschrift.xml", Reference: message,
				Content: []byte(fmt.Sprintf("<CreDtTm>2026-11-09T10:0%d:00</CreDtTm>", i)), SEPADebitIDs: []int64{seeded.debit}})
			require.NoError(t, err)
			if i == 0 {
				firstHash, firstNumber = entry.Fingerprint, entry.FileNumber
			} else {
				assert.NotEqual(t, firstHash, entry.Fingerprint, "actual bytes retain their own fingerprint")
			}
		}
		page, err := store.ExportFiles(ctx, scope.organization, "", 0, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		files := page.Files
		require.Len(t, files, 3)
		assert.Zero(t, files[0].RepeatOf, "changed collection is not a repeat")
		assert.Equal(t, firstNumber, files[1].RepeatOf, "a later creation time is the same collection")
		_, steps, history, err := store.InvoiceHistory(ctx, scope.organization, parentbilling.HistoryCareInvoice, invoice)
		require.NoError(t, err)
		require.Len(t, history, 3)
		assert.Equal(t, firstNumber, history[1].RepeatOf)
		assert.Zero(t, history[2].RepeatOf)
		require.NotEmpty(t, steps)
		assert.Equal(t, 1, steps[0].Positions, "a Mensa-only invoice has one position")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A Sollstellung names both parents of an invoice to both, bills the period
// of the issued positions (a Mensa month billed later widens it) and writes
// the Storno as a row of its own (review round 2).
func TestExportRunContentNamesBothParentsAndThePeriodOfThePositions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Sollstellung Inhalt")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Sollstellung Inhalt")
	store := NewStore()
	rollback := errors.New("roll the content back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		seeded := seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-IN-1")
		var invoice int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, seeded.debit).Scan(ctx, &invoice))
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		for _, statement := range []string{
			`UPDATE parent_billing.care_invoices SET recipient_kind = 'both', recipient_second_name = 'Ben Beispiel' WHERE id = ?1`,
			`INSERT INTO parent_billing.care_invoice_positions (organization_id, invoice_id, position, care_offering_id, module_name,
				contract_version_number, service_from, service_until, weekdays, amount_cents, net_cents)
				VALUES (?0, ?1, 1, 1, 'Betreuung', 1, DATE '2029-06-10', DATE '2029-06-30', '[{"weekday":"mon"}]'::jsonb, 7750, 7750)`,
			`INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id, student_id, meal_month,
				mode, quantity, unit_cents, amount_cents, net_cents)
				VALUES (?0, ?1, 1, ?2, 1, DATE '2029-03-01', 'per_meal', 1, 480, 480, 480)`,
			`INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number, number_year,
				number_sequence, issued_on, reason, total_cents)
				VALUES (?0, (SELECT run_id FROM parent_billing.care_invoices WHERE id = ?1), ?1, 'ST-1', 2029, 99, DATE '2029-06-20',
				'Doppelt', -7750)`,
		} {
			_, err = tx.NewRaw(statement, scope.organization, invoice, school).Exec(ctx)
			require.NoError(t, err, statement)
		}
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		content, err := store.ExportRunContent(ctx, scope.organization, parentbilling.DebitRunCare, seeded.run)
		require.NoError(t, err)
		require.Len(t, content.Invoices, 2, "the invoice and its Storno")
		for _, row := range content.Invoices {
			assert.Equal(t, "Eva Beispiel und Ben Beispiel", row.RecipientName)
			assert.Equal(t, "2029-03-01", row.ServiceFrom.String(), "the Mensa month billed later")
			assert.Equal(t, "2029-06-30", row.ServiceUntil.String())
		}
		assert.Equal(t, "cancellation", content.Invoices[1].Kind)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The history of a holiday invoice shows the mail of its Storno too: the
// Storno is a document of its own that names the invoice (review round 3).
func TestHolidayInvoiceHistoryShowsTheMailOfItsCancellation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Ferienverlauf")
	store := NewStore()
	rollback := errors.New("roll the holiday history back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		var run, invoice, cancellation int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name,
			service_from, service_until, status, released_at, issued_on, document_count, total_cents)
			VALUES (?, 'holiday', 1, 'Herbst', DATE '2029-10-01', DATE '2029-10-05', 'released', NOW(), DATE '2029-09-20', 1, 1000)
			RETURNING id`, scope.organization).Scan(ctx, &run))
		insertDocument := `INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
			VALUES (?, ?, 'holiday', ?, 2029, ?, DATE '2029-09-20', 1, ?, 'Eva', 'Beispiel', '', DATE '2029-10-01', DATE '2029-10-05',
				?, ?, ?, ?) RETURNING id`
		require.NoError(t, tx.NewRaw(insertDocument, scope.organization, run, "FE-1", 1, 1, 1000, "invoice", nil, "").Scan(ctx, &invoice))
		require.NoError(t, tx.NewRaw(insertDocument, scope.organization, run, "FE-2", 2, 2, -1000, "cancellation", invoice, "Abgesagt").
			Scan(ctx, &cancellation))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id,
			recipient_email) VALUES (?, ?, ?, 4242, 'eltern@example.org')`, scope.organization, run, cancellation)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, details)
			VALUES (?, ?, 'cancel', 4243, jsonb_build_object('document_id', ?::bigint))`, scope.organization, run, cancellation)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		_, steps, _, err := store.InvoiceHistory(ctx, scope.organization, parentbilling.HistoryHolidayDocument, invoice)
		require.NoError(t, err)
		notes := map[string]string{}
		var canceller *int64
		for _, step := range steps {
			notes[step.Kind] = step.Note
			if step.Kind == parentbilling.HistoryCancelled {
				canceller = step.AccountID
			}
		}
		require.NotNil(t, canceller, "the Storno names the account that cancelled")
		assert.EqualValues(t, 4243, *canceller)
		assert.Equal(t, "cancellation", notes["mail_requested"], "the Storno's mail is in the invoice's history")
		assert.Equal(t, "FE-2", notes[parentbilling.HistoryCancelled])
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
