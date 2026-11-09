package postgres

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

		files, err := store.ExportFiles(ctx, own.organization, "", 0)
		require.NoError(t, err)
		require.Len(t, files, 1, "ExportFiles")
		assert.Equal(t, entry.ID, files[0].ID)
		files, err = store.ExportFiles(ctx, own.organization, parentbilling.DebitRunCare, own.run)
		require.NoError(t, err)
		require.Len(t, files, 1, "ExportFiles of a run")

		runs, err := store.ExportRuns(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, runs, 1, "ExportRuns")
		assert.Equal(t, own.run, runs[0].RunID)
		assert.Equal(t, 1, runs[0].Exports, "the other Träger's file of the same run ID is not counted")

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
