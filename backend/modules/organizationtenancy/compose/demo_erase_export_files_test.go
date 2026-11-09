package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// leaveVisitorExportFiles records, as the office would by downloading, a
// Sollstellung that held the school's monthly invoice and a PAIN.008 file
// that held its Jahresbescheid (#4693). It answers the files.
func leaveVisitorExportFiles(t *testing.T, db *bun.DB, schoolID int64) []int64 {
	t.Helper()
	var files []int64
	require.NoError(t, db.NewRaw(`WITH invoice AS (
			SELECT id, organization_id, run_id FROM parent_billing.care_invoices WHERE school_id = ?0 ORDER BY id LIMIT 1),
		file AS (
			INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
				content_sha256, invoice_count, total_cents)
			SELECT organization_id, ?0::bigint * 2, 'sollstellung', 'care', run_id, 'sollstellung.csv', repeat('a', 64), 1, 3200
			FROM invoice RETURNING id, organization_id)
		INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_invoice_id)
		SELECT file.organization_id, file.id, invoice.id FROM file, invoice RETURNING export_file_id`, schoolID).
		Scan(t.Context(), &files))
	var notice []int64
	require.NoError(t, db.NewRaw(`WITH notice AS (
			SELECT id, organization_id FROM parent_billing.care_notices WHERE school_id = ?0 ORDER BY id LIMIT 1),
		file AS (
			INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
				content_sha256, invoice_count, total_cents, reference)
			SELECT organization_id, ?0::bigint * 2 + 1, 'pain008', 'notice', 202610, 'lastschrift.xml', repeat('b', 64), 1, 3200,
				'MOTO-DEMO' FROM notice RETURNING id, organization_id)
		INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_notice_id)
		SELECT file.organization_id, file.id, notice.id FROM file, notice RETURNING export_file_id`, schoolID).
		Scan(t.Context(), &notice))
	return append(files, notice...)
}

// The export files that held an invoice or a Jahresbescheid of a retired
// demo school go with the visitor (erasure of 1.15.843, tables of 1.15.851, #6015); a file of a school of
// the same Träger that is still open stays with its links.
func TestDemoSchoolQueueErasesTheExportFilesOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	files := leaveVisitorExportFiles(t, db, schoolID)
	require.Len(t, files, 2, "a Sollstellung and a PAIN.008 file of the retired school")
	openFiles := leaveVisitorExportFiles(t, db, openID)
	require.Len(t, openFiles, 2)

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "parent_billing.export_files", files), "no file that held the school's invoices")
	assert.Equal(t, len(openFiles), remainingRows(t, db, "parent_billing.export_files", openFiles), "the open school keeps its files")
	var links int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.export_file_invoices WHERE export_file_id IN (?)`,
		bun.List(openFiles)).Scan(t.Context(), &links))
	assert.Equal(t, 2, links, "and their links")
}
