package compose_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTenantExportRejectsPartialFilesAndForeignReferences(t *testing.T) {
	t.Parallel()
	db, catalog, a, b := exportFixture(t)
	uploads := fixtureUploads(t, db, catalog, a, b)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	avatar := "avatars/global/" + strconv.FormatInt(a, 10) + ".bin"
	require.NoError(t, os.Remove(filepath.Join(uploads, avatar)))
	assertFailedExport(t, exporter, a, uploads)
	require.NoError(t, os.WriteFile(filepath.Join(uploads, avatar), []byte("synthetic-restored-original"), 0o600))
	foreign := "/uploads/student-photos/" + strconv.FormatInt(b, 10) + "_original.bin"
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", foreign, a).Exec(t.Context())
	require.NoError(t, err)
	assertFailedExport(t, exporter, a, uploads)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", "/uploads/"+avatar, a).Exec(t.Context())
	require.NoError(t, err)
	key := "student-photos/" + strconv.FormatInt(a, 10) + "_linked.bin"
	require.NoError(t, os.Symlink(filepath.Join(uploads, "student-photos", strconv.FormatInt(b, 10)+"_original.bin"), filepath.Join(uploads, key)))
	assertFailedExport(t, exporter, a, uploads)
	require.NoError(t, os.Remove(filepath.Join(uploads, key)))
	_, err = db.NewRaw("UPDATE documents.files SET filename_stored = 'missing.bin' WHERE id = ?", a).Exec(t.Context())
	require.NoError(t, err)
	assertFailedExport(t, exporter, a, uploads)
	_, err = db.NewRaw("UPDATE documents.files SET filename_stored = 'original.bin' WHERE id = ?", a).Exec(t.Context())
	require.NoError(t, err)
	// A fresh attempt after resolving the failure succeeds; no partial files
	// from a prior attempt are reused.
	out := filepath.Join(t.TempDir(), "complete")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: uploads}))
	readExportManifest(t, out)
}

func assertFailedExport(t *testing.T, exporter organizationtenancy.TenantExporter, school int64, uploads string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "partial")
	err := exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, Output: out, Uploads: uploads})
	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
	require.NotContains(t, err.Error(), uploads, "errors must not disclose original names")
}

func TestTenantExportDatabaseFailureCannotComplete(t *testing.T) {
	t.Parallel()
	db, _, a, _ := exportFixture(t)
	_, err := db.ExecContext(t.Context(), "DROP TABLE users.teachers")
	require.NoError(t, err)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	assertFailedExport(t, exporter, a, t.TempDir())
}

type snapshotExportHook struct {
	*testpkg.QueryCounter
	once   sync.Once
	err    error
	action func() error
}

func (h *snapshotExportHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	h.QueryCounter.AfterQuery(ctx, event)
	if !strings.HasPrefix(event.Query, "SELECT row_to_json(export_row)") {
		return
	}
	h.once.Do(func() { h.err = h.action() })
}

func TestTenantExportSnapshotIgnoresConcurrentWrites(t *testing.T) {
	t.Parallel()
	db, _, a, _ := exportFixture(t)
	hook := &snapshotExportHook{QueryCounter: testpkg.NewQueryCounter(), action: func() error {
		_, err := db.NewRaw("INSERT INTO facilities.rooms (id,tenant_id,fixture_school,fixture_label) VALUES (?,?,?,?)", -a, a, a, "synthetic-concurrent-row").Exec(t.Context())
		return err
	}}
	db.AddQueryHook(hook) //nolint:staticcheck // observe the pool the export already holds
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: t.TempDir()}))
	require.NoError(t, hook.err)
	data, err := os.ReadFile(filepath.Join(out, "facilities.rooms.jsonl"))
	require.NoError(t, err)
	require.NotContains(t, string(data), "synthetic-concurrent-row")
}

func TestTenantExportWriteFailureCannotComplete(t *testing.T) {
	t.Parallel()
	db, _, a, _ := exportFixture(t)
	out := filepath.Join(t.TempDir(), "partial")
	hook := &snapshotExportHook{QueryCounter: testpkg.NewQueryCounter(), action: func() error { return os.Mkdir(filepath.Join(out, "active.activity_session_attendance.jsonl"), 0o700) }}
	db.AddQueryHook(hook) //nolint:staticcheck // inject a disk failure on the pool the export holds
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	err = exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: t.TempDir()})
	require.NoError(t, hook.err)
	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
}

func TestTenantExportReaderCannotWriteOrBeUsedByHTTPRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var canWrite, canLogin, canAssume bool
	require.NoError(t, db.NewRaw("SELECT has_table_privilege('phoenix_tenant_export','users.persons','INSERT'), rolcanlogin, pg_has_role('phoenix_auth','phoenix_tenant_export','MEMBER') FROM pg_roles WHERE rolname='phoenix_tenant_export'").Scan(t.Context(), &canWrite, &canLogin, &canAssume))
	require.False(t, canWrite)
	require.False(t, canLogin)
	require.False(t, canAssume)
}
