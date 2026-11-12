package integration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-ogs/project-phoenix/api"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Step 5b of #5552 (PR #6408): the batch command collects each row's
// retention consent and consent history and writes each kind in one bundled
// statement after the rows. The schedules stay in the row loop, so their
// created_by lock on the importing staff member keeps its place.

// sessionPID is the backend PID of the transaction's own session.
func sessionPID(t *testing.T, ctx context.Context, tx bun.Tx) int {
	t.Helper()
	var pid int
	require.NoError(t, tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid))
	return pid
}

// namedImportPool is a private pool over dsn whose sessions carry an
// application_name of their own, so a test can find its upload's sessions
// and no others (Befundklasse 13). A positive readTimeout replaces pgdriver's
// read timeout.
func namedImportPool(t *testing.T, dsn string, readTimeout time.Duration) (*bun.DB, string) {
	t.Helper()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("import-test-%d-%d", testpkg.Tenant(t), time.Now().UnixNano())
	query := parsed.Query()
	query.Set("application_name", name)
	if readTimeout > 0 {
		query.Set("read_timeout", readTimeout.String())
	}
	parsed.RawQuery = query.Encode()
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(testpkg.OpenPostgresSQL(parsed.String())))
	t.Cleanup(func() { _ = pool.Close() })
	return pool, name
}

// waitBlockedBy waits, within a hard deadline, until a session of the
// upload's own pool (application name upload) waits for a lock the test's
// holder session holds: pg_blocking_pids of the upload's PID names the
// holder's PID. No other session counts.
func waitBlockedBy(t *testing.T, db *bun.DB, upload string, holder int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND ?::int = ANY(pg_blocking_pids(pid))`, upload, holder).
			Scan(context.Background(), &blocked)
		return err == nil && blocked > 0
	}, 10*time.Second, 5*time.Millisecond, "the upload waits for the holder session")
}

// holdStaffMembership locks the importing staff member's membership row FOR
// UPDATE, which every created_by insert of a schedule waits for.
func holdStaffMembership(t *testing.T, ctx context.Context, db *bun.DB, staffID int64) (bun.Tx, int) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	pid := sessionPID(t, ctx, tx)
	var id int64
	require.NoError(t, tx.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, staffID).Scan(ctx, &id))
	return tx, pid
}

// TestStudentLeafBatchesKeepEachRowsInstant (Befundklasse 5): each consent
// carries the acceptance instant its row took, as on the row path; no row of
// the bundled statement shares a time stamp taken once for the segment.
func TestStudentLeafBatchesKeepEachRowsInstant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.OwnTenant(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := namedStudentBatch("Zeit", studentChunkRows)
	upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "instants.csv"))
	require.NoError(t, upload.err)
	observation := (*module.Observations)[0]
	require.Equal(t, 1, commandCount(observation, "record_privacy_consents"), "the consents went in one bundled statement")
	require.Zero(t, commandCount(observation, "record_privacy_consent"))

	var accepted []time.Time
	require.NoError(t, db.NewRaw(`SELECT c.accepted_at FROM users.privacy_consents c
		JOIN users.student_profiles s ON s.id = c.student_id JOIN users.persons p ON p.id = s.person_id
		WHERE c.tenant_id = ? ORDER BY p.first_name`, testpkg.Tenant(t)).Scan(context.Background(), &accepted))
	require.Len(t, accepted, studentChunkRows)
	for i := 1; i < len(accepted); i++ {
		require.True(t, accepted[i].After(accepted[i-1]), "row %d accepted after row %d, in processing order", i+1, i)
	}
}

// TestStudentLeafBatchesFormNoNewDeadlockPair (Befundklasse 6, PR #6408): a
// writer that locks the importing staff member's membership and then a
// guardian a later row reuses. The upload waits for the membership at the
// first schedule of its first row, before it touches that guardian, so the
// writer gets the guardian without waiting, on the row path as in the batch.
// With the schedules bundled after the rows (the PR's first draft), the
// batch would hold the guardian while waiting for the membership: 40P01.
func TestStudentLeafBatchesFormNoNewDeadlockPair(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, run := range []struct {
		name    string
		options api.ImportTestOptions
	}{
		{"batch path", api.ImportTestOptions{}},
		{"row path as before", api.ImportTestOptions{RowPathOnly: true}},
	} {
		t.Run(run.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			pool, name := namedImportPool(t, testpkg.PackageTestDSN(t), 0)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), run.options)
			require.NoError(t, err)
			actor := newImporter(t, db)
			reused := testpkg.CreateTestGuardianProfileWithExactEmail(t, db, testpkg.Tenant(t), testpkg.GuardianProfileWithEmail{
				FirstName: "Eltern", LastName: "Wieder", Email: fmt.Sprintf("wieder.%d@example.test", testpkg.Tenant(t))})
			rows := namedStudentBatch("Sperre", studentChunkRows)
			rows[6].Guardians[0].Email = *reused.Email
			// Rows 1, 3 and 5 carry guardians without e-mail, which the batch
			// path bundles after the rows (#5552 step 5c): they lock nothing
			// another writer can see, so the pair stays as it was.
			for _, i := range []int{1, 3, 5} {
				rows[i].Guardians[0].Email = ""
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holder, pid := holdStaffMembership(t, ctx, db, actor.staffID)
			done := uploadStudents(t, module, pool, actor, rows, "pair.csv")
			waitBlockedBy(t, db, name, pid)

			lockCtx, lockCancel := context.WithTimeout(ctx, 3*time.Second)
			defer lockCancel()
			started := time.Now()
			var id int64
			err = holder.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, reused.ID).Scan(lockCtx, &id)
			require.NoError(t, err, "the writer locks the guardian without waiting for the upload")
			require.Less(t, time.Since(started), time.Second, "no wait, so no deadlock to detect")
			require.NoError(t, holder.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.Deadlocks)
			require.Zero(t, observation.BatchesRetried)
			require.Zero(t, observation.BatchFallbacks)
			if run.options.RowPathOnly {
				require.Zero(t, commandCount(observation, "record_privacy_consents"))
				require.Zero(t, commandCount(observation, "create_new_student_guardians"))
			} else {
				require.Equal(t, 1, commandCount(observation, "record_privacy_consents"), "the batch path ran")
				require.Equal(t, 1, commandCount(observation, "create_new_student_guardians"), "the guardians without e-mail were bundled")
			}
		})
	}
}

// TestStudentLeafBatchesAddNoWait (Befundklasse 1): with a 1 s read timeout,
// a holder of the importing staff member's membership for 0.7 s delays only
// the first schedule of the row loop, once, as on the row path; the bundled
// statements wait for nothing. Bounded and unbounded end with 200, nothing
// is cancelled, nothing replays.
func TestStudentLeafBatchesAddNoWait(t *testing.T) {
	t.Parallel()
	const readTimeout, hold, limit = time.Second, 700 * time.Millisecond, 800 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	pool, name := namedImportPool(t, testpkg.IsolatedTestDSN(t), readTimeout)
	for _, bounded := range []bool{true, false} {
		t.Run(fmt.Sprintf("bounded=%t", bounded), func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{
				BatchStatementLimit: limit, UnboundedBatches: !bounded,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holder, pid := holdStaffMembership(t, ctx, db, actor.staffID)
			done := uploadStudents(t, module, pool, actor, namedStudentBatch("Warte", studentChunkRows), "wait.csv")
			waitBlockedBy(t, db, name, pid)
			// The hold time is the subject of the test.
			time.Sleep(hold)
			require.NoError(t, holder.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks, "the bound cancelled nothing")
			require.Zero(t, observation.BatchesRetried)
			require.Equal(t, 1, commandCount(observation, "record_privacy_consents"), "the batch path ran")
		})
	}
}

// TestStudentLeafBatchCommitOutcomeUnknown (table A, COMMIT → U): the COMMIT
// of a chunk that wrote its consents in one bundled statement fails after
// the server committed. The upload ends with 500 and no retry, the error
// says the outcome is unknown, and the chunk is durable as committed.
func TestStudentLeafBatchCommitOutcomeUnknown(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	testpkg.OwnTenant(t)
	var bundled, failed atomic.Bool
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)),
		decide: func(query string) error {
			if strings.HasPrefix(strings.TrimSpace(query), consentBatch) && strings.Contains(statementKind(query), "), (") {
				bundled.Store(true)
			}
			return nil
		},
		commit: func() error {
			if bundled.Load() && failed.CompareAndSwap(false, true) {
				return driver.ErrBadConn
			}
			return nil
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitStudents(t, uploadStudents(t, module, pool, actor, namedStudentBatch("Commit", studentChunkRows), "commit.csv"))
	require.True(t, failed.Load(), "the COMMIT fault fired")
	require.Error(t, upload.err)
	require.Equal(t, "500", uploadStatus(upload.err))
	var unknown interface{ CommitOutcomeUnknown() }
	require.ErrorAs(t, upload.err, &unknown, "the outcome of the COMMIT is unknown")
	observation := (*module.Observations)[0]
	require.Zero(t, observation.BatchesRetried, "no retry after an unknown COMMIT")
	require.Zero(t, observation.BatchFallbacks)
	require.Equal(t, studentChunkRows, countIn(t, db, studentCount), "the server committed the chunk")
}
