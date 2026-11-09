package integration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Faults, retries and waits of the student batch command (#5552 step 5; the
// fault table is in the PR). The rules are those of the class list (#5827):
// only a failure of the batch command itself (CreateChunk: the person
// statement and every owner write after it) replays the chunk.

const (
	personBatch      = "WITH staff_person_merge_gate AS MATERIALIZED"
	enrollmentWrite  = "WITH class_write_gate AS MATERIALIZED"
	studentCount     = `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`
	studentChunkRows = 8
)

// namedStudentBatch is studentBatch with its own first-name prefix, so a
// trigger can tell this file's persons from fixture persons, and with
// guardians of its own.
func namedStudentBatch(prefix string, n int) []importModels.StudentImportRow {
	rows := studentBatch(0, n)
	for i := range rows {
		name := fmt.Sprintf("%s%d", prefix, i)
		rows[i].FirstName = name
		rows[i].Guardians[0].LastName = name
		rows[i].Guardians[0].Email = strings.ToLower(name) + "@example.test"
	}
	return rows
}

// installPersonDeadlockScript fails every multi-row person insert of a
// "Deadlock" file with 40P01 and its first rowPathFailures single-row
// inserts too. Sequence values survive rollbacks, so the count spans retries.
func installPersonDeadlockScript(t *testing.T, db *bun.DB, rowPathFailures int) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), strings.ReplaceAll(fmt.Sprintf(`
		CREATE SEQUENCE public.person_row_deadlocks_N;
		GRANT USAGE ON SEQUENCE public.person_row_deadlocks_N TO phoenix_tenant;
		CREATE FUNCTION public.person_deadlock_script_N() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM inserted WHERE first_name LIKE 'Deadlock%%') THEN
				RETURN NULL;
			END IF;
			IF (SELECT count(*) FROM inserted) > 1 OR nextval('public.person_row_deadlocks_N') <= %d THEN
				RAISE EXCEPTION 'injected deadlock' USING ERRCODE = '40P01';
			END IF;
			RETURN NULL;
		END $$;
		CREATE TRIGGER person_deadlock_script_N AFTER INSERT ON users.persons
			REFERENCING NEW TABLE AS inserted FOR EACH STATEMENT EXECUTE FUNCTION public.person_deadlock_script_N()`, rowPathFailures), "_N", fmt.Sprintf("_%d", rowPathFailures)))
	require.NoError(t, err)
}

func runPersonDeadlockScript(t *testing.T, rowPathFailures int) (importModels.ImportObservation, int, error) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	installPersonDeadlockScript(t, db, rowPathFailures)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitStudents(t, uploadStudents(t, module, db, actor, namedStudentBatch("Deadlock", studentChunkRows), "deadlock.csv"))
	observation := (*module.Observations)[0]
	require.Equal(t, 1, observation.BatchFallbacks)
	return observation, countIn(t, db, studentCount), upload.err
}

// TestStudentChunkDeadlockFallbackKeepsTheRetryBudget: a 40P01 of the person
// batch consumes no attempt; the row path then fails three times and
// succeeds in the fourth attempt, as it does today.
func TestStudentChunkDeadlockFallbackKeepsTheRetryBudget(t *testing.T) {
	t.Parallel()
	observation, students, err := runPersonDeadlockScript(t, 3)
	require.NoError(t, err)
	require.Equal(t, 3, observation.BatchesRetried, "the row path used all three retries")
	require.Equal(t, studentChunkRows, students)
}

// TestStudentChunkDeadlockFallbackEndsLikeTodayAfterTheBudget: a fourth
// row-path deadlock ends with the error, as it does today.
func TestStudentChunkDeadlockFallbackEndsLikeTodayAfterTheBudget(t *testing.T) {
	t.Parallel()
	observation, students, err := runPersonDeadlockScript(t, 4)
	var state interface{ Field(byte) string }
	require.True(t, errors.As(err, &state), "%v", err)
	require.Equal(t, "40P01", state.Field('C'))
	require.Equal(t, 3, observation.BatchesRetried)
	require.Zero(t, students)
}

type studentFaultCase struct {
	point, prefix string
	nth           int32
	err           error
	prefix2       string
	err2          error
	want          faultOutcome
}

// runStudentFaultTable imports an eight-row file once per case over a pool
// that fails the chosen statement. faultOutcome.entries counts students.
func runStudentFaultTable(t *testing.T, cases []studentFaultCase) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	dsn := testpkg.IsolatedTestDSN(t)
	for _, c := range cases {
		name := c.point + " × " + c.err.Error()
		if c.err2 != nil {
			name += " then " + c.err2.Error()
		}
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			armed, armed2 := &atomic.Bool{}, &atomic.Bool{}
			pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
				Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)), armed: armed, prefix: c.prefix,
				nth: c.nth, err: c.err, matches: &atomic.Int32{},
				prefix2: c.prefix2, err2: c.err2, armed2: armed2,
			})))
			t.Cleanup(func() { _ = pool.Close() })
			module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
			require.NoError(t, err)
			actor := newImporter(t, db)
			armed.Store(true)
			armed2.Store(c.prefix2 != "")
			upload := awaitStudents(t, uploadStudents(t, module, pool, actor, namedStudentBatch("Fault", studentChunkRows), "fault.csv"))
			require.False(t, armed.Load(), "the fault fired")
			require.False(t, armed2.Load(), "the second fault fired")
			observation := (*module.Observations)[0]
			got := faultOutcome{retries: observation.BatchesRetried, fallbacks: observation.BatchFallbacks, entries: countIn(t, db, studentCount)}
			var state interface{ Field(byte) string }
			switch {
			case upload.err == nil:
			case strings.Contains(upload.err.Error(), "chunk rollback failed"):
				got.failsWith = "rollback failed"
			case errors.Is(upload.err, driver.ErrBadConn):
				got.failsWith = "bad connection"
			case errors.As(upload.err, &state):
				got.failsWith = state.Field('C')
			default:
				got.failsWith = upload.err.Error()
			}
			require.Equal(t, c.want, got, "%v", upload.err)
		})
	}
}

// TestStudentChunkFaultTable: the person statement and the owner writes
// after it are the batch command and replay; a broken connection is
// returned; the statements around them behave as on the row path.
func TestStudentChunkFaultTable(t *testing.T) {
	t.Parallel()
	replayed := faultOutcome{fallbacks: 1, entries: studentChunkRows}
	retried := faultOutcome{retries: 1, entries: studentChunkRows}
	broken := faultOutcome{failsWith: "bad connection"}
	runStudentFaultTable(t, []studentFaultCase{
		{point: "person batch", prefix: personBatch, err: injectedSQLState("40P01"), want: replayed},
		{point: "person batch", prefix: personBatch, err: injectedSQLState("40001"), want: replayed},
		{point: "person batch", prefix: personBatch, err: injectedSQLState("57014"), want: replayed},
		{point: "person batch", prefix: personBatch, err: injectedSQLState("55P03"), want: replayed},
		{point: "person batch", prefix: personBatch, err: injectedSQLState("23505"), want: replayed},
		{point: "person batch", prefix: personBatch, err: driver.ErrBadConn, want: broken},
		{point: "person batch", prefix: personBatch, err: injectedSQLState("40P01"), prefix2: rollbackTo, err2: driver.ErrBadConn, want: faultOutcome{failsWith: "rollback failed"}},
		// The first enrollment statement of the chunk runs inside the batch
		// command, after the person statement.
		{point: "enrollment in the batch", prefix: enrollmentWrite, nth: 1, err: injectedSQLState("40P01"), want: replayed},
		{point: "enrollment in the batch", prefix: enrollmentWrite, nth: 1, err: injectedSQLState("57014"), want: replayed},
		{point: "enrollment in the batch", prefix: enrollmentWrite, nth: 1, err: injectedSQLState("23514"), want: replayed},
		{point: "enrollment in the batch", prefix: enrollmentWrite, nth: 1, err: driver.ErrBadConn, want: broken},
		// ResolveChunk is a protected read: it falls back to FindExisting per
		// row (ADR F3), with no retry and no replay.
		{point: "ResolveChunk person read", prefix: personRead, nth: 1, err: injectedSQLState("40P01"), want: faultOutcome{entries: studentChunkRows}},
		{point: "bound set", prefix: boundSet, err: injectedSQLState("40P01"), want: retried},
		{point: "bound set", prefix: boundSet, err: driver.ErrBadConn, want: broken},
		{point: "chunk savepoint", prefix: savepoint, err: driver.ErrBadConn, want: broken},
		{point: "checkpoint", prefix: checkpoint, err: injectedSQLState("40P01"), want: retried},
	})
}

// holdStudentTag inserts an uncommitted person wearing the card; a person
// insert with the same card waits for it until the holder rolls back.
func holdStudentTag(t *testing.T, db *bun.DB, tag string) bun.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), `INSERT INTO users.persons (tenant_id, first_name, last_name, tag_id) VALUES (?, 'Halter', 'Karte', ?)`, testpkg.Tenant(t), tag)
	require.NoError(t, err)
	return tx
}

// TestStudentCardWaitsStayOnePerStatement is test (a) of E13 for the person
// statement, after rows with a card moved to the row path (ADR 0067
// addendum, unique entries of new rows). On 41f3e4f9f4 the person statement
// waited for every uncommitted holder of one of its cards in one read:
// unbounded, two holders of 0.7 s each outlasted the 1 s read timeout (i/o
// timeout, a 500); bounded, the attempt was cancelled and replayed. Now each
// card row waits for its holder in a statement of its own, as on the row
// path, so neither variant times out, nothing is cancelled and nothing
// replays.
func TestStudentCardWaitsStayOnePerStatement(t *testing.T) {
	t.Parallel()
	const readTimeout, hold, limit = time.Second, 700 * time.Millisecond, 800 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	pool := shortReadTimeoutPool(t, readTimeout)
	pool.SetMaxOpenConns(2)
	for _, bounded := range []bool{true, false} {
		t.Run(fmt.Sprintf("bounded=%t", bounded), func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{
				BatchStatementLimit: limit, UnboundedBatches: !bounded,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := namedStudentBatch("Karte", studentChunkRows)
			rows[0].TagID = testpkg.CreateTestRFIDCard(t, db, "EINS").ID
			rows[1].TagID = testpkg.CreateTestRFIDCard(t, db, "ZWEI").ID
			first, second := holdStudentTag(t, db, rows[0].TagID), holdStudentTag(t, db, rows[1].TagID)
			done := uploadStudents(t, module, pool, actor, rows, "bounded.csv")
			waitForLockWaiters(t, db)
			started := time.Now()
			// The hold times are the subject of the test.
			time.Sleep(hold)
			require.NoError(t, first.Rollback())
			waitForLockWaiters(t, db)
			time.Sleep(hold)
			require.NoError(t, second.Rollback())
			upload := awaitStudents(t, done)
			require.Greater(t, time.Since(started), readTimeout, "the waits together exceed the read timeout")

			require.NoError(t, upload.err, "no read waited for both holders")
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			require.Zero(t, upload.result.ErrorCount)
			require.Equal(t, studentChunkRows, countIn(t, db, studentCount))
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks, "the bound cancelled nothing")
			require.Zero(t, observation.BatchesRetried)
			require.Positive(t, commandCount(observation, "create_persons"), "the rows without a card took the batch path")
		})
	}
}

// TestStudentBoundedReplayMatchesTheRowPath is test (b) of E13: the staff
// person merge holds its exclusive lock past the bound, the person statement
// is cancelled, the chunk replays in a new transaction and waits there
// without a bound; result and persistence equal a run on the row path.
func TestStudentBoundedReplayMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	run := func(t *testing.T, options api.ImportTestOptions, merge bool) (studentOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		testpkg.CreateTestStudent(t, db, "Runtime0-3", "Evidence", "4A")
		var held bun.Tx
		if merge {
			held, err = db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = held.Rollback() })
			_, err = held.ExecContext(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, fmt.Sprintf("staff-person-merge:%d", testpkg.Tenant(t)))
			require.NoError(t, err)
		}
		done := uploadStudents(t, module, db, actor, studentBatch(0, studentChunkRows), "merge.csv")
		if merge {
			waitForLockWaiters(t, db)
			// The hold outlasts the bound, the subject of the test.
			time.Sleep(600 * time.Millisecond)
			require.NoError(t, held.Rollback())
		}
		upload := awaitStudents(t, done)
		require.NoError(t, upload.err)
		return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var bounded, rowPath studentOutcome
	t.Run("bounded attempt replays", func(t *testing.T) {
		var observation importModels.ImportObservation
		bounded, observation = run(t, api.ImportTestOptions{BatchStatementLimit: 200 * time.Millisecond}, true)
		require.Equal(t, 1, observation.BatchFallbacks)
		require.Zero(t, observation.BatchesRetried)
	})
	t.Run("row path", func(t *testing.T) {
		rowPath, _ = run(t, api.ImportTestOptions{RowPathOnly: true}, false)
	})
	require.Equal(t, rowPath, bounded)
	require.Equal(t, studentChunkRows-1, bounded.Created)
	require.Equal(t, 1, bounded.Errors)
}

// TestParallelStudentImportsOfOneSchool: two processes import into one
// school at once, the first holding its chunk open over the quota lock; two
// uploads in one process serialize over the import lock. No deadlock
// arises and every upload commits all its children.
func TestParallelStudentImportsOfOneSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.Tenant(t)
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw(`SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(context.Background(), &count))
		return count
	}
	before := deadlocks()
	_, err := db.ExecContext(context.Background(), `
		CREATE FUNCTION public.slow_first_student_checkpoint() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.filename = 'first.csv' THEN PERFORM pg_sleep(0.5); END IF; RETURN NULL; END $$;
		CREATE TRIGGER slow_first_student_checkpoint AFTER INSERT ON audit.data_imports FOR EACH ROW EXECUTE FUNCTION public.slow_first_student_checkpoint()`)
	require.NoError(t, err)
	processA, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	processB, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	doneA := uploadStudents(t, processA, db, actor, namedStudentBatch("ErsterProzess", 30), "first.csv")
	require.Eventually(t, func() bool {
		var sleeping int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event = 'PgSleep'`).Scan(context.Background(), &sleeping)
		return err == nil && sleeping > 0
	}, 10*time.Second, 5*time.Millisecond, "the first upload holds its chunk open")
	doneB := uploadStudents(t, processB, db, actor, namedStudentBatch("ZweiterProzess", 30), "second.csv")
	sameProcess := []<-chan studentUpload{
		uploadStudents(t, processB, db, actor, namedStudentBatch("GleicherProzessA", 10), "same-0.csv"),
		uploadStudents(t, processB, db, actor, namedStudentBatch("GleicherProzessB", 10), "same-1.csv"),
	}
	for _, done := range append([]<-chan studentUpload{doneA, doneB}, sameProcess...) {
		upload := awaitStudents(t, done)
		require.NoError(t, upload.err)
		require.Zero(t, upload.result.ErrorCount)
	}
	require.Equal(t, 80, countIn(t, db, studentCount), "every child once")
	require.Equal(t, before, deadlocks(), "no deadlock")
}

// childQuotaLock is School Membership's advisory lock of the counting
// writes (student_counts.go:54): a parallel upload holds it from its first
// child until it commits.
const childQuotaLock = `SELECT pg_advisory_xact_lock(1802464884, ?::int)`

// TestStudentCardRowsFormNoNewDeadlockPair (Befundklasse 6, #5552 step 5a):
// a parallel row-path upload of the same school holds the child quota after
// its first child and then creates a child with the card of a later row of
// this upload. On the row path of development this upload has not claimed
// that card yet when it waits for the quota, so the other upload proceeds.
// The batch path must end the same: the card row is not in the person
// batch, which claims nothing before the quota but cardless persons. With
// the card in the batch (41f3e4f9f4) the other upload waited for this one
// while this one waited for the quota: a deadlock the row path never forms.
func TestStudentCardRowsFormNoNewDeadlockPair(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, run := range []struct {
		name    string
		options api.ImportTestOptions
	}{
		{"batch path", api.ImportTestOptions{}},
		{"row path as on development", api.ImportTestOptions{RowPathOnly: true}},
	} {
		t.Run(run.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), run.options)
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := namedStudentBatch("Paar", studentChunkRows)
			card := testpkg.CreateTestRFIDCard(t, db, "PAAR").ID
			rows[6].TagID = card // after six cardless rows, so the batch path runs

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			other, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = other.Rollback() })
			_, err = other.ExecContext(ctx, childQuotaLock, testpkg.Tenant(t))
			require.NoError(t, err)
			var holder int
			require.NoError(t, other.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holder))

			done := uploadStudents(t, module, db, actor, rows, "pair.csv")
			waitForBlockedBy(t, db, holder) // this upload waits for the quota the other holds

			// The other upload's next child wears the card of row 8.
			insertCtx, insertCancel := context.WithTimeout(ctx, 3*time.Second)
			defer insertCancel()
			started := time.Now()
			_, err = other.ExecContext(insertCtx, `INSERT INTO users.persons (tenant_id, first_name, last_name, tag_id) VALUES (?, 'Andere', 'Anlage', ?)`, testpkg.Tenant(t), card)
			require.NoError(t, err, "the other upload claims the card without waiting for this one")
			require.Less(t, time.Since(started), time.Second, "no wait, so no deadlock to detect")
			require.NoError(t, other.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			require.Zero(t, (*module.Observations)[0].Deadlocks)
			require.Zero(t, (*module.Observations)[0].BatchesRetried)
			if run.options.RowPathOnly {
				require.Zero(t, commandCount((*module.Observations)[0], "create_persons"))
			} else {
				require.Positive(t, commandCount((*module.Observations)[0], "create_persons"), "the batch path ran")
			}
		})
	}
}

// waitForBlockedBy waits, at most 10 s, until a session waits for a lock the
// session holder holds; waiters of other tests on the same clone do not count.
func waitForBlockedBy(t *testing.T, db *bun.DB, holder int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND ?::int = ANY(pg_blocking_pids(pid))`, holder).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 5*time.Millisecond, "no session waits for the holder")
}
