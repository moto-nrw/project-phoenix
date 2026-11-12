package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The handovers of protocol entries (ADR 0218) keep a Träger to its rows
// below row level security: the Träger transaction drops its request role,
// so each assertion holds only because the adapter names the Träger in its
// own statement. The other Träger holds a handed over file of the same run
// and content and handovers on this Träger's files, as only a broken write
// could leave them.
func TestExportHandoversKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	unique := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Übergabe A", Slug: fmt.Sprintf("handover-a-%d", unique)}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Übergabe B", Slug: fmt.Sprintf("handover-b-%d", unique)}).ID
	store := NewStore()
	rollback := errors.New("roll the handovers back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own)
		require.NoError(t, err)
		file := func(organization, number int64) int64 {
			var id int64
			require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
				file_name, content_sha256, invoice_count, total_cents) VALUES (?, ?, 'sollstellung', 'care', 77, ?, repeat('c', 64), 1, 7800)
				RETURNING id`, organization, number, fmt.Sprintf("sollstellung-%d.csv", number)).Scan(ctx, &id))
			return id
		}
		ownFirst, ownSecond, foreign := file(own, 110), file(own, 111), file(other, 110)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host,
			target_directory, state, settled_at) VALUES (?, ?, 'fremd.example.org', '/x', 'transferred', NOW())`, other, foreign)
		require.NoError(t, err)

		repeated, err := store.SameContentHandedOver(ctx, own, ownSecond)
		require.NoError(t, err)
		assert.False(t, repeated, "another Träger's handover of the same run and content does not count")

		// Handovers of the other Träger on this Träger's files (keys off).
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host,
			target_directory) VALUES (?, ?, 'fremd.example.org', '/x'), (?, ?, 'fremd.example.org', '/x')`, other, ownFirst, other, ownSecond)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		files, err := store.ExportFiles(ctx, own, "", 0, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		require.Len(t, files.Files, 2)
		for _, entry := range files.Files {
			assert.Nil(t, entry.Handover, "ExportFiles shows no handover of another Träger")
		}

		require.NoError(t, store.InsertExportFileHandover(ctx, own, ownFirst, "kaemmerei.example.org", "/eingang"))
		repeated, err = store.SameContentHandedOver(ctx, own, ownSecond)
		require.NoError(t, err)
		assert.True(t, repeated, "the own pending handover of the same content counts")

		require.NoError(t, store.SettleExportFileHandover(ctx, own, ownFirst, parentbilling.HandoverFailed, "connection_failed"))
		var pending int
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.export_file_transfers WHERE organization_id = ? AND state = 'pending'`,
			other).Scan(ctx, &pending))
		assert.Equal(t, 2, pending, "settling the own handover leaves the other Träger's")

		files, err = store.ExportFiles(ctx, own, "", 0, 0, parentbilling.ExportPageSize)
		require.NoError(t, err)
		var settled *parentbilling.ExportFileHandover
		for _, entry := range files.Files {
			if entry.ID == ownFirst {
				settled = entry.Handover
			}
		}
		require.NotNil(t, settled)
		assert.Equal(t, parentbilling.HandoverFailed, settled.State)
		assert.Equal(t, "connection_failed", settled.Reason)
		assert.Equal(t, "kaemmerei.example.org", settled.TargetHost)
		require.Error(t, store.SettleExportFileHandover(ctx, own, ownFirst, parentbilling.HandoverTransferred, ""),
			"a settled handover settles once")
		// Last, because the refusal aborts the transaction: no write turns a
		// settled handover into another outcome.
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.export_file_transfers SET state = 'transferred', failure_reason = ''
			WHERE organization_id = ? AND export_file_id = ?`, own, ownFirst)
		require.Error(t, err, "the database keeps a settled handover")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A DATEV file whose handover failed did not arrive: its bookings become new
// again, so the next handover sends them (ADR 0218 section 6). A booking a
// later DATEV file held too moves to that file; a file that arrived keeps its
// bookings.
func TestFailedHandoverReleasesItsDatevBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	unique := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger DATEV A", Slug: fmt.Sprintf("handover-datev-a-%d", unique)}).ID
	store := NewStore()
	rollback := errors.New("roll the bookings back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		// Seeded as the table owner; the handover runs as the request role.
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own)
		require.NoError(t, err)
		_, released := seedHolidayExportRun(t, ctx, tx, own, "DATEV frei", 1000, false)
		_, kept := seedHolidayExportRun(t, ctx, tx, own, "DATEV angekommen", 1000, false)
		_, repeated := seedHolidayExportRun(t, ctx, tx, own, "DATEV wiederholt", 1000, false)
		file := func(number int64, keys ...string) int64 {
			var id int64
			require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
				file_name, content_sha256, booking_sha256, invoice_count, total_cents, datev_booking_keys)
				VALUES (?, ?, 'datev', 'care', 77, ?, repeat('d', 64), repeat('e', 64), 1, 7800, ?) RETURNING id`,
				own, number, fmt.Sprintf("EXTF_Buchungsstapel_%d.csv", number), pgdialect.Array(append([]string{}, keys...))).Scan(ctx, &id))
			return id
		}
		key := func(document int64) string { return fmt.Sprintf("d:%d", document) }
		failed, arrived := file(5), file(6)
		// A later "all bookings" file held one booking of the failed file too;
		// another later file held the other one, but its handover failed; a
		// third held the Storno of an invoice whose invoice booking the failed
		// file took, never the invoice itself.
		again, lost := file(7, key(repeated)), file(8, key(released))
		file(9, fmt.Sprintf("c:%d", unique))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, document_id)
			VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?)`, own, failed, released, own, arrived, kept, own, failed, repeated)
		require.NoError(t, err)
		// The invoice booking of an invoice this test does not create.
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, care_invoice_id)
			VALUES (?, ?, ?)`, own, failed, unique)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`)
		require.NoError(t, err)
		for _, id := range []int64{failed, arrived, lost} {
			require.NoError(t, store.InsertExportFileHandover(ctx, own, id, "kaemmerei.example.org", "/eingang"))
		}
		require.NoError(t, store.SettleExportFileHandover(ctx, own, lost, parentbilling.HandoverFailed, "connection_failed"))
		require.NoError(t, store.SettleExportFileHandover(ctx, own, failed, parentbilling.HandoverFailed, "connection_failed"))
		require.NoError(t, store.SettleExportFileHandover(ctx, own, arrived, parentbilling.HandoverTransferred, ""))
		bookedIn := func(document int64) []int64 {
			var files []int64
			require.NoError(t, tx.NewRaw(`SELECT export_file_id FROM parent_billing.datev_booked_documents WHERE organization_id = ? AND document_id = ?`,
				own, document).Scan(ctx, &files))
			return files
		}
		assert.Empty(t, bookedIn(released), "a failed handover releases the bookings of its file, also one a failed later file held")
		assert.Equal(t, []int64{again}, bookedIn(repeated), "a booking a later file held stays booked, on that file")
		assert.Equal(t, []int64{arrived}, bookedIn(kept), "a file that arrived keeps its bookings")
		var invoiceBooked int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.datev_booked_documents WHERE organization_id = ? AND care_invoice_id = ?`,
			own, unique).Scan(ctx, &invoiceBooked))
		assert.Zero(t, invoiceBooked, "a later file with only the Storno does not keep the invoice booked")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The policy on handovers hides another Träger's rows from an unfiltered
// query of the request role, in a Träger and in a school transaction.
func TestExportHandoverPolicyHidesForeignAndSchoolRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	unique := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Sicht A", Slug: fmt.Sprintf("handover-rls-a-%d", unique)}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Sicht B", Slug: fmt.Sprintf("handover-rls-b-%d", unique)}).ID
	var file int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
		content_sha256, invoice_count, total_cents) VALUES (?, 1, 'sollstellung', 'care', 77, 'fremd.csv', repeat('a', 64), 1, 100)
		RETURNING id`, other).Scan(context.Background(), &file))
	_, err := db.NewRaw(`INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host, target_directory)
		VALUES (?, ?, 'kaemmerei.example.org', '/eingang')`, other, file).Exec(context.Background())
	require.NoError(t, err)
	hidden := func(ctx context.Context, tx bun.IDB) {
		var visible int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.export_file_transfers`).Scan(ctx, &visible))
		assert.Zero(t, visible)
	}
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		tx, err := traegerDB(ctx, own)
		require.NoError(t, err)
		hidden(ctx, tx)
		return nil
	}))
	school := testpkg.OwnTenant(t)
	require.NoError(t, testpkg.WithinTenantTransaction(testpkg.Ctx(t), db, school, func(ctx context.Context, tx bun.Tx) error {
		hidden(ctx, tx)
		return nil
	}))
}
