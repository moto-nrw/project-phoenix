package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// seedHolidayExportRun inserts a released holiday run of the Träger with an
// invoice of cents, and its Storno when cancelled. The run's own counts are
// those of its release: one document of cents. Foreign keys are off.
func seedHolidayExportRun(t *testing.T, ctx context.Context, tx bun.IDB, organization int64, name string, cents int64, cancelled bool,
) (run, invoice int64) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name,
		service_from, service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', 1, ?, DATE '2029-10-01', DATE '2029-10-05', 'released', clock_timestamp(), DATE '2029-09-20', 1, ?)
		RETURNING id`, organization, name, cents).Scan(ctx, &run))
	insertDocument := `INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
		number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
		recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
		VALUES (?, ?, 'holiday', ?, 2029, ?, DATE '2029-09-20', 1, ?, 'Eva', 'Beispiel', '', DATE '2029-10-01', DATE '2029-10-05',
			?, ?, ?, ?) RETURNING id`
	sequence := testpkg.UniqueTestTenantID(t)
	require.NoError(t, tx.NewRaw(insertDocument, organization, run, fmt.Sprintf("FE-%d", sequence), sequence, sequence, cents,
		"invoice", nil, "").Scan(ctx, &invoice))
	if cancelled {
		var cancellation int64
		require.NoError(t, tx.NewRaw(insertDocument, organization, run, fmt.Sprintf("FS-%d", sequence), sequence+1, sequence+1, -cents,
			"cancellation", invoice, "Abgesagt").Scan(ctx, &cancellation))
	}
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
	require.NoError(t, err)
	return run, invoice
}

// recordSollstellung records a Sollstellung of a run as SollstellungFile
// would, with the figures of its content at that moment.
func recordSollstellung(t *testing.T, ctx context.Context, store *Store, organization int64, runKind string, run int64, count int,
	cents int64, care, documents []int64,
) parentbilling.ExportProtocolEntry {
	t.Helper()
	number, err := store.ReserveExportFileNumber(ctx, organization)
	require.NoError(t, err)
	entry, err := store.InsertExportFile(ctx, organization, 0, parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatSollstellung,
		RunKind: runKind, RunID: run, FileNumber: number, FileName: fmt.Sprintf("sollstellung-%d.csv", number),
		Content: []byte(fmt.Sprintf("datei %d", number)), TotalCents: cents, DocumentCount: count,
		CareInvoiceIDs: care, DocumentIDs: documents})
	require.NoError(t, err)
	return entry
}

// The figures of a run on the export page are those of the documents a
// Sollstellung of it writes now, not those of its release; a file keeps
// the figures of the content it had (review of 9d0709ae69: after a Storno
// the overview showed 1 / 77,50 € instead of 2 / 0 €). Reading the release
// counts of care_runs or runs again turns this test red.
func TestExportOverviewCountsTheDocumentsASollstellungWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger Übersicht")
	school := testpkg.CreateTestSchoolInOrganization(t, db, scope.organization, "OGS Übersicht")
	store := NewStore()
	rollback := errors.New("roll the overview back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		care := seedDebitScope(t, ctx, tx, scope, school, "DE98ZZZ09999999999", "SJR-UE-1")
		var careInvoice int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, care.debit).Scan(ctx, &careInvoice))
		careFile := recordSollstellung(t, ctx, store, scope.organization, parentbilling.DebitRunCare, care.run, 1, 7750, []int64{careInvoice}, nil)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number,
			number_year, number_sequence, issued_on, reason, total_cents)
			VALUES (?, ?, ?, 'ST-UE-1', 2029, 99, DATE '2029-06-20', 'Doppelt', -7750)`, scope.organization, care.run, careInvoice)
		require.NoError(t, err)
		holiday, holidayInvoice := seedHolidayExportRun(t, ctx, tx, scope.organization, "Herbst", 1000, false)
		holidayFile := recordSollstellung(t, ctx, store, scope.organization, parentbilling.DebitRunHoliday, holiday, 1, 1000, nil,
			[]int64{holidayInvoice})
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
			SELECT organization_id, run_id, service, 'FS-UE', number_year, number_sequence + 1000, issued_on, holiday_period_id,
				holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email, service_from, service_until,
				-total_cents, 'cancellation', id, 'Abgesagt'
			FROM parent_billing.documents WHERE id = ?`, holidayInvoice)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		page, err := store.ExportRuns(ctx, scope.organization, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		byKind := map[string]parentbilling.ExportRun{}
		for _, run := range page.Runs {
			byKind[run.RunKind] = run
		}
		for kind, file := range map[string]parentbilling.ExportProtocolEntry{
			parentbilling.DebitRunCare: careFile, parentbilling.DebitRunHoliday: holidayFile,
		} {
			run, ok := byKind[kind]
			require.True(t, ok, kind)
			assert.Equal(t, 2, run.InvoiceCount, "%s: the invoice and its Storno", kind)
			assert.Zero(t, run.TotalCents, "%s: the Storno takes the amount back", kind)
			require.NotNil(t, run.LastSollstellung, kind)
			assert.Equal(t, file.InvoiceCount, run.LastSollstellung.InvoiceCount, "%s: the file keeps its own count", kind)
			assert.Equal(t, file.TotalCents, run.LastSollstellung.TotalCents, "%s: the file keeps its own sum", kind)
			assert.Equal(t, 1, run.LastSollstellung.InvoiceCount, kind)

			content, err := store.ExportRunContent(ctx, scope.organization, kind, run.RunID)
			require.NoError(t, err)
			assert.Len(t, content.Invoices, run.InvoiceCount, "%s: the overview counts what the file writes", kind)
			assert.Equal(t, run.InvoiceCount, content.Run.InvoiceCount, kind)
			assert.Equal(t, run.TotalCents, content.Run.TotalCents, kind)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The lists of the export page go on in pages and count the whole list
// (review of 9d0709ae69: a fixed LIMIT cut older runs and files off). Each
// Träger pages through its own rows only, also below row level security.
func TestExportListsPageThroughEveryRowOfTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Seiten A")
	other := newMandateScope(t, db, "Träger Seiten B")
	store := NewStore()
	rollback := errors.New("roll the pages back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		ownRuns := map[int64]bool{}
		for i := range 3 {
			run, invoice := seedHolidayExportRun(t, ctx, tx, own.organization, fmt.Sprintf("Eigen %d", i), 1000, false)
			ownRuns[run] = true
			if i > 0 {
				recordSollstellung(t, ctx, store, own.organization, parentbilling.DebitRunHoliday, run, 1, 1000, nil, []int64{invoice})
			}
		}
		for i := range 2 {
			run, _ := seedHolidayExportRun(t, ctx, tx, other.organization, fmt.Sprintf("Fremd %d", i), 500, false)
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
				file_name, content_sha256, invoice_count, total_cents) VALUES (?, ?, 'sollstellung', 'holiday', ?, 'fremd.csv',
				repeat('b', 64), 1, 500)`, other.organization, 100+i, run)
			require.NoError(t, err)
		}

		seen := map[int64]bool{}
		for offset := 0; offset < 3; offset++ {
			page, err := store.ExportRuns(ctx, own.organization, offset, 1)
			require.NoError(t, err)
			require.Len(t, page.Runs, 1, "offset %d", offset)
			assert.Equal(t, 3, page.Total, "every own run, none of the other Träger")
			assert.Equal(t, 1, page.NotExported, "one own run has no Sollstellung")
			seen[page.Runs[0].RunID] = true
		}
		assert.Equal(t, ownRuns, seen, "the pages hold every own run once")
		last, err := store.ExportRuns(ctx, own.organization, 3, 1)
		require.NoError(t, err)
		assert.Empty(t, last.Runs, "no page after the last own run")

		files := map[int64]bool{}
		for offset := 0; offset < 2; offset++ {
			page, err := store.ExportFiles(ctx, own.organization, "", 0, offset, 1)
			require.NoError(t, err)
			require.Len(t, page.Files, 1, "offset %d", offset)
			assert.Equal(t, 2, page.Total, "every own file, none of the other Träger")
			files[page.Files[0].ID] = true
		}
		assert.Len(t, files, 2, "the pages hold every own file once")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
