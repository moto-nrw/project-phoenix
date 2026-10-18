package api

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// staffDocument stores a staff document of the category the uploader's
// account uploaded daysAgo days back and returns its id and stored file name.
func (s retentionSuite) staffDocument(t *testing.T, tenantID, staffID, uploaderID int64, category string, daysAgo int) (int64, string) {
	t.Helper()
	var row struct {
		ID             int64  `bun:"id"`
		FilenameStored string `bun:"filename_stored"`
	}
	require.NoError(t, s.admin.NewRaw(`INSERT INTO users.staff_documents
		(tenant_id, staff_id, category, filename_display, filename_stored, size_bytes, content_type, uploaded_by, created_at, updated_at)
		SELECT ?, ?, ?, 'Bescheinigung.pdf', gen_random_uuid()::text || '.pdf', 1024, 'application/pdf', ?, at, at
		FROM (SELECT CURRENT_TIMESTAMP - make_interval(days => ?) AS at) AS upload
		RETURNING id, filename_stored`, tenantID, staffID, category, uploaderID, daysAgo).Scan(context.Background(), &row))
	return row.ID, row.FilenameStored
}

// fileCleanupIntent reports whether the file of the stored name waits in the
// staff document cleanup queue and is eligible for the next sweep.
func (s retentionSuite) fileCleanupIntent(t *testing.T, tenantID int64, filename string) bool {
	t.Helper()
	return s.count(t, `SELECT COUNT(*) FROM users.staff_document_file_cleanup
		WHERE tenant_id = ? AND filename_stored = ? AND cleaned_at IS NULL AND retry_after <= CURRENT_TIMESTAMP`,
		tenantID, filename) == 1
}

func TestNightlyRetentionDeletesSickNotesAfterTheirWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	staff, uploader := testpkg.CreateTestStaffWithAccount(t, suite.admin, "Frist", "Personal")
	otherStaff, otherUploader := testpkg.CreateTestStaffWithAccountForTenant(t, suite.admin, otherSchool, "Andere", "Schule")

	expired, expiredFile := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 400)
	kept, keptFile := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 364)
	contract, contractFile := suite.staffDocument(t, school, staff.ID, uploader.ID, "arbeitsvertrag", 400)
	payslip, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "lohnabrechnung", 400)
	otherExpired, otherFile := suite.staffDocument(t, otherSchool, otherStaff.ID, otherUploader.ID, "au_bescheinigung", 400)

	// A sick note the school deleted itself: its file is already gone and
	// only the metadata row is left.
	removed, removedFile := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 500)
	_, err := suite.admin.ExecContext(context.Background(), `UPDATE users.staff_documents
		SET deleted_at = CURRENT_TIMESTAMP - INTERVAL '100 days', deleted_by = ?, file_deleted_at = CURRENT_TIMESTAMP - INTERVAL '100 days'
		WHERE id = ?`, uploader.ID, removed)
	require.NoError(t, err)

	// The upload of the expired sick note finished its cleanup intent; the
	// retention must queue that intent again.
	_, err = suite.admin.ExecContext(context.Background(), `INSERT INTO users.staff_document_file_cleanup
		(tenant_id, staff_id, filename_stored, retry_after, cleaned_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '10 days')`, school, staff.ID, expiredFile)
	require.NoError(t, err)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.staff_documents", expired), "a sick note older than 365 days is deleted")
	assert.True(t, suite.fileCleanupIntent(t, school, expiredFile), "its file waits in the staff document cleanup queue")
	assert.False(t, suite.exists(t, "users.staff_documents", removed), "the row of a sick note the school deleted goes as well")
	assert.False(t, suite.fileCleanupIntent(t, school, removedFile), "a file already removed is not queued again")
	assert.True(t, suite.exists(t, "users.staff_documents", kept), "a younger sick note stays")
	assert.False(t, suite.fileCleanupIntent(t, school, keptFile))
	assert.True(t, suite.exists(t, "users.staff_documents", contract), "other categories keep their documents until the staff member leaves")
	assert.True(t, suite.exists(t, "users.staff_documents", payslip))
	assert.False(t, suite.fileCleanupIntent(t, school, contractFile))
	assert.True(t, suite.exists(t, "users.staff_documents", otherExpired), "the run of one school never touches another")
	assert.False(t, suite.fileCleanupIntent(t, otherSchool, otherFile))
	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffSickNoteRetention))
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, auditModels.DeletionTypeStaffSickNoteRetention))

	suite.run(t, school)

	assert.Equal(t, []int{2}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffSickNoteRetention), "a second run deletes and logs nothing")
}

func TestNightlyRetentionFollowsTheSchoolsRecordedSickNoteWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	staff, uploader := testpkg.CreateTestStaffWithAccount(t, suite.admin, "Lange", "Frist")
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.staff_sick_note_retention_days', '730'::jsonb)`, school)
	require.NoError(t, err)

	kept, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 400)
	expired, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 732)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "users.staff_documents", kept), "a longer contractual window keeps the sick note")
	assert.False(t, suite.exists(t, "users.staff_documents", expired))
}

func TestSickNoteRetentionRefusesACutoffInsideTheFrame(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	staff, uploader := testpkg.CreateTestStaffWithAccount(t, suite.admin, "Kurze", "Frist")
	recent, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 30)

	inSchool := func(query string, args ...any) error {
		return testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
			transaction, _ := tenant.TransactionFromContext(ctx)
			tx, ok := transaction.(bun.Tx)
			require.True(t, ok, "the school's tenant transaction")
			_, err := tx.ExecContext(ctx, query, args...)
			return err
		})
	}
	require.ErrorContains(t, inSchool(`SELECT users.delete_expired_staff_sick_notes(?, CURRENT_TIMESTAMP)`, school),
		"sick note cutoff exceeds retention window")
	require.ErrorContains(t, inSchool(`SELECT users.delete_expired_staff_sick_notes(?, CURRENT_TIMESTAMP - INTERVAL '1096 days')`, school),
		"sick note cutoff exceeds retention window")
	require.ErrorContains(t, inSchool(`DELETE FROM users.staff_documents WHERE id = ?`, recent),
		"permission denied for table staff_documents", "the tenant role keeps no direct DELETE on staff documents")
	assert.True(t, suite.exists(t, "users.staff_documents", recent))
}

func TestNightlyRetentionTreatsARecordedDefaultSickNoteWindowLikeTheDefault(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	staff, uploader := testpkg.CreateTestStaffWithAccount(t, suite.admin, "Gleiche", "Frist")
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.staff_sick_note_retention_days', '365'::jsonb)`, school)
	require.NoError(t, err)

	kept, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 364)
	expired, _ := suite.staffDocument(t, school, staff.ID, uploader.ID, "au_bescheinigung", 367)

	suite.run(t, school)

	assert.True(t, suite.exists(t, "users.staff_documents", kept))
	assert.False(t, suite.exists(t, "users.staff_documents", expired))
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, auditModels.DeletionTypeStaffSickNoteRetention))
}
