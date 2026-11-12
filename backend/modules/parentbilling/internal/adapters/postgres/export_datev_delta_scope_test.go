package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The record of what went to DATEV (#6433): a booking is in a DATEV file
// of its own Träger only, also below row level security, and two standard
// downloads at the same time cannot both take the same bookings.

func datevFileRecord(run, number int64, all bool, bookings ...parentbilling.DatevBookingRef) parentbilling.ExportFileRecord {
	return parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatDatev, RunKind: parentbilling.DebitRunHoliday, RunID: run,
		FileNumber: number, FileName: "EXTF_Buchungsstapel.csv", Content: []byte{byte(number)}, TotalCents: 1000,
		DatevAll: all, DatevBookings: bookings}
}

// Brief rule 25: the other Träger holds rows on this Träger's invoice, its
// Storno and its holiday documents, as only a broken write could leave
// them. Without the Träger filter of datevBooked* the run reads them as
// booked and the next DATEV file leaves them out.
func TestDatevBookedDocumentsOfTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger DATEV Delta A")
	other := newMandateScope(t, db, "Träger DATEV Delta B")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own.organization, "OGS DATEV Delta")
	store := NewStore()
	rollback := errors.New("roll the DATEV bookings back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		holiday, document := seedHolidayExportRun(t, ctx, tx, own.organization, "DATEV Delta", 1000, true)
		care := seedDebitScope(t, ctx, tx, own, school, "DE98ZZZ09999999998", "SJR-DELTA-1")
		var invoice, cancellation, holidayStorno int64
		require.NoError(t, tx.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, care.run).Scan(ctx, &invoice))
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number,
			number_year, number_sequence, issued_on, reason, total_cents)
			VALUES (?, ?, ?, 'ST-DELTA', 2029, 2, DATE '2029-06-20', 'Storno', -7750) RETURNING id`,
			own.organization, care.run, invoice).Scan(ctx, &cancellation))
		require.NoError(t, tx.NewRaw(`SELECT id FROM parent_billing.documents WHERE reference_document_id = ?`, document).
			Scan(ctx, &holidayStorno))
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		for _, column := range []struct {
			name string
			id   int64
		}{{"care_invoice_id", invoice}, {"care_cancellation_id", cancellation}, {"document_id", document}, {"document_id", holidayStorno}} {
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, `+
				column.name+`) VALUES (?, 1, ?)`, other.organization, column.id)
			require.NoError(t, err)
		}
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		booked := func() map[string]bool {
			states := map[string]bool{}
			for _, runKind := range []string{parentbilling.DebitRunCare, parentbilling.DebitRunHoliday} {
				id := care.run
				if runKind == parentbilling.DebitRunHoliday {
					id = holiday
				}
				content, err := store.ExportRunContent(ctx, own.organization, runKind, id)
				require.NoError(t, err)
				for _, row := range content.Invoices {
					states["content "+row.Number] = row.DatevBooked
				}
			}
			runs, err := store.DatevRunBookings(ctx, own.organization, []parentbilling.ExportRunRef{
				{RunKind: parentbilling.DebitRunCare, RunID: care.run}, {RunKind: parentbilling.DebitRunHoliday, RunID: holiday}})
			require.NoError(t, err)
			for ref, rows := range runs {
				for _, row := range rows {
					states["runs "+ref.RunKind+" "+row.Kind] = row.DatevBooked
				}
			}
			return states
		}
		states := booked()
		require.Len(t, states, 8, "invoice and Storno of both runs, read by both statements")
		for name, state := range states {
			assert.False(t, state, "%s: a row of the other Träger is no booking of this one", name)
		}

		// The own record counts; the other Träger's rows did not take the
		// bookings from this Träger.
		number, err := store.ReserveExportFileNumber(ctx, own.organization)
		require.NoError(t, err)
		entry, err := store.InsertExportFile(ctx, own.organization, 0, datevFileRecord(holiday, number, false,
			parentbilling.DatevBookingRef{DocumentID: document}, parentbilling.DatevBookingRef{DocumentID: holidayStorno}))
		require.NoError(t, err)
		assert.False(t, entry.DatevAll)
		number, err = store.ReserveExportFileNumber(ctx, own.organization)
		require.NoError(t, err)
		_, err = store.InsertExportFile(ctx, own.organization, 0, parentbilling.ExportFileRecord{Format: parentbilling.ExportFormatDatev,
			RunKind: parentbilling.DebitRunCare, RunID: care.run, FileNumber: number, FileName: "EXTF_Buchungsstapel.csv",
			Content: []byte("care"), DatevBookings: []parentbilling.DatevBookingRef{{CareInvoiceID: invoice}, {CareCancellationID: cancellation}}})
		require.NoError(t, err)
		for name, state := range booked() {
			assert.True(t, state, "%s: the own DATEV file holds it", name)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// Two standard downloads at the same time take the same new bookings. The
// second waits on the unique index until the first commits, finds the
// bookings recorded and fails, so its transaction rolls back with its file
// number and protocol entry. Without the unique index both commit and the
// tax office books the run twice. "All again" is not refused.
func TestDatevBookingsGoIntoOneOfTwoConcurrentFiles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newMandateScope(t, db, "Träger DATEV Gleichzeitig")
	store := NewStore()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	var run, document int64
	require.NoError(t, testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		run, document = seedHolidayExportRun(t, ctx, tx, scope.organization, "DATEV Gleichzeitig", 1000, false)
		return nil
	}))
	booking := parentbilling.DatevBookingRef{DocumentID: document}

	recorded, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseFirst)
	first := make(chan error, 1)
	go func() {
		first <- testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
			if _, err := store.InsertExportFile(ctx, scope.organization, 0, datevFileRecord(run, 1, false, booking)); err != nil {
				close(recorded)
				return err
			}
			close(recorded)
			<-release
			return nil
		})
	}()
	<-recorded
	second := make(chan error, 1)
	go func() {
		second <- testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
			_, err := store.InsertExportFile(ctx, scope.organization, 0, datevFileRecord(run, 2, false, booking))
			return err
		})
	}()
	// The second download waits on the booking of the first, not on a
	// fixed time.
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'
			AND query LIKE '%datev_booked_documents%' AND datname = current_database()`).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "the second file waits for the first")
	releaseFirst()
	require.NoError(t, <-first)
	require.ErrorIs(t, <-second, parentbilling.ErrDatevBookingsTaken)

	var files []int64
	require.NoError(t, db.NewRaw(`SELECT file_number FROM parent_billing.export_files WHERE organization_id = ? ORDER BY file_number`,
		scope.organization).Scan(context.Background(), &files))
	assert.Equal(t, []int64{1}, files, "the refused file left no protocol entry")

	// "All again" repeats the booking on purpose; its first file stays 1.
	require.NoError(t, testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		entry, err := store.InsertExportFile(ctx, scope.organization, 0, datevFileRecord(run, 2, true, booking))
		require.NoError(t, err)
		assert.True(t, entry.DatevAll)
		return nil
	}))
	var firstFile int64
	require.NoError(t, db.NewRaw(`SELECT file.file_number FROM parent_billing.datev_booked_documents AS booked
		JOIN parent_billing.export_files AS file ON file.id = booked.export_file_id
		WHERE booked.organization_id = ? AND booked.document_id = ?`, scope.organization, document).Scan(context.Background(), &firstFile))
	assert.Equal(t, files[0], firstFile, "the first standard file keeps the booking")
}
