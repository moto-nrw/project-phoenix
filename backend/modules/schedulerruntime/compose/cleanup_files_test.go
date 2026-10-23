package compose_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func seedSickNoteFile(t *testing.T, admin *bun.DB, school int64, publicDir string) (int64, string) {
	t.Helper()
	staff, uploader := testpkg.CreateTestStaffWithAccountForTenant(t, admin, school, "Test", "Dokument")
	var row struct {
		ID       int64
		Filename string
	}
	require.NoError(t, admin.NewRaw(`INSERT INTO users.staff_documents
 (tenant_id, staff_id, category, filename_display, filename_stored, size_bytes, content_type, uploaded_by, created_at, updated_at)
 SELECT ?, ?, 'au_bescheinigung', 'Test.pdf', gen_random_uuid()::text || '.pdf', 18, 'application/pdf', ?, at, at
 FROM (SELECT CURRENT_TIMESTAMP - INTERVAL '400 days' AS at) AS upload
 RETURNING id, filename_stored AS filename`, school, staff.ID, uploader.ID).Scan(context.Background(), &row))
	dir := filepath.Join(publicDir, "uploads", "staff-documents", strconv.FormatInt(school, 10))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, row.Filename)
	require.NoError(t, os.WriteFile(path, []byte("synthetic document"), 0o600))
	return row.ID, path
}

func TestSoftDeletedSchoolRetentionRemovesSickNoteFile(t *testing.T) {
	t.Parallel()
	admin := testpkg.SetupIsolatedTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, admin)
	other, _ := testpkg.CreateTestTenant(t, admin)
	publicDir := t.TempDir()
	expired, expiredPath := seedSickNoteFile(t, admin, school, publicDir)
	otherDoc, otherPath := seedSickNoteFile(t, admin, other, publicDir)
	_, err := admin.ExecContext(context.Background(), `UPDATE platform.schools SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, school)
	require.NoError(t, err)
	_, err = admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
 VALUES (?, 'gdpr.data_cleanup_enabled', 'false'::jsonb)`, other)
	require.NoError(t, err)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	now := func() time.Time { return time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin) }
	deps, err := owners.NewSchedulerFileRetentionTestDependencies(testpkg.NewDatabase(t, serve), testpkg.TenantRuntime(t, serve), now, publicDir)
	require.NoError(t, err)
	var workerErr error
	deps.Tracer.Failure = func(_ context.Context, _, _ string, err error) { workerErr = err }
	scheduler.NewCleanupTestRunner(deps)()
	assert.NoError(t, workerErr)
	var exists bool
	require.NoError(t, admin.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_documents WHERE id = ?)`, expired).Scan(context.Background(), &exists))
	require.False(t, exists, "nightly retention removes the overdue sick note metadata")
	require.FileExists(t, expiredPath, "file removal follows in the recovery pass")
	scheduler.NewFileCleanupTestRunner(deps)()
	assert.NoError(t, workerErr)
	assert.NoFileExists(t, expiredPath, "a soft-deleted school's sick note must lose its stored bytes")
	var pending int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM users.staff_document_file_cleanup WHERE tenant_id = ? AND cleaned_at IS NULL`, school).Scan(context.Background(), &pending))
	assert.Zero(t, pending, "successful removal completes the durable intent")
	assert.FileExists(t, otherPath, "another school's unqueued file must stay")
	require.NoError(t, admin.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_documents WHERE id = ?)`, otherDoc).Scan(context.Background(), &exists))
	assert.True(t, exists, "another school's retention pause remains effective")
}
