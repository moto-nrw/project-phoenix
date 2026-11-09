package integration

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Protected reads of the row path and of the validation under E19 (#5552
// step 5a, table A of the PR, ADR 0067 addendum): when the read fails and
// its savepoint rolls back, the rows read one by one as before; when a
// savepoint statement of the read fails, the upload ends with 500, without
// retry and without a fallback onto the transaction. The same holds for a
// savepoint statement of an owner command in a row write.

// readFault fails the first statement matching match with err, then the
// cleanup-th savepoint cleanup statement after it with err2.
type readFault struct {
	match   func(query string) bool
	err     error
	cleanup int
	err2    error
}

// faultPool is a private pool on the package clone that injects fault once
// armed is set; fired reports which faults fired.
func faultPool(t *testing.T, dsn string, armed *atomic.Bool, fault readFault) (*bun.DB, func() (bool, bool)) {
	t.Helper()
	var mu sync.Mutex
	primary, cleaned, cleanups := false, false, 0
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)),
		decide: func(query string) error {
			if !armed.Load() {
				return nil
			}
			mu.Lock()
			defer mu.Unlock()
			switch {
			case !primary && fault.match(strings.TrimSpace(query)):
				primary = true
				return fault.err
			case primary && !cleaned && fault.cleanup > 0 && isSavepointCleanup(query):
				cleanups++
				if cleanups == fault.cleanup {
					cleaned = true
					return fault.err2
				}
			}
			return nil
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	return pool, func() (bool, bool) {
		mu.Lock()
		defer mu.Unlock()
		return primary, cleaned
	}
}

func prefixed(prefix string) func(string) bool {
	return func(query string) bool { return strings.HasPrefix(query, prefix) }
}

// nth matches the nth statement starting with prefix.
func nth(prefix string, n int) func(string) bool {
	var seen atomic.Int32
	return func(query string) bool {
		return strings.HasPrefix(query, prefix) && seen.Add(1) == int32(n)
	}
}

type protectedReadCase struct {
	name  string
	fault readFault
	lost  bool // E19: 500 without retry; otherwise the read falls back
}

// TestRowPathProtectedReadsUnderE19: the name read of BatchWriteConfig on the
// row path of a create, and an owner savepoint in a row write.
func TestRowPathProtectedReadsUnderE19(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	// On the row path the first SAVEPOINT and the first person read are the
	// protected name read's; the second SAVEPOINT is an owner's in the first
	// row write (the row's own savepoint is skipped, WithBatchRowWrite).
	cases := []protectedReadCase{
		{"name read fails, rollback succeeds", readFault{match: prefixed(personRead), err: injectedSQLState("40P01")}, false},
		{"name read fails, rollback fails", readFault{match: prefixed(personRead), err: injectedSQLState("40P01"), cleanup: 1, err2: driver.ErrBadConn}, true},
		{"name read fails, rollback 40P01", readFault{match: prefixed(personRead), err: fmt.Errorf("injected read failure"), cleanup: 1, err2: injectedSQLState("40P01")}, true},
		{"name read fails, release after rollback fails", readFault{match: prefixed(personRead), err: injectedSQLState("40001"), cleanup: 2, err2: injectedSQLState("40001")}, true},
		{"name read savepoint fails", readFault{match: nth(savepoint, 1), err: injectedSQLState("40P01")}, true},
		{"name read release fails", readFault{match: nth(release, 1), err: injectedSQLState("40P01")}, true},
		{"owner savepoint in a row write fails", readFault{match: nth(savepoint, 2), err: injectedSQLState("40P01")}, true},
		{"owner savepoint in a row write fails without SQLSTATE", readFault{match: nth(savepoint, 2), err: fmt.Errorf("injected savepoint failure")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			armed := &atomic.Bool{}
			pool, fired := faultPool(t, dsn, armed, c.fault)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{RowPathOnly: true})
			require.NoError(t, err)
			actor := newImporter(t, db)
			armed.Store(true)
			upload := awaitStudents(t, uploadStudents(t, module, pool, actor, namedStudentBatch("Lesen", studentChunkRows), "read.csv"))
			armed.Store(false)
			primary, cleaned := fired()
			require.True(t, primary, "the fault fired")
			require.Equal(t, c.fault.cleanup > 0, cleaned, "the cleanup fault fired")
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchesRetried, "no retry")
			if c.lost {
				require.Error(t, upload.err)
				require.Contains(t, upload.err.Error(), "a savepoint of the write failed")
				require.Equal(t, "500", uploadStatus(upload.err))
				require.Zero(t, countIn(t, db, studentCount))
				return
			}
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			require.Zero(t, upload.result.ErrorCount)
		})
	}
}

// TestStaffRoleReadUnderE19: the one read of the role permissions in the
// staff validation (#5887).
func TestStaffRoleReadUnderE19(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	roleRead := func(query string) bool { return strings.Contains(query, "auth.role_permissions") }
	cases := []protectedReadCase{
		{"role read fails, rollback succeeds", readFault{match: roleRead, err: injectedSQLState("40P01")}, false},
		{"role read fails, rollback fails", readFault{match: roleRead, err: injectedSQLState("40P01"), cleanup: 1, err2: driver.ErrBadConn}, true},
		{"role read fails, release after rollback fails", readFault{match: roleRead, err: fmt.Errorf("injected read failure"), cleanup: 2, err2: injectedSQLState("57014")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			armed := &atomic.Bool{}
			pool, fired := faultPool(t, dsn, armed, c.fault)
			module, err := api.NewImportTestModuleWithAuth(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{}, api.WithAuthTestMailer(testpkg.NewCapturingMailer()))
			require.NoError(t, err)
			actor := newImporter(t, db)
			namedTenantRole(t, db, "Betreuungskraft")
			rows := make([]importModels.StaffImportRow, 2)
			for i := range rows {
				rows[i] = importModels.StaffImportRow{FirstName: fmt.Sprintf("Rollenread%d", i), LastName: "Rolle", RoleName: "Betreuungskraft",
					Email: fmt.Sprintf("rollenread%d@example.test", i), PersonnelNumber: fmt.Sprintf("RR-%d", i)}
			}
			ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, pool))
			ctx = importModels.ContextWithImporterPermissions(ctx, []string{"admin:*"})
			armed.Store(true)
			result, err := module.StaffImport.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
				Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
			}, importModels.BatchAudit{EntityType: "staff", Filename: "roles.csv", AccountID: actor.accountID})
			armed.Store(false)
			primary, cleaned := fired()
			require.True(t, primary, "the fault fired")
			require.Equal(t, c.fault.cleanup > 0, cleaned, "the cleanup fault fired")
			staff := countIn(t, db, `SELECT count(*) FROM users.persons WHERE tenant_id = ? AND first_name LIKE 'Rollenread%'`)
			if c.lost {
				require.Error(t, err)
				require.Contains(t, err.Error(), "a savepoint of the write failed")
				require.Zero(t, staff, "the upload ended before any write")
				return
			}
			require.NoError(t, err)
			require.Zero(t, result.ErrorCount, "the rows read their role one by one")
			require.Equal(t, len(rows), staff)
		})
	}
}

// isSavepointCleanup is a statement that cleans up after a failure inside a
// savepoint: its rollback or the release after it.
func isSavepointCleanup(query string) bool {
	query = strings.TrimSpace(query)
	return strings.HasPrefix(query, "ROLLBACK TO SAVEPOINT ") || strings.HasPrefix(query, "RELEASE SAVEPOINT ")
}

// TestStudentPreviewReadUnderE19: the bulk name read of the dry run. A read
// that fails and rolls back falls back to the reads per row as before; a
// failed savepoint statement of the read ends the preview with 500.
func TestStudentPreviewReadUnderE19(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	cases := []protectedReadCase{
		{"preview read fails, rollback succeeds", readFault{match: prefixed(personRead), err: injectedSQLState("40P01")}, false},
		{"preview read fails, rollback fails", readFault{match: prefixed(personRead), err: injectedSQLState("40P01"), cleanup: 1, err2: driver.ErrBadConn}, true},
		{"preview read fails, rollback 40P01", readFault{match: prefixed(personRead), err: fmt.Errorf("injected read failure"), cleanup: 1, err2: injectedSQLState("40P01")}, true},
		{"preview read fails, release after rollback fails", readFault{match: prefixed(personRead), err: injectedSQLState("40001"), cleanup: 2, err2: fmt.Errorf("injected release failure")}, true},
		{"preview read savepoint fails", readFault{match: nth(savepoint, 1), err: injectedSQLState("40P01")}, true},
		{"preview read release fails", readFault{match: nth(release, 1), err: injectedSQLState("57014")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			armed := &atomic.Bool{}
			pool, fired := faultPool(t, dsn, armed, c.fault)
			module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
			require.NoError(t, err)
			actor := newImporter(t, db)
			ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, pool))
			armed.Store(true)
			result, err := module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
				Rows: namedStudentBatch("Vorschau", studentChunkRows), Mode: importModels.ImportModeCreate, UserID: actor.staffID,
				SkipInvalidRows: true, DryRun: true,
			}, importModels.BatchAudit{EntityType: "student", Filename: "preview.csv", AccountID: actor.accountID})
			armed.Store(false)
			primary, cleaned := fired()
			require.True(t, primary, "the fault fired")
			require.Equal(t, c.fault.cleanup > 0, cleaned, "the cleanup fault fired")
			if c.lost {
				require.Error(t, err)
				require.Contains(t, err.Error(), "a savepoint of the write failed")
				require.Equal(t, "500", uploadStatus(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, studentChunkRows, result.CreatedCount, "every row previews as a create")
			require.Zero(t, result.ErrorCount)
		})
	}
}
