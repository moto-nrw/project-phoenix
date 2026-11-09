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
	importService "github.com/moto-nrw/project-phoenix/modules/dataimport/internal/application/importservice"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Chunk executor of the class-list import (#5827, ADR 0067 addendum of
// 2026-11-08, point 2 and "Jeder Schritt prüft").

// probedClassList is the real class-list config with what the class list
// itself never produces: a validation warning, a failing match, a failing
// ResolveChunk and a segment boundary. It drives the real executor.
type probedClassList struct {
	*importService.ClassListImportConfig
	warnLastName   string            // rows with this last name carry a warning
	failFirstName  string            // FindExisting fails for this first name
	failResolve    bool              // ResolveChunk fails
	chunkKeyByName map[string]string // first name → shared chunk key
}

func (p probedClassList) Validate(ctx context.Context, row *importModels.ClassListEntryImportRow) []importModels.ValidationError {
	errs := p.ClassListImportConfig.Validate(ctx, row)
	if p.warnLastName != "" && row.LastName == p.warnLastName {
		errs = append(errs, importModels.ValidationError{Field: "last_name", Message: "Prüfhinweis", Code: "probe_warning", Severity: importModels.ErrorSeverityWarning})
	}
	return errs
}

func (p probedClassList) FindExisting(ctx context.Context, row importModels.ClassListEntryImportRow) (*int64, error) {
	if p.failFirstName != "" && row.FirstName == p.failFirstName {
		return nil, errors.New("probe match failure")
	}
	return p.ClassListImportConfig.FindExisting(ctx, row)
}

func (p probedClassList) ChunkKey(row importModels.ClassListEntryImportRow) string {
	if key, ok := p.chunkKeyByName[row.FirstName]; ok {
		return key
	}
	return p.ClassListImportConfig.ChunkKey(row)
}

func (p probedClassList) ResolveChunk(ctx context.Context, rows []importModels.ClassListEntryImportRow) (ports.ImportConfig[importModels.ClassListEntryImportRow], error) {
	if p.failResolve {
		return nil, errors.New("probe resolve failure")
	}
	return p.ClassListImportConfig.ResolveChunk(ctx, rows)
}

// failingBatches fails the class-list batch command a number of times.
type failingBatches struct {
	ports.ClassListEntryBatches
	failures *atomic.Int32
	before   func()
}

func (f failingBatches) CreateClassListEntries(ctx context.Context, inputs []ports.CreateClassListEntry) ([]ports.ClassListEntry, error) {
	if f.failures.Add(-1) >= 0 {
		if f.before != nil {
			f.before()
		}
		return nil, errors.New("probe batch failure")
	}
	return f.ClassListEntryBatches.CreateClassListEntries(ctx, inputs)
}

// probeImport composes the class-list import over probe on the test's own
// tenant. rowPath composes it without the statement bound.
func probeImport(t *testing.T, db *bun.DB, probe probedClassList, rowPath bool, batchFailures int32) importModels.RowImporter[importModels.ClassListEntryImportRow] {
	t.Helper()
	runtime, _, err := api.NewImportTestRuntime(testpkg.NewDatabase(t, db), api.ImportTestOptions{RowPathOnly: rowPath})
	require.NoError(t, err)
	return probeImportOver(t, db, runtime, probe, batchFailures)
}

// probeImportOver composes the probe over a given import runtime.
func probeImportOver(t *testing.T, db *bun.DB, runtime ports.ImportRuntime, probe probedClassList, batchFailures int32) importModels.RowImporter[importModels.ClassListEntryImportRow] {
	t.Helper()
	_, persons, membership, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	batches, ok := membership.(ports.ClassListEntryBatches)
	require.True(t, ok)
	failures := &atomic.Int32{}
	failures.Store(batchFailures)
	probe.ClassListImportConfig = importService.NewClassListImportConfig(ports.ClassListImportDeps{
		Transactions: runtime.Transactions, Membership: membership, Batches: failingBatches{ClassListEntryBatches: batches, failures: failures},
		Persons: persons, Students: persons, Audit: runtime.Audit,
	})
	return importService.NewImportService[importModels.ClassListEntryImportRow](probe, runtime)
}

func runProbe(t *testing.T, db *bun.DB, importer importModels.RowImporter[importModels.ClassListEntryImportRow], rows []importModels.ClassListEntryImportRow) (*importModels.ImportResult[importModels.ClassListEntryImportRow], error) {
	t.Helper()
	actor := newImporter(t, db)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	return importer.ImportBatches(ctx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
		Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "class_list_entries", Filename: "probe.csv", AccountID: actor.accountID})
}

// TestChunkRollbackRestoresTheResultState is the regression test of the
// invariant "Chunk-Rollback": row 2 carries a validation warning and gets
// already_exists in the batch attempt, row 1 fails in the batch and in the
// replay. Compared with the row path, errors, counters and persistence are
// equal: row 2 carries only its warning, because the replay never reaches it.
// The same holds for duplicate_check_failed after a failed ResolveChunk, for
// two segments of which the second fails, and for a replay that succeeds.
func TestChunkRollbackRestoresTheResultState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	installRefusedMarkers(t, db)
	seedWarned := func(t *testing.T) {
		newImporter(t, db) // creates the tenant row the seed refers to
		_, err := db.ExecContext(context.Background(), `INSERT INTO users.class_list_entries (tenant_id, first_name, last_name, school_class) VALUES (?, 'Schon', 'Warnung', '1a')`, testpkg.Tenant(t))
		require.NoError(t, err)
	}
	cases := []struct {
		name          string
		probe         probedClassList
		rows          []importModels.ClassListEntryImportRow
		batchFailures int32
		failed        bool
		check         func(t *testing.T, outcome classListOutcome)
	}{
		{
			name:  "warning and already_exists in row 2, failure in row 1",
			probe: probedClassList{warnLastName: "Warnung"},
			rows: []importModels.ClassListEntryImportRow{
				{FirstName: "Erste", LastName: "Refused", SchoolClass: "1a"},
				{FirstName: "Schon", LastName: "Warnung", SchoolClass: "1a"},
				{FirstName: "Dritte", LastName: "Zeile", SchoolClass: "1a"},
			},
			failed: true,
			check: func(t *testing.T, outcome classListOutcome) {
				for _, rowError := range outcome.RowErrors {
					require.NotContains(t, rowError, "already_exists", "the replay never reached row 2")
				}
				require.Contains(t, outcome.RowErrors, "3 warning last_name probe_warning Prüfhinweis")
			},
		},
		{
			name:  "duplicate_check_failed after a failed ResolveChunk",
			probe: probedClassList{warnLastName: "Warnung", failFirstName: "Pruefung", failResolve: true},
			rows: []importModels.ClassListEntryImportRow{
				{FirstName: "Erste", LastName: "Refused", SchoolClass: "1a"},
				{FirstName: "Pruefung", LastName: "Warnung", SchoolClass: "1a"},
				{FirstName: "Dritte", LastName: "Zeile", SchoolClass: "1a"},
			},
			failed: true,
			check: func(t *testing.T, outcome classListOutcome) {
				for _, rowError := range outcome.RowErrors {
					require.NotContains(t, rowError, "duplicate_check_failed", "the replay never reached row 2")
				}
			},
		},
		{
			name:  "two segments, the second fails",
			probe: probedClassList{warnLastName: "Warnung", chunkKeyByName: map[string]string{"Erste": "same"}},
			rows: []importModels.ClassListEntryImportRow{
				{FirstName: "Erste", LastName: "Eins", SchoolClass: "1a"},
				{FirstName: "Schon", LastName: "Warnung", SchoolClass: "1a"},
				{FirstName: "Erste", LastName: "Refused", SchoolClass: "1a"},
				{FirstName: "Vierte", LastName: "Zeile", SchoolClass: "1a"},
			},
			failed: true,
			check: func(t *testing.T, outcome classListOutcome) {
				require.Equal(t, 1, strings.Count(strings.Join(outcome.RowErrors, "\n"), "already_exists"), "row 2 gets already_exists once, from the replay")
			},
		},
		{
			name:  "replay succeeds",
			probe: probedClassList{warnLastName: "Warnung"},
			rows: []importModels.ClassListEntryImportRow{
				{FirstName: "Erste", LastName: "Eins", SchoolClass: "1a"},
				{FirstName: "Schon", LastName: "Warnung", SchoolClass: "1a"},
				{FirstName: "Dritte", LastName: "Zeile", SchoolClass: "1a"},
			},
			batchFailures: 1,
			check: func(t *testing.T, outcome classListOutcome) {
				require.Equal(t, 2, outcome.Created)
				require.Equal(t, 1, strings.Count(strings.Join(outcome.RowErrors, "\n"), "already_exists"))
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outcomes := map[bool]classListOutcome{}
			for _, rowPath := range []bool{false, true} {
				t.Run(fmt.Sprintf("rowPath=%t", rowPath), func(t *testing.T) {
					testpkg.OwnTenant(t)
					seedWarned(t)
					result, err := runProbe(t, db, probeImport(t, db, c.probe, rowPath, c.batchFailures), c.rows)
					require.Equal(t, c.failed, err != nil, "%v", err)
					outcomes[rowPath] = classListOutcomeOf(t, db, result)
				})
			}
			require.Equal(t, outcomes[true], outcomes[false], "batch path and row path end alike")
			c.check(t, outcomes[false])
		})
	}
}

// TestChunkReplaySeesAParallelChange: between the first preload and the
// replay another connection commits a student carrying row 3's name and
// class. The replay answers like a sequential run that starts after that
// change: row 3 is already_exists. Once for the replay in the same
// transaction (a failed batch command) and once for the replay in a new
// transaction (E13′, the bound cancelled the attempt).
func TestChunkReplaySeesAParallelChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	rows := []importModels.ClassListEntryImportRow{
		{FirstName: "Erste", LastName: "Zeile", SchoolClass: "1a"},
		{FirstName: "Zweite", LastName: "Gehalten", SchoolClass: "1a"},
		{FirstName: "Parallel", LastName: "Kind", SchoolClass: "1a"},
	}
	outcomes := map[string]classListOutcome{}
	t.Run("same transaction", func(t *testing.T) {
		testpkg.OwnTenant(t)
		failures := &atomic.Int32{}
		failures.Store(1)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
			WrapClassListBatches: func(inner ports.ClassListEntryBatches) ports.ClassListEntryBatches {
				return failingBatches{ClassListEntryBatches: inner, failures: failures, before: func() {
					testpkg.CreateTestStudent(t, db, "Parallel", "Kind", "1a")
				}}
			},
		})
		require.NoError(t, err)
		actor := newImporter(t, db)
		upload := awaitUpload(t, uploadClassList(t, module, db, actor, rows, "parallel.csv"))
		require.NoError(t, upload.err)
		require.Equal(t, 1, (*module.Observations)[0].BatchFallbacks)
		outcomes["same transaction"] = classListOutcomeOf(t, db, upload.result)
	})
	t.Run("new transaction", func(t *testing.T) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{BatchStatementLimit: 200 * time.Millisecond})
		require.NoError(t, err)
		actor := newImporter(t, db)
		held := holdClassListKey(t, db, rows[1])
		done := uploadClassList(t, module, db, actor, rows, "parallel.csv")
		waitForLockWaiters(t, db)
		testpkg.CreateTestStudent(t, db, "Parallel", "Kind", "1a")
		// The hold outlasts the bound, the subject of the test.
		time.Sleep(400 * time.Millisecond)
		require.NoError(t, held.Rollback())
		upload := awaitUpload(t, done)
		require.NoError(t, upload.err)
		require.Equal(t, 1, (*module.Observations)[0].BatchFallbacks)
		outcomes["new transaction"] = classListOutcomeOf(t, db, upload.result)
	})
	t.Run("sequential after the change", func(t *testing.T) {
		testpkg.OwnTenant(t)
		actor := newImporter(t, db)
		testpkg.CreateTestStudent(t, db, "Parallel", "Kind", "1a")
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{RowPathOnly: true})
		require.NoError(t, err)
		upload := awaitUpload(t, uploadClassList(t, module, db, actor, rows, "parallel.csv"))
		require.NoError(t, upload.err)
		outcomes["sequential"] = classListOutcomeOf(t, db, upload.result)
	})
	require.Equal(t, outcomes["sequential"], outcomes["same transaction"])
	require.Equal(t, outcomes["sequential"], outcomes["new transaction"])
	require.Contains(t, strings.Join(outcomes["sequential"].RowErrors, "\n"), "4 error duplicate already_exists")
}

// installDeadlockScript makes every multi-row INSERT into the class list
// fail with 40P01 and the first rowPathFailures single-row INSERTs too.
// Sequence values survive rollbacks, so the count spans retries.
func installDeadlockScript(t *testing.T, db *bun.DB, rowPathFailures int) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), strings.ReplaceAll(fmt.Sprintf(`
		CREATE SEQUENCE public.class_list_row_deadlocks_N;
		GRANT USAGE ON SEQUENCE public.class_list_row_deadlocks_N TO phoenix_tenant;
		CREATE FUNCTION public.class_list_deadlock_script_N() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF (SELECT count(*) FROM inserted) > 1 OR nextval('public.class_list_row_deadlocks_N') <= %d THEN
				RAISE EXCEPTION 'injected deadlock' USING ERRCODE = '40P01';
			END IF;
			RETURN NULL;
		END $$;
		CREATE TRIGGER class_list_deadlock_script_N AFTER INSERT ON users.class_list_entries
			REFERENCING NEW TABLE AS inserted FOR EACH STATEMENT EXECUTE FUNCTION public.class_list_deadlock_script_N()`, rowPathFailures), "_N", fmt.Sprintf("_%d", rowPathFailures)))
	require.NoError(t, err)
}

// TestChunkDeadlockFallbackKeepsTheRetryBudget: a 40P01 in the batch attempt
// consumes no attempt. The row path then fails three times with 40P01 and
// succeeds in the fourth attempt, as it does today.
func TestChunkDeadlockFallbackKeepsTheRetryBudget(t *testing.T) {
	t.Parallel()
	observation, entries, err := runDeadlockScript(t, 3)
	require.NoError(t, err)
	require.Equal(t, 3, observation.BatchesRetried, "the row path used all three retries")
	require.Equal(t, 5, entries)
}

// TestChunkDeadlockFallbackEndsLikeTodayAfterTheBudget: a fourth row-path
// deadlock ends with the error, as it does today.
func TestChunkDeadlockFallbackEndsLikeTodayAfterTheBudget(t *testing.T) {
	t.Parallel()
	observation, entries, err := runDeadlockScript(t, 4)
	var state interface{ Field(byte) string }
	require.True(t, errors.As(err, &state), "%v", err)
	require.Equal(t, "40P01", state.Field('C'))
	require.Equal(t, 3, observation.BatchesRetried)
	require.Zero(t, entries)
}

// runDeadlockScript fails every batch INSERT of the class list and the first
// rowPathFailures single-row INSERTs with 40P01.
func runDeadlockScript(t *testing.T, rowPathFailures int) (importModels.ImportObservation, int, error) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	installDeadlockScript(t, db, rowPathFailures)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitUpload(t, uploadClassList(t, module, db, actor, overlapFile(), "deadlock.csv"))
	observation := (*module.Observations)[0]
	require.Equal(t, 1, observation.BatchFallbacks)
	return observation, countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`), upload.err
}

// faultConnector fails one statement starting with prefix once it is armed:
// the nth such statement (first when nth is 0), with err (driver.ErrBadConn
// when nil). Every other statement runs on a real PostgreSQL connection.
type faultConnector struct {
	driver.Connector
	armed   *atomic.Bool
	prefix  string
	nth     int32
	err     error
	matches *atomic.Int32
	// A second, independent rule fails the first statement starting with
	// prefix2 with err2 once armed2 is set (a fault after a fault).
	prefix2 string
	err2    error
	armed2  *atomic.Bool
}

func (c faultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &faultConn{Conn: conn, connector: c}, nil
}

type faultConn struct {
	driver.Conn
	connector faultConnector
}

func (c *faultConn) fail(query string) error {
	if second := c.connector; second.armed2 != nil && strings.HasPrefix(strings.TrimSpace(query), second.prefix2) && second.armed2.CompareAndSwap(true, false) {
		return second.err2
	}
	if !c.connector.armed.Load() || !strings.HasPrefix(strings.TrimSpace(query), c.connector.prefix) {
		return nil
	}
	if c.connector.matches != nil && c.connector.matches.Add(1) < max(c.connector.nth, 1) {
		return nil
	}
	if !c.connector.armed.CompareAndSwap(true, false) {
		return nil
	}
	if c.connector.err != nil {
		return c.connector.err
	}
	return driver.ErrBadConn
}

func (c *faultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.fail(query); err != nil {
		return nil, err
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *faultConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.fail(query); err != nil {
		return nil, err
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *faultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (c *faultConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *faultConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *faultConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *faultConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

// TestChunkFrameStartFailureIsNeitherReplayedNorSwallowed: a real one-time
// driver.ErrBadConn on the chunk's SAVEPOINT or on the bound's first
// statement ends the import with that error. Nothing replays, nothing is
// retried, nothing of the chunk commits (lesson of Q6).
func TestChunkFrameStartFailureIsNeitherReplayedNorSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	dsn := testpkg.IsolatedTestDSN(t)
	for _, prefix := range []string{"SAVEPOINT phoenix_operation", "SELECT current_setting('statement_timeout')"} {
		t.Run(prefix, func(t *testing.T) {
			testpkg.OwnTenant(t)
			armed := &atomic.Bool{}
			pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
				Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)), armed: armed, prefix: prefix,
			})))
			t.Cleanup(func() { _ = pool.Close() })
			module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
			require.NoError(t, err)
			actor := newImporter(t, db)
			armed.Store(true)
			upload := awaitUpload(t, uploadClassList(t, module, pool, actor, overlapFile(), "frame.csv"))
			require.False(t, armed.Load(), "the fault fired")
			require.ErrorIs(t, upload.err, driver.ErrBadConn)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks, "no replay")
			require.Zero(t, observation.BatchesRetried, "no retry")
			for _, command := range observation.Commands {
				require.NotEqual(t, "create_class_list_entry", command.Operation, "no row-path write ran")
			}
			require.Zero(t, countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`))
			require.Zero(t, countIn(t, db, `SELECT count(*) FROM audit.data_imports WHERE tenant_id = ?`))
		})
	}
}

// TestIdenticalUploadMeanwhileIsCaughtUpAfterTheBound (E13′, condition 2):
// the bound ends upload A's chunk transaction and with it A's import lock.
// The identical upload B, waiting for that lock, writes the chunk and its
// checkpoint. A's replay takes the lock after B, reads B's checkpoint and
// catches up: the chunk is written once, and exactly one checkpoint exists.
func TestIdenticalUploadMeanwhileIsCaughtUpAfterTheBound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.Tenant(t)
	first, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{BatchStatementLimit: 600 * time.Millisecond})
	require.NoError(t, err)
	second, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := overlapFile()
	held := holdClassListKey(t, db, rows[2])
	doneA := uploadClassList(t, first, db, actor, rows, "identical.csv")
	waitForLockWaiters(t, db)
	doneB := uploadClassList(t, second, db, actor, rows, "identical.csv")
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted`).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 5*time.Millisecond, "the identical upload waits for the import lock")
	// B waits for the import lock before A's bound of 0.6 s ends A's chunk
	// transaction; the hold outlasts that bound. The times are the subject of
	// the test.
	time.Sleep(900 * time.Millisecond)
	require.NoError(t, held.Rollback())
	uploadA, uploadB := awaitUpload(t, doneA), awaitUpload(t, doneB)
	require.NoError(t, uploadA.err)
	require.NoError(t, uploadB.err)
	require.Equal(t, 1, (*first.Observations)[0].BatchFallbacks)
	require.Equal(t, 5, countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`), "the chunk is written once")
	require.Equal(t, 1, countIn(t, db, `SELECT count(*) FROM audit.data_imports WHERE tenant_id = ?`), "one checkpoint, neither twice nor missing")
	require.Equal(t, uploadB.result.CreatedCount, uploadA.result.CreatedCount, "A reports the chunk B wrote")
	require.Equal(t, 5, uploadA.result.CreatedCount)
	for _, command := range (*first.Observations)[0].Commands {
		require.NotEqual(t, "create_class_list_entry", command.Operation, "A's replay caught up instead of writing")
	}
	require.Zero(t, (*second.Observations)[0].BatchFallbacks)
}

// TestParallelClassListImportsOfOneSchool: two processes import overlapping
// files into one school at once, the first holding its chunk open; two
// uploads in one process serialize over the import lock of the service.
// No deadlock arises, every child is on the list once, and both answer.
func TestParallelClassListImportsOfOneSchool(t *testing.T) {
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
		CREATE FUNCTION public.slow_first_checkpoint() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.filename = 'first.csv' THEN PERFORM pg_sleep(0.5); END IF; RETURN NULL; END $$;
		CREATE TRIGGER slow_first_checkpoint AFTER INSERT ON audit.data_imports FOR EACH ROW EXECUTE FUNCTION public.slow_first_checkpoint()`)
	require.NoError(t, err)
	processA, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	processB, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	file := func(prefix string, from, to int) []importModels.ClassListEntryImportRow {
		var rows []importModels.ClassListEntryImportRow
		for i := from; i < to; i++ {
			rows = append(rows, importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Kind%d", i), LastName: prefix, SchoolClass: "1a"})
		}
		return rows
	}
	doneA := uploadClassList(t, processA, db, actor, file("Parallel", 0, 30), "first.csv")
	require.Eventually(t, func() bool {
		var sleeping int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event = 'PgSleep'`).Scan(context.Background(), &sleeping)
		return err == nil && sleeping > 0
	}, 10*time.Second, 5*time.Millisecond, "the first upload holds its chunk open")
	doneB := uploadClassList(t, processB, db, actor, file("Parallel", 20, 50), "second.csv")
	sameProcess := []<-chan classListUpload{
		uploadClassList(t, processB, db, actor, file("Prozess0", 0, 10), "same-0.csv"),
		uploadClassList(t, processB, db, actor, file("Prozess1", 0, 10), "same-1.csv"),
	}
	uploadA, uploadB := awaitUpload(t, doneA), awaitUpload(t, doneB)
	require.NoError(t, uploadA.err)
	require.NoError(t, uploadB.err, "the second process waits for the first and reports its rows as existing")
	for _, done := range sameProcess {
		upload := awaitUpload(t, done)
		require.NoError(t, upload.err)
		require.Equal(t, 10, upload.result.CreatedCount)
	}
	require.Equal(t, 30, uploadA.result.CreatedCount)
	require.Equal(t, 20, uploadB.result.CreatedCount)
	require.Equal(t, 10, uploadB.result.ErrorCount, "the overlap is already on the list")
	require.Equal(t, 70, countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`), "every child once")
	require.Equal(t, before, deadlocks(), "no deadlock")
}
