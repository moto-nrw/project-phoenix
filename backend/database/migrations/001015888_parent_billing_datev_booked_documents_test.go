package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The backfill of 1.15.888 takes from a legacy monthly DATEV file's
// recorded booking count how many of its Stornos it held, not from the
// transaction timestamps: a Storno whose transaction started before the
// export but committed after it was not in the file. It never stops the
// migration.
func TestDatevBackfillCountsTheStornosALegacyFileHeld(t *testing.T) {
	t.Parallel()
	// The test drops and rebuilds the table: a clone of its own keeps the
	// schema of the other tests of the package.
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, parentBillingDatevBookedDocumentsDown(ctx, db))

	// Issued at 09:00, exported at 10:00, but the file counts only the
	// invoice: the Storno was not visible to the export.
	notHeld := seedLegacyCareDatev(t, db, []int64{3700}, "2029-10-30 09:00:00+00")
	notHeldFile := notHeld.export(t, db, 1, 1)
	// Issued at 11:00 after the export, yet the file counts it.
	held := seedLegacyCareDatev(t, db, []int64{3700}, "2029-10-30 11:00:00+00")
	heldFile := held.export(t, db, 1, 2)
	// Two Stornos, the file counts one: the one issued first.
	partial := seedLegacyCareDatev(t, db, []int64{3700, 2500}, "2029-10-30 09:00:00+00", "2029-10-30 12:00:00+00")
	partialFile := partial.export(t, db, 1, 3)
	// A zero-value invoice is no booking and leaves no room for a Storno.
	zero := seedLegacyCareDatev(t, db, []int64{0}, "2029-10-30 09:00:00+00")
	zero.export(t, db, 1, 0)

	require.NoError(t, parentBillingDatevBookedDocumentsUp(ctx, db), "the backfill never stops the migration")

	fileOf := func(f legacyCareDatevFixture, column string, id int64) []int64 {
		t.Helper()
		var files []int64
		require.NoError(t, db.NewRaw(`SELECT export_file_id FROM parent_billing.datev_booked_documents
			WHERE organization_id = ? AND `+column+` = ?`, f.organization, id).Scan(ctx, &files))
		return files
	}
	assert.Equal(t, []int64{notHeldFile}, fileOf(notHeld, "care_invoice_id", notHeld.invoices[0]))
	assert.Empty(t, fileOf(notHeld, "care_cancellation_id", notHeld.cancellations[0]), "the next standard file books it")
	assert.Equal(t, []int64{heldFile}, fileOf(held, "care_cancellation_id", held.cancellations[0]))
	assert.Equal(t, []int64{partialFile}, fileOf(partial, "care_cancellation_id", partial.cancellations[0]))
	assert.Empty(t, fileOf(partial, "care_cancellation_id", partial.cancellations[1]))
	assert.Empty(t, fileOf(zero, "care_invoice_id", zero.invoices[0]))
	assert.Empty(t, fileOf(zero, "care_cancellation_id", zero.cancellations[0]))
}

type legacyCareDatevFixture struct {
	organization, run       int64
	invoices, cancellations []int64
}

// seedLegacyCareDatev writes a monthly run with one invoice per amount and
// a Storno of each, issued at the given instants.
func seedLegacyCareDatev(t *testing.T, db *bun.DB, amounts []int64, cancelledAt ...string) legacyCareDatevFixture {
	t.Helper()
	ctx := context.Background()
	f := legacyCareDatevFixture{organization: testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger DATEV Bestand", Slug: fmt.Sprintf("traeger-datev-bestand-%d", testpkg.UniqueTestTenantID(t)),
	}).ID}
	school := testpkg.CreateTestSchoolInOrganization(t, db, f.organization, "DATEV Bestand OGS")
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month)
		VALUES (?, DATE '2029-10-01') RETURNING id`, f.organization).Scan(ctx, &f.run))
	for i, amount := range amounts {
		var invoice, cancellation int64
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
			VALUES (?, ?, ?, 2029, ?, DATE '2029-10-30', DATE '2029-10-01', ?, ?, 'OGS', 'Mia', 'Beispiel', 'Eva', 'Beispiel', '',
			'version_on_first', ?) RETURNING id`, f.organization, f.run, fmt.Sprintf("MB-%d", i+1), i+1,
			testpkg.UniqueTestTenantID(t), school, amount).Scan(ctx, &invoice))
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id,
			number, number_year, number_sequence, issued_on, issued_at, reason, total_cents)
			VALUES (?, ?, ?, ?, 2029, ?, DATE '2029-10-30', ?::timestamptz, 'Doppelt', ?) RETURNING id`,
			f.organization, f.run, invoice, fmt.Sprintf("ST-%d", i+1), len(amounts)+i+1, cancelledAt[min(i, len(cancelledAt)-1)],
			-amount).Scan(ctx, &cancellation))
		f.invoices = append(f.invoices, invoice)
		f.cancellations = append(f.cancellations, cancellation)
	}
	return f
}

// export records a legacy DATEV file of the run at 10:00 with its booking
// count and links every invoice to it.
func (f legacyCareDatevFixture) export(t *testing.T, db *bun.DB, number, count int) int64 {
	t.Helper()
	ctx := context.Background()
	var file int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
		file_name, content_sha256, invoice_count, total_cents, exported_at)
		VALUES (?, ?, 'datev', 'care', ?, 'bestand.csv', repeat('a', 64), ?, 0, '2029-10-30 10:00:00+00') RETURNING id`,
		f.organization, number, f.run, count).Scan(ctx, &file))
	for _, invoice := range f.invoices {
		_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_invoice_id)
			VALUES (?, ?, ?)`, f.organization, file, invoice)
		require.NoError(t, err)
	}
	return file
}
