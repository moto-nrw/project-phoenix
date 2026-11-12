package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The last DATEV batch of a run (#4694) is one of this Träger, also below
// row level security (brief rule 25): the other Träger holds a newer DATEV
// file on this Träger's run, as only a broken write could leave it. Without
// the Träger filter of the DATEV subquery of exportRunsQuery the run shows
// that file; without its format filter it shows the Sollstellung.
func TestExportRunsNameTheLastDatevBatchOfTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger DATEV A")
	other := newMandateScope(t, db, "Träger DATEV B")
	store := NewStore()
	rollback := errors.New("roll the DATEV files back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		run, invoice := seedHolidayExportRun(t, ctx, tx, own.organization, "DATEV Eigen", 1000, false)

		number, err := store.ReserveExportFileNumber(ctx, own.organization)
		require.NoError(t, err)
		datev, err := store.InsertExportFile(ctx, own.organization, 0, parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatDatev,
			RunKind: parentbilling.DebitRunHoliday, RunID: run, FileNumber: number, FileName: "EXTF_Buchungsstapel_1.csv",
			Content: []byte("datev"), TotalCents: 1000, DocumentIDs: []int64{invoice}})
		require.NoError(t, err)
		soll := recordSollstellung(t, ctx, store, own.organization, parentbilling.DebitRunHoliday, run, 1, 1000, nil, []int64{invoice})
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
			file_name, content_sha256, invoice_count, total_cents) VALUES (?, 900, 'datev', 'holiday', ?, 'fremd.csv', repeat('d', 64), 1, 1)`,
			other.organization, run)
		require.NoError(t, err)

		page, err := store.ExportRuns(ctx, own.organization, 0, 10)
		require.NoError(t, err)
		require.Len(t, page.Runs, 1)
		got := page.Runs[0]
		require.NotNil(t, got.LastDatev)
		assert.Equal(t, datev.ID, got.LastDatev.ID, "the newest DATEV file of this Träger, not the other's")
		assert.Equal(t, parentbilling.ExportFormatDatev, got.LastDatev.Format)
		assert.Equal(t, "EXTF_Buchungsstapel_1.csv", got.LastDatev.FileName)
		assert.Equal(t, int64(1000), got.LastDatev.TotalCents)
		require.NotNil(t, got.LastSollstellung)
		assert.Equal(t, soll.ID, got.LastSollstellung.ID, "the Sollstellung stays the Sollstellung")
		assert.Equal(t, 2, got.Exports, "both own files, none of the other Träger")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

func TestDatevProtocolAndHolidayHistoryRecognizeTheSameBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger DATEV Wiederholung")
	other := newMandateScope(t, db, "Träger DATEV Fremde Wiederholung")
	store := NewStore()
	rollback := errors.New("roll the DATEV repeats back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		run, invoice := seedHolidayExportRun(t, ctx, tx, scope.organization, "DATEV Wiederholung", 1000, false)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
			file_name, content_sha256, booking_sha256, invoice_count, total_cents)
			VALUES (?, 1, 'datev', 'holiday', ?, 'fremd.csv', repeat('a', 64), encode(sha256('same bookings'::bytea), 'hex'), 1, 1000)`, other.organization, run)
		require.NoError(t, err)
		var firstNumber int64
		for i, bookings := range []string{"same bookings", "same bookings", "changed bookings"} {
			entry, err := store.InsertExportFile(ctx, scope.organization, 0, parentbilling.ExportFileRecord{
				Format: parentbilling.ExportFormatDatev, RunKind: parentbilling.DebitRunHoliday, RunID: run,
				FileNumber: int64(i + 2), FileName: fmt.Sprintf("EXTF_Buchungsstapel_%d.csv", i+2),
				Content: []byte(fmt.Sprintf("file %d", i)), BookingContent: []byte(bookings),
				DocumentIDs: []int64{invoice}, TotalCents: 1000})
			require.NoError(t, err)
			if i == 0 {
				firstNumber = entry.FileNumber
			}
		}
		page, err := store.ExportFiles(ctx, scope.organization, "", 0, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		require.Len(t, page.Files, 3)
		assert.Zero(t, page.Files[0].RepeatOf, "changed bookings are not a repeat")
		assert.Equal(t, firstNumber, page.Files[1].RepeatOf, "the first file of this Träger, also below RLS")
		assert.Zero(t, page.Files[2].RepeatOf)
		_, _, history, err := store.InvoiceHistory(ctx, scope.organization, parentbilling.HistoryHolidayDocument, invoice)
		require.NoError(t, err)
		require.Len(t, history, 3)
		assert.Zero(t, history[0].RepeatOf)
		assert.Equal(t, firstNumber, history[1].RepeatOf)
		assert.Zero(t, history[2].RepeatOf)
		assert.NotEqual(t, history[0].Fingerprint, history[1].Fingerprint)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
